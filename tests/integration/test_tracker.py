"""Re-entry and tracking edge case tests."""
import numpy as np
import pytest

from app.tracker import ByteTracker, movement_pattern, ReIDGallery


def test_reid_gallery_match():
    gallery = ReIDGallery(threshold=0.5, window_sec=60)
    emb = np.random.rand(512).astype(np.float32)
    emb /= np.linalg.norm(emb)
    gallery.register("VIS_001", emb)
    matched = gallery.match(emb * 0.99)
    assert matched == "VIS_001"


def test_movement_pattern_loitering():
    traj = [(100, 100)] * 20
    assert movement_pattern(traj) == "LOITERING"


def test_byte_tracker_assigns_visitor_ids():
    tracker = ByteTracker(reid_threshold=0.99, reid_window=60)
    frame = np.zeros((480, 640, 3), dtype=np.uint8)
    dets = [{"bbox": [10, 10, 100, 200], "confidence": 0.9}]
    tracks = tracker.update(dets, frame, lambda x, y: "ENTRY")
    assert len(tracks) == 1
    assert tracks[0].visitor_id.startswith("VIS_")


def test_empty_store_no_tracks():
    tracker = ByteTracker()
    frame = np.zeros((480, 640, 3), dtype=np.uint8)
    assert tracker.update([], frame, lambda x, y: "GENERAL") == []
