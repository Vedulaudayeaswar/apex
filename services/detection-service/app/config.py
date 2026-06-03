import os

STORE_ID = os.getenv("STORE_ID", "STORE_BLR_002")
CAMERA_ID = os.getenv("CAMERA_ID", "CAM_ENTRY_01")
KAFKA_BROKERS = os.getenv("KAFKA_BROKERS", "kafka:29092")
REDIS_URL = os.getenv("REDIS_URL", "redis://redis:6379/0")
HTTP_PORT = int(os.getenv("HTTP_PORT", "8090"))
MODEL_PATH = os.getenv("YOLO_MODEL", "yolov8n.pt")
CONFIDENCE = float(os.getenv("DETECTION_CONF", "0.5"))
REID_THRESHOLD = float(os.getenv("REID_THRESHOLD", "0.65"))
REID_WINDOW_SEC = int(os.getenv("REID_WINDOW_SEC", "300"))

ZONES = {
    "ENTRY": [(0, 0), (320, 480)],
    "ELECTRONICS": [(320, 0), (640, 240)],
    "APPAREL": [(320, 240), (640, 480)],
    "BILLING": [(500, 300), (640, 480)],
}

QUEUE_ZONE = "BILLING"
CROWD_THRESHOLD = 6
INFERENCE_SIZE = int(os.getenv("INFERENCE_SIZE", "320"))
INFERENCE_FRAME_INTERVAL = max(1, int(os.getenv("INFERENCE_FRAME_INTERVAL", "5")))
EVENT_INTERVAL_SEC = float(os.getenv("EVENT_INTERVAL_SEC", "1.0"))
STREAM_FPS = max(1, int(os.getenv("STREAM_FPS", "10")))
STREAM_WIDTH = max(320, int(os.getenv("STREAM_WIDTH", "1280")))
MAX_VIDEO_FPS = max(1, int(os.getenv("MAX_VIDEO_FPS", "15")))
TORCH_NUM_THREADS = max(1, int(os.getenv("TORCH_NUM_THREADS", "2")))
