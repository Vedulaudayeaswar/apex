"""YOLOv8 detection pipeline with zone/queue analytics and Kafka publishing."""
from __future__ import annotations

import json
import logging
import threading
import time
import uuid
from datetime import datetime, timezone
from typing import Dict, List, Optional

import cv2
import numpy as np
import torch
from kafka import KafkaProducer
from ultralytics import YOLO

from app.config import (
    CAMERA_ID,
    CONFIDENCE,
    CROWD_THRESHOLD,
    EVENT_INTERVAL_SEC,
    INFERENCE_FRAME_INTERVAL,
    INFERENCE_SIZE,
    KAFKA_BROKERS,
    MAX_VIDEO_FPS,
    MODEL_PATH,
    QUEUE_ZONE,
    REID_THRESHOLD,
    REID_WINDOW_SEC,
    STORE_ID,
    TORCH_NUM_THREADS,
    ZONES,
)
from app.tracker import ByteTracker, movement_pattern

logger = logging.getLogger(__name__)
torch.set_num_threads(TORCH_NUM_THREADS)
torch.set_num_interop_threads(1)
cv2.setNumThreads(1)


def resolve_zone(x: float, y: float) -> str:
    zones_by_specificity = sorted(
        ZONES.items(),
        key=lambda item: (
            (item[1][1][0] - item[1][0][0])
            * (item[1][1][1] - item[1][0][1])
        ),
    )
    for zone_id, ((x1, y1), (x2, y2)) in zones_by_specificity:
        if x1 <= x <= x2 and y1 <= y <= y2:
            return zone_id
    return "GENERAL"


class DetectionPipeline:
    def __init__(self):
        self.model = YOLO(MODEL_PATH)
        self.tracker = ByteTracker(REID_THRESHOLD, REID_WINDOW_SEC)
        self._producer: Optional[KafkaProducer] = None
        self._producer_retry_at = 0.0
        self._last_detection_emit: Dict[str, float] = {}
        self._last_anomaly_emit: Dict[str, float] = {}
        self._emitted_reentries: set[str] = set()
        self._last_zone: Dict[str, str] = {}
        self.latest_frame: Optional[np.ndarray] = None
        self.latest_overlay: List[dict] = []
        self.heatmap_points: List[dict] = []
        self._lock = threading.Lock()
        self._running = False
        self._video_source = ""
        self._source_version = 0
        self.frame_size = {"width": 640, "height": 480}
        self.is_processing = False
        self.current_source = ""

    def _get_producer(self) -> KafkaProducer:
        if self._producer is None:
            if time.time() < self._producer_retry_at:
                raise RuntimeError("kafka retry pending")
            self._producer = KafkaProducer(
                bootstrap_servers=KAFKA_BROKERS.split(","),
                value_serializer=lambda v: json.dumps(v).encode("utf-8"),
                key_serializer=lambda k: k.encode("utf-8") if k else None,
                acks=1,
                retries=3,
                api_version_auto_timeout_ms=1500,
            )
        return self._producer

    def _emit(self, topic: str, event: dict) -> None:
        if time.time() < self._producer_retry_at:
            return
        try:
            self._get_producer().send(topic, key=event.get("store_id"), value=event)
        except Exception as e:
            self._producer = None
            self._producer_retry_at = time.time() + 5
            logger.warning("kafka publish failed: %s", e)

    def _build_event(
        self,
        visitor_id: str,
        event_type: str,
        zone_id: str,
        confidence: float,
        metadata: dict,
    ) -> dict:
        return {
            "event_id": str(uuid.uuid4()),
            "store_id": STORE_ID,
            "camera_id": CAMERA_ID,
            "visitor_id": visitor_id,
            "event_type": event_type,
            "timestamp": datetime.now(timezone.utc).isoformat(),
            "zone_id": zone_id,
            "dwell_ms": 0,
            "is_staff": False,
            "confidence": confidence,
            "metadata": metadata,
        }

    def set_source(self, path: str) -> None:
        with self._lock:
            self._video_source = path
            self._source_version += 1
            self.current_source = path
            self.tracker = ByteTracker(REID_THRESHOLD, REID_WINDOW_SEC)
            self._last_zone.clear()
            self._last_detection_emit.clear()
            self._last_anomaly_emit.clear()
            self._emitted_reentries.clear()
            self.latest_frame = None
            self.latest_overlay = []
            self.heatmap_points = []
            self.frame_size = {"width": 640, "height": 480}
        logger.info("Video source switched to %s", path)
        self._emit("store-events", self._build_event(
            "SYSTEM", "RESET", "GENERAL", 1.0, {"source": path}))

    def process_frame(self, frame: np.ndarray) -> np.ndarray:
        self.is_processing = True
        h, w = frame.shape[:2]
        self.frame_size = {"width": w, "height": h}

        results = self.model(
            frame, conf=CONFIDENCE, classes=[0], imgsz=INFERENCE_SIZE, verbose=False)
        detections = []
        for r in results:
            for box in r.boxes:
                x1, y1, x2, y2 = box.xyxy[0].tolist()
                detections.append({
                    "bbox": [x1, y1, x2, y2],
                    "confidence": float(box.conf[0]),
                })

        tracks = self.tracker.update(
            detections, frame,
            lambda x, y: resolve_zone(x * 640 / w, y * 480 / h),
        )
        overlay = []
        queue_count = sum(1 for t in tracks if t.zone_id == QUEUE_ZONE)

        for track in tracks:
            pattern = movement_pattern(track.trajectory)
            conf = next(
                (d["confidence"] for d in detections if self._iou(d["bbox"], track.bbox) > 0.3),
                0.91,
            )
            reentry = track.session_count > 1
            meta = {
                "bbox": track.bbox,
                "track_id": track.track_id,
                "queue_depth": queue_count,
                "session_seq": track.session_count,
                "reentry_detected": reentry,
                "movement_pattern": pattern,
            }
            cx = (track.bbox[0] + track.bbox[2]) / 2
            cy = (track.bbox[1] + track.bbox[3]) / 2
            overlay.append({
                "visitor_id": track.visitor_id,
                "bbox": track.bbox,
                "zone_id": track.zone_id,
                "confidence": round(conf, 2),
                "trajectory": track.trajectory[-30:],
                "reentry": reentry,
                "movement_pattern": pattern,
                "is_anomaly": pattern in ("ERRATIC", "LOITERING"),
            })
            self.heatmap_points.append({"x": cx / w, "y": cy / h, "t": time.time()})
            if len(self.heatmap_points) > 500:
                self.heatmap_points = self.heatmap_points[-500:]

            prev_zone = self._last_zone.get(track.visitor_id)
            if prev_zone != track.zone_id:
                if prev_zone:
                    self._emit("store-events", self._build_event(
                        track.visitor_id, "ZONE_EXIT", prev_zone, conf, meta))
                self._emit("store-events", self._build_event(
                    track.visitor_id, "ZONE_ENTER", track.zone_id, conf, meta))
                if track.zone_id == "ENTRY" and not prev_zone:
                    self._emit("store-events", self._build_event(
                        track.visitor_id, "ENTRY", track.zone_id, conf, meta))
                if reentry and track.visitor_id not in self._emitted_reentries:
                    self._emit("store-events", self._build_event(
                        track.visitor_id, "REENTRY", track.zone_id, conf, meta))
                    self._emitted_reentries.add(track.visitor_id)
                self._last_zone[track.visitor_id] = track.zone_id

            if track.zone_id == QUEUE_ZONE and queue_count >= 3:
                self._emit("store-events", self._build_event(
                    track.visitor_id, "QUEUE_JOIN", QUEUE_ZONE, 0.85, meta))

            anomaly_key = f"{track.visitor_id}:{pattern}"
            if (
                pattern in ("ERRATIC", "LOITERING")
                and time.time() - self._last_anomaly_emit.get(anomaly_key, 0) >= 30
            ):
                self._emit("store-events", self._build_event(
                    track.visitor_id, "ANOMALY", track.zone_id, conf, {
                        **meta,
                        "anomaly_type": "BEHAVIORAL_ANOMALY",
                        "severity": "MEDIUM",
                        "movement_pattern": "Behavioral anomaly detected",
                    }))
                self._last_anomaly_emit[anomaly_key] = time.time()

            now = time.time()
            if now - self._last_detection_emit.get(track.visitor_id, 0) >= EVENT_INTERVAL_SEC:
                raw = self._build_event(track.visitor_id, "DETECTION", track.zone_id, conf, meta)
                self._emit("raw-detections", raw)
                self._emit("store-events", raw)
                self._last_detection_emit[track.visitor_id] = now

        if len(tracks) >= CROWD_THRESHOLD:
            self._emit("store-events", self._build_event(
                "SYSTEM", "ANOMALY", "GENERAL", 0.88,
                {"anomaly_type": "CROWDED_ZONE", "severity": "MEDIUM",
                 "movement_pattern": f"{len(tracks)} persons in frame"},
            ))

        with self._lock:
            self.latest_frame = frame.copy()
            self.latest_overlay = overlay
        self.is_processing = False
        return frame

    @staticmethod
    def _iou(a: List[float], b: List[float]) -> float:
        ax1, ay1, ax2, ay2 = a
        bx1, by1, bx2, by2 = b
        ix1, iy1 = max(ax1, bx1), max(ay1, by1)
        ix2, iy2 = min(ax2, bx2), min(ay2, by2)
        inter = max(0, ix2 - ix1) * max(0, iy2 - iy1)
        union = (ax2 - ax1) * (ay2 - ay1) + (bx2 - bx1) * (by2 - by1) - inter + 1e-8
        return inter / union

    def _open_capture(self, src: str) -> cv2.VideoCapture:
        if src.isdigit():
            return cv2.VideoCapture(int(src))
        return cv2.VideoCapture(src)

    def run_loop(self) -> None:
        self._running = True
        local_version = -1
        cap = None
        frame_number = 0

        while self._running:
            with self._lock:
                src = self._video_source
                version = self._source_version

            if version != local_version:
                local_version = version
                if cap is not None:
                    cap.release()
                cap = self._open_capture(src) if src else None
                frame_number = 0
                if cap is None:
                    logger.info("Waiting for CCTV footage upload")
                elif not cap.isOpened():
                    logger.error("Cannot open video source: %s", src)
                else:
                    logger.info("Opened video source: %s", src)

            if cap is None or not cap.isOpened():
                frame = np.zeros((480, 640, 3), dtype=np.uint8)
                cv2.putText(frame, "UPLOAD CCTV FOOTAGE TO BEGIN", (40, 240),
                            cv2.FONT_HERSHEY_SIMPLEX, 0.7, (255, 255, 255), 2)
                with self._lock:
                    self.latest_frame = frame
                    self.latest_overlay = []
                time.sleep(0.5)
                continue

            ret, frame = cap.read()
            if not ret:
                cap.set(cv2.CAP_PROP_POS_FRAMES, 0)
                continue
            started_at = time.time()
            frame_number += 1
            if frame_number % INFERENCE_FRAME_INTERVAL == 0:
                self.process_frame(frame)
            else:
                with self._lock:
                    self.latest_frame = frame.copy()
            source_fps = cap.get(cv2.CAP_PROP_FPS) or MAX_VIDEO_FPS
            target_fps = min(source_fps, MAX_VIDEO_FPS)
            time.sleep(max(0, (1 / target_fps) - (time.time() - started_at)))

    def stop(self) -> None:
        self._running = False

    def get_heatmap_grid(self, grid=16) -> List[dict]:
        now = time.time()
        cells = [[0.0] * grid for _ in range(grid)]
        for p in self.heatmap_points:
            if now - p["t"] > 120:
                continue
            gx = min(grid - 1, int(p["x"] * grid))
            gy = min(grid - 1, int(p["y"] * grid))
            cells[gy][gx] += 1.0
        max_v = max(max(row) for row in cells) or 1.0
        out = []
        for y in range(grid):
            for x in range(grid):
                out.append({"grid_x": x, "grid_y": y, "intensity": cells[y][x] / max_v})
        return out
