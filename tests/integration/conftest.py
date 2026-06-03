import sys
from pathlib import Path

# Add detection service to path for unit tests
det_path = Path(__file__).resolve().parents[2] / "services" / "detection-service"
sys.path.insert(0, str(det_path))
