"""Detection service HTTP + video stream + upload."""
import logging
import os
import threading
import time
import uuid
from pathlib import Path

import cv2
import numpy as np
from flask import Flask, Response, jsonify, request
from flask_cors import CORS
from prometheus_client import Counter, generate_latest
from werkzeug.utils import secure_filename

from app.config import HTTP_PORT, STREAM_FPS, STREAM_WIDTH, ZONES
from app.pipeline import DetectionPipeline

logging.basicConfig(level=logging.INFO)
app = Flask(__name__)
CORS(app)

UPLOAD_DIR = Path(os.getenv("UPLOAD_DIR", "/data/uploads"))
UPLOAD_DIR.mkdir(parents=True, exist_ok=True)
ALLOWED = {".mp4", ".avi", ".mov", ".mkv", ".webm"}

pipeline = DetectionPipeline()
frames_processed = Counter("detection_frames_total", "Frames processed")


def mjpeg_stream():
    while True:
        with pipeline._lock:
            frame = pipeline.latest_frame
        if frame is None:
            frame = np.zeros((480, 640, 3), dtype=np.uint8)
        if frame.shape[1] > STREAM_WIDTH:
            height = int(frame.shape[0] * STREAM_WIDTH / frame.shape[1])
            frame = cv2.resize(frame, (STREAM_WIDTH, height), interpolation=cv2.INTER_AREA)
        _, buf = cv2.imencode(".jpg", frame, [cv2.IMWRITE_JPEG_QUALITY, 75])
        yield (
            b"--frame\r\n"
            b"Content-Type: image/jpeg\r\n\r\n" + buf.tobytes() + b"\r\n"
        )
        time.sleep(1 / STREAM_FPS)


@app.route("/health")
def health():
    return jsonify({
        "status": "ok",
        "service": "detection-service",
        "processing": pipeline.is_processing,
        "source": pipeline.current_source,
    })


@app.route("/metrics")
def metrics():
    return Response(generate_latest(), mimetype="text/plain")


@app.route("/stream")
def stream():
    return Response(mjpeg_stream(), mimetype="multipart/x-mixed-replace; boundary=frame")


@app.route("/overlay")
def overlay():
    with pipeline._lock:
        width = pipeline.frame_size["width"]
        height = pipeline.frame_size["height"]
        zones = {
            name: [
                (int(x1 * width / 640), int(y1 * height / 480)),
                (int(x2 * width / 640), int(y2 * height / 480)),
            ]
            for name, ((x1, y1), (x2, y2)) in ZONES.items()
        }
        return jsonify({
            "detections": pipeline.latest_overlay,
            "frame_size": pipeline.frame_size,
            "zones": zones,
        })


@app.route("/heatmap")
def heatmap():
    return jsonify({"cells": pipeline.get_heatmap_grid()})


@app.route("/upload", methods=["POST"])
def upload():
    if "video" not in request.files:
        return jsonify({"error": "missing video field"}), 400
    f = request.files["video"]
    if not f.filename:
        return jsonify({"error": "empty filename"}), 400
    ext = Path(f.filename).suffix.lower()
    if ext not in ALLOWED:
        return jsonify({"error": f"unsupported format {ext}"}), 400

    safe = f"{uuid.uuid4().hex}_{secure_filename(f.filename)}"
    dest = UPLOAD_DIR / safe
    f.save(dest)
    pipeline.set_source(str(dest))
    return jsonify({"status": "processing", "path": str(dest), "filename": safe})


if __name__ == "__main__":
    t = threading.Thread(target=pipeline.run_loop, daemon=True)
    t.start()
    app.run(host="0.0.0.0", port=HTTP_PORT, threaded=True)
