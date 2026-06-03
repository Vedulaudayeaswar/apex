#!/usr/bin/env python3
"""
Single orchestrator for AI Retail Store Intelligence Platform.
Starts Docker Compose, initializes infra, verifies health, streams logs.
"""
from __future__ import annotations

import argparse
import http.client
import json
import os
import subprocess
import sys
import time
import urllib.error
import urllib.request
import webbrowser
from pathlib import Path

ROOT = Path(__file__).resolve().parent
COMPOSE_FILE = ROOT / "docker-compose.yml"

HEALTH_CHECKS = [
    ("api-gateway", "http://localhost:8080/health"),
    ("session-service", "http://localhost:8081/health"),
    ("ingestion-service", "http://localhost:8082/health"),
    ("metrics-service", "http://localhost:8083/health"),
    ("anomaly-service", "http://localhost:8084/health"),
    ("detection-service", "http://localhost:8090/health"),
]

DASHBOARD_URL = "http://localhost:3000"


def run(cmd: list[str], check: bool = True, cwd: Path | None = None) -> subprocess.CompletedProcess:
    print(f"$ {' '.join(cmd)}")
    return subprocess.run(cmd, cwd=cwd or ROOT, check=check)


def docker_available() -> bool:
    try:
        run(["docker", "info"], check=True)
        run(["docker", "compose", "version"], check=True)
        return True
    except (subprocess.CalledProcessError, FileNotFoundError):
        return False


def compose_up(build: bool = True) -> None:
    cmd = ["docker", "compose", "-f", str(COMPOSE_FILE), "up", "-d"]
    if build:
        cmd.append("--build")
    run(cmd)


def wait_kafka(timeout: int = 240) -> bool:
  print("Waiting for Kafka topic initialization...")
  deadline = time.time() + timeout
  while time.time() < deadline:
    try:
      result = subprocess.run(
        [
          "docker", "compose", "-f", str(COMPOSE_FILE),
          "exec", "-T", "kafka",
          "kafka-topics", "--bootstrap-server", "localhost:29092", "--list",
        ],
        capture_output=True,
        text=True,
        cwd=ROOT,
      )
      topics = result.stdout.strip().split("\n")
      required = {
        "raw-detections", "store-events", "session-events",
        "metrics-events", "anomaly-events", "dashboard-events",
      }
      if required.issubset(set(topics)):
        print("Kafka topics ready:", ", ".join(sorted(required)))
        return True
    except Exception:
      pass
    time.sleep(3)
  print("WARN: Kafka topics not confirmed within timeout")
  subprocess.run(
    ["docker", "compose", "-f", str(COMPOSE_FILE), "ps", "-a", "kafka", "kafka-init"],
    cwd=ROOT,
    check=False,
  )
  return False


def check_health(timeout: int = 180) -> dict[str, bool]:
    results: dict[str, bool] = {}
    deadline = time.time() + timeout
    pending = set(HEALTH_CHECKS)

    while pending and time.time() < deadline:
        for name, url in list(pending):
            try:
                req = urllib.request.Request(url, method="GET")
                with urllib.request.urlopen(req, timeout=3) as resp:
                    if resp.status == 200:
                        results[name] = True
                        pending.discard((name, url))
                        print(f"  OK {name}")
            except (urllib.error.URLError, TimeoutError, http.client.RemoteDisconnected, ConnectionError):
                pass
        if pending:
            time.sleep(4)

    for name, url in pending:
        results[name] = False
        print(f"  FAIL {name} (not ready)")
    return results


def stream_logs(follow: bool = True) -> None:
    cmd = ["docker", "compose", "-f", str(COMPOSE_FILE), "logs", "-f", "--tail=50"]
    if not follow:
        cmd.remove("-f")
    try:
        subprocess.run(cmd, cwd=ROOT)
    except KeyboardInterrupt:
        print("\nLog streaming stopped.")


def ensure_data_dir() -> None:
    data = ROOT / "data"
    data.mkdir(exist_ok=True)
    print("NOTE: Upload CCTV footage from the dashboard to start inference.")


def main() -> int:
    parser = argparse.ArgumentParser(description="Retail Intelligence Platform orchestrator")
    parser.add_argument("--no-build", action="store_true", help="Skip image rebuild")
    parser.add_argument("--no-open", action="store_true", help="Do not open browser")
    parser.add_argument("--logs-only", action="store_true", help="Only stream logs")
    parser.add_argument("--down", action="store_true", help="Stop all services")
    args = parser.parse_args()

    if not docker_available():
        print("ERROR: Docker and Docker Compose are required.", file=sys.stderr)
        return 1

    if args.down:
        run(["docker", "compose", "-f", str(COMPOSE_FILE), "down"])
        return 0

    if args.logs_only:
        stream_logs()
        return 0

    print("=" * 60)
    print("AI Retail Store Intelligence Platform")
    print("=" * 60)

    ensure_data_dir()
    compose_up(build=not args.no_build)
    if not wait_kafka():
        print("ERROR: Kafka did not become ready. Check `docker compose logs kafka`.", file=sys.stderr)
        return 1
    print("\nVerifying service health...")
    health = check_health()
    ok_count = sum(1 for v in health.values() if v)

    print(f"\n{ok_count}/{len(HEALTH_CHECKS)} services healthy")
    print(f"Dashboard: {DASHBOARD_URL}")
    print("API:       http://localhost:8080")
    print("Grafana:   http://localhost:3001 (admin/admin)")

    if ok_count != len(HEALTH_CHECKS):
        print("ERROR: One or more application services did not become healthy.", file=sys.stderr)
        return 1

    if not args.no_open:
        try:
            webbrowser.open(DASHBOARD_URL)
        except Exception:
            pass

    print("\nStreaming logs (Ctrl+C to exit)...\n")
    stream_logs()
    return 0


if __name__ == "__main__":
    sys.exit(main())
