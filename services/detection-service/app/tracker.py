"""ByteTrack-style multi-object tracking with cosine ReID embeddings."""
from __future__ import annotations

import time
import uuid
from collections import defaultdict
from dataclasses import dataclass, field
from typing import Dict, List, Optional, Tuple

import numpy as np


@dataclass
class Track:
    track_id: int
    visitor_id: str
    bbox: List[float]
    embedding: np.ndarray
    zone_id: str = ""
    last_seen: float = field(default_factory=time.time)
    trajectory: List[Tuple[float, float]] = field(default_factory=list)
    session_count: int = 1


class ReIDGallery:
    """Temporal gallery for re-identification after occlusion."""

    def __init__(self, threshold: float = 0.65, window_sec: int = 300):
        self.threshold = threshold
        self.window_sec = window_sec
        self._embeddings: Dict[str, Tuple[np.ndarray, float]] = {}

    def _cosine(self, a: np.ndarray, b: np.ndarray) -> float:
        na, nb = np.linalg.norm(a), np.linalg.norm(b)
        if na < 1e-8 or nb < 1e-8:
            return 0.0
        return float(np.dot(a, b) / (na * nb))

    def match(self, emb: np.ndarray, excluded_ids: set[str] | None = None) -> Optional[str]:
        now = time.time()
        best_id, best_sim = None, -1.0
        expired = []
        excluded_ids = excluded_ids or set()
        for vid, (stored, ts) in self._embeddings.items():
            if now - ts > self.window_sec:
                expired.append(vid)
                continue
            if vid in excluded_ids:
                continue
            sim = self._cosine(emb, stored)
            if sim > best_sim:
                best_sim, best_id = sim, vid
        for vid in expired:
            del self._embeddings[vid]
        if best_id and best_sim >= self.threshold:
            return best_id
        return None

    def register(self, visitor_id: str, emb: np.ndarray) -> None:
        self._embeddings[visitor_id] = (emb.copy(), time.time())


class ByteTracker:
    def __init__(self, reid_threshold: float = 0.65, reid_window: int = 300):
        self._next_track = 1
        self._tracks: Dict[int, Track] = {}
        self._gallery = ReIDGallery(reid_threshold, reid_window)
        self._visitor_counter = 0

    def _new_visitor_id(self) -> str:
        self._visitor_counter += 1
        return f"VIS_{self._visitor_counter:03d}"

    @staticmethod
    def _embedding_from_crop(crop: np.ndarray) -> np.ndarray:
        """OSNet-lite: normalized color histogram embedding (production: swap for OSNet ONNX)."""
        if crop.size == 0:
            return np.zeros(64)
        small = crop.reshape(-1, 3) if len(crop.shape) == 3 else crop.flatten().reshape(-1, 1)
        hist, _ = np.histogramdd(
            small[:, :3] if small.shape[1] >= 3 else np.zeros((len(small), 3)),
            bins=(8, 8, 8),
            range=((0, 256), (0, 256), (0, 256)),
        )
        emb = hist.flatten().astype(np.float32)
        emb /= np.linalg.norm(emb) + 1e-8
        return emb

    def _iou(self, a: List[float], b: List[float]) -> float:
        ax1, ay1, ax2, ay2 = a
        bx1, by1, bx2, by2 = b
        ix1, iy1 = max(ax1, bx1), max(ay1, by1)
        ix2, iy2 = min(ax2, bx2), min(ay2, by2)
        inter = max(0, ix2 - ix1) * max(0, iy2 - iy1)
        area_a = (ax2 - ax1) * (ay2 - ay1)
        area_b = (bx2 - bx1) * (by2 - by1)
        union = area_a + area_b - inter + 1e-8
        return inter / union

    def update(
        self,
        detections: List[dict],
        frame: np.ndarray,
        zone_resolver,
    ) -> List[Track]:
        """Associate detections to tracks; assign visitor IDs with ReID."""
        matched_tracks: Dict[int, dict] = {}
        unmatched_dets = list(detections)
        used_track_ids = set()

        for det in detections:
            best_tid, best_iou = None, 0.3
            for tid, track in self._tracks.items():
                if tid in used_track_ids:
                    continue
                iou = self._iou(det["bbox"], track.bbox)
                if iou > best_iou:
                    best_iou, best_tid = iou, tid
            if best_tid is not None:
                matched_tracks[best_tid] = det
                used_track_ids.add(best_tid)
                unmatched_dets.remove(det)

        active: List[Track] = []
        now = time.time()

        for tid, det in matched_tracks.items():
            track = self._tracks[tid]
            x1, y1, x2, y2 = [int(v) for v in det["bbox"]]
            crop = frame[max(0, y1):y2, max(0, x1):x2]
            emb = self._embedding_from_crop(crop)
            track.bbox = det["bbox"]
            track.embedding = emb
            track.last_seen = now
            cx, cy = (x1 + x2) / 2, (y1 + y2) / 2
            track.trajectory.append((cx, cy))
            if len(track.trajectory) > 50:
                track.trajectory = track.trajectory[-50:]
            track.zone_id = zone_resolver(cx, cy)
            active.append(track)

        active_visitor_ids = {
            track.visitor_id
            for track in self._tracks.values()
            if now - track.last_seen <= 5.0
        }
        for det in unmatched_dets:
            x1, y1, x2, y2 = [int(v) for v in det["bbox"]]
            crop = frame[max(0, y1):y2, max(0, x1):x2]
            emb = self._embedding_from_crop(crop)
            vid = self._gallery.match(emb, active_visitor_ids)
            reentry = vid is not None
            if not vid:
                vid = self._new_visitor_id()
                self._gallery.register(vid, emb)
            else:
                self._gallery.register(vid, emb)

            tid = self._next_track
            self._next_track += 1
            cx, cy = (x1 + x2) / 2, (y1 + y2) / 2
            track = Track(
                track_id=tid,
                visitor_id=vid,
                bbox=det["bbox"],
                embedding=emb,
                zone_id=zone_resolver(cx, cy),
                trajectory=[(cx, cy)],
                session_count=2 if reentry else 1,
            )
            self._tracks[tid] = track
            active.append(track)
            active_visitor_ids.add(vid)

        stale = [tid for tid, t in self._tracks.items() if now - t.last_seen > 5.0]
        for tid in stale:
            del self._tracks[tid]

        return active


def movement_pattern(trajectory: List[Tuple[float, float]]) -> str:
    if len(trajectory) < 4:
        return "NORMAL"
    deltas = []
    for i in range(1, len(trajectory)):
        dx = trajectory[i][0] - trajectory[i - 1][0]
        dy = trajectory[i][1] - trajectory[i - 1][1]
        deltas.append(np.hypot(dx, dy))
    if np.std(deltas) > 80:
        return "ERRATIC"
    if len(trajectory) > 15 and np.mean(deltas) < 2:
        return "LOITERING"
    return "NORMAL"
