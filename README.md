# AI Retail Store Intelligence Platform

Real-time CCTV analytics for physical retail stores. The platform accepts uploaded store footage, detects and tracks visitors, understands store zones, calculates live operational metrics, raises anomalies, and presents the results in a live dashboard.

The goal is simple: give a physical store the kind of visibility that an online store gets from web analytics.

## Quick Start (5 Commands)

This is the complete setup to run the full system locally with Docker Compose:

```bash
# 1. Clone repository and navigate to root
cd /path/to/apex

# 2. Start all services (Kafka, PostgreSQL, Redis, detection, API gateway, frontend)
docker compose up -d

# 3. Verify all services are healthy (wait ~30 seconds for health checks to pass)
python run_system.py status

# 4. Open dashboard in browser
open http://localhost:3000
# (or on Windows: start http://localhost:3000)

# 5. Upload a CCTV test clip and watch real-time analytics
# - Click "Upload CCTV" on the dashboard
# - Select any MP4/AVI video file from tests/clips/ or your own
# - Dashboard switches to live analytics view showing visitor detections, zones, and metrics
```

**Done!** Your detection pipeline is running. You'll see:

- Live MJPEG stream of the video with bounding boxes
- Real-time visitor count, queue depth, conversion funnel
- Zone heatmap and anomaly feed

## Submission Deliverables

Mandatory challenge artifacts are included at the repository root unless noted:

- `README.md` - setup, architecture, API, and testing guide
- `DESIGN.md` - system design plus AI-Assisted Design Decisions
- `CHOICES.md` - model selection, schema design, and API architecture decisions
- `data/events.jsonl` - valid JSONL event log following `shared/events/schema.json`

To clean up:

```bash
docker compose down
```

## Running Detection Pipeline Against Clips

If you want to test the detection pipeline directly (without the full system):

```bash
# 1. Start only Kafka + Redis (needed for event publishing)
docker compose up -d kafka redis

# 2. Run detection service in foreground (press Ctrl+C to stop)
docker compose up detection-service

# 3. In another terminal, upload a test clip to the detection service
curl -X POST \
  -F "file=@path/to/test_video.mp4" \
  http://localhost:8000/upload

# 4. Watch the MJPEG stream in browser
open http://localhost:8000/stream
```

You can also run the detection pipeline locally (requires Python 3.10+, CUDA-capable GPU recommended):

```bash
# Install dependencies
cd services/detection-service
pip install -r requirements.txt

# Set environment variables
export KAFKA_BROKERS=localhost:9092
export UPLOAD_DIR=./uploads
export MODEL_PATH=yolov8m.pt

# Run detection service
python -m app.main
```

## What The Website Does

The dashboard starts with an upload screen. After a CCTV video is uploaded, the backend begins inference and the website switches to a live operations view.

The main video experience includes:

- Uploaded CCTV footage displayed as a live MJPEG stream.
- Visitor bounding boxes and stable visitor IDs.
- Zone labels such as `ENTRY`, `APPAREL`, `ELECTRONICS`, and `BILLING`.
- Detection confidence values.
- A floating `AI` button that opens the analytics panel.

The analytics panel includes:

- Live visitor count.
- Billing queue depth.
- Estimated conversion rate.
- Anomaly count.
- Re-entry count.
- Staff metric placeholder for future staff classification.
- Zone popularity leaderboard.
- Anomaly feed.
- Live event stream.
- Movement heatmap.
- Visitor journey view.
- System health status.

## Implemented Features

| Feature                       | Status          | How it works                                                                                                        |
| ----------------------------- | --------------- | ------------------------------------------------------------------------------------------------------------------- |
| CCTV video upload             | Implemented     | The frontend uploads footage through the API gateway. The detection service stores and processes the uploaded file. |
| Person detection              | Implemented     | YOLOv8 detects people in sampled video frames.                                                                      |
| Visitor tracking              | Implemented     | The tracker assigns stable visitor IDs across consecutive frames.                                                   |
| Lightweight re-identification | Implemented     | Color histogram embeddings help recognize returning visitors.                                                       |
| Zone analytics                | Implemented     | Each visitor location is mapped to a configured store zone.                                                         |
| Queue monitoring              | Implemented     | Visitors inside the `BILLING` zone contribute to queue depth.                                                       |
| Live metrics                  | Implemented     | Redis stores short-lived metrics and WebSockets push updates to the dashboard.                                      |
| Zone leaderboard              | Implemented     | Zone visits are ranked from highest to lowest activity.                                                             |
| Re-entry detection            | Implemented     | A returning visitor can be reported as a re-entry event.                                                            |
| Behavioral anomalies          | Implemented     | The detection pipeline can report loitering, erratic movement, and crowd conditions.                                |
| Operational anomalies         | Implemented     | The anomaly service checks queue spikes, suspicious patterns, stale feeds, dead zones, and conversion drops.        |
| Movement heatmap              | Implemented     | Recent visitor centroids are aggregated into a grid for visualization.                                              |
| PostgreSQL event history      | Implemented     | The ingestion service stores deduplicated events for durable history.                                               |
| Staff classification          | Extension point | The UI contains a staff metric, but the current model does not classify staff members separately.                   |

## User Flow

```mermaid
flowchart TD
    A[Open dashboard] --> B[Upload CCTV footage]
    B --> C[API gateway accepts multipart upload]
    C --> D[Detection service stores uploaded video]
    D --> E[YOLOv8 inference starts]
    E --> F[Track visitors and resolve zones]
    F --> G[Publish store events to Kafka]
    G --> H[Calculate sessions, metrics, and anomalies]
    H --> I[Publish Redis dashboard updates]
    I --> J[WebSocket pushes updates to browser]
    E --> K[MJPEG stream and overlay API]
    K --> J
```

## System Architecture

```mermaid
flowchart LR
    subgraph Browser
        UI[React Dashboard]
        Canvas[Canvas Overlay]
        Panel[AI Analytics Panel]
    end

    subgraph Edge
        Gateway[API Gateway :8080]
    end

    subgraph Analytics Services
        Detection[Detection Service :8000]
        Session[Session Service :8081]
        Metrics[Metrics Service :8082]
        Anomaly[Anomaly Service :8083]
        Ingestion[Ingestion Service :8084]
    end

    subgraph Event Backbone
        Kafka[(Kafka)]
        ZooKeeper[(ZooKeeper)]
    end

    subgraph Storage
        Redis[(Redis)]
        Postgres[(PostgreSQL)]
    end

    subgraph Monitoring
        Prometheus[Prometheus :9090]
        Grafana[Grafana :3001]
    end

    UI -->|REST upload and health| Gateway
    UI <-->|WebSocket updates| Gateway
    Canvas -->|overlay and heatmap polling| Gateway
    Gateway -->|proxy upload and stream| Detection
    Detection -->|store-events| Kafka
    Kafka --> Session
    Kafka --> Metrics
    Kafka --> Anomaly
    Kafka --> Ingestion
    Session -->|session-events| Kafka
    Metrics -->|metrics-events| Kafka
    Anomaly -->|anomaly-events| Kafka
    Session <--> Redis
    Metrics <--> Redis
    Anomaly <--> Redis
    Ingestion --> Postgres
    Gateway <--> Redis
    Kafka --- ZooKeeper
    Prometheus --> Gateway
    Grafana --> Prometheus
```

## Service Responsibilities

| Service           | Technology                                  | Default port                  | Responsibility                                                                                                                 | Why it is used                                                                             |
| ----------------- | ------------------------------------------- | ----------------------------- | ------------------------------------------------------------------------------------------------------------------------------ | ------------------------------------------------------------------------------------------ |
| Frontend          | React, Vite, Tailwind CSS                   | `3000`                        | Upload experience, video display, overlays, analytics panel, heatmap, and WebSocket updates.                                   | React keeps the UI component-based and responsive while metrics update continuously.       |
| API gateway       | Go, Gin, Gorilla WebSocket                  | `8080`                        | Single browser-facing entry point, CORS handling, upload proxying, REST endpoints, health checks, and dashboard WebSockets.    | A gateway gives the frontend one stable API and keeps internal services private.           |
| Detection service | Python, FastAPI, OpenCV, Ultralytics YOLOv8 | `8000`                        | Video ingestion, person detection, tracking, re-identification, zones, anomalies, overlays, heatmap data, and MJPEG streaming. | Python has mature computer vision libraries and direct YOLO integration.                   |
| Session service   | Go                                          | `8081`                        | Builds visitor session state from store events and distinguishes zone changes from actual store exits.                         | Session logic is stateful and benefits from Go's lightweight concurrent processing.        |
| Metrics service   | Go                                          | `8082`                        | Produces active visitor metrics, queue depth, conversion estimates, and zone leaderboard data.                                 | Metrics processing is event-driven and efficient as a small Go service.                    |
| Anomaly service   | Go                                          | `8083`                        | Detects operational problems such as queue spikes, stale feeds, dead zones, and conversion drops.                              | Keeping anomaly rules separate makes them easier to evolve independently.                  |
| Ingestion service | Go                                          | `8084`                        | Deduplicates events and persists durable records in PostgreSQL.                                                                | Durable storage is isolated from the real-time path so dashboard updates stay fast.        |
| Kafka             | Apache Kafka                                | `9092` host, `29092` internal | Connects services through asynchronous events.                                                                                 | Services can process events independently without tightly coupling their runtime behavior. |
| Redis             | Redis                                       | `6379`                        | Stores live session state, short-lived metrics, leaderboards, and dashboard pub/sub updates.                                   | Redis is a good fit for fast, expiring operational data.                                   |
| PostgreSQL        | PostgreSQL                                  | `5432`                        | Stores historical event records.                                                                                               | PostgreSQL provides durable, queryable persistence.                                        |
| Prometheus        | Prometheus                                  | `9090`                        | Collects service health and operational metrics.                                                                               | Prometheus is a standard monitoring backend for containerized services.                    |
| Grafana           | Grafana                                     | `3001`                        | Visualizes Prometheus monitoring data.                                                                                         | Grafana provides an extensible operations view beyond the product dashboard.               |

## Video Processing Pipeline

```mermaid
flowchart TD
    A[Uploaded video file] --> B[OpenCV frame reader]
    B --> C[Resize and sample frames]
    C --> D[YOLOv8 person detection]
    D --> E[Visitor tracker]
    E --> F[Color histogram re-identification]
    F --> G[Zone resolver]
    G --> H[Queue, journey, and anomaly rules]
    H --> I[Kafka store-events]
    H --> J[Latest overlay state]
    H --> K[Movement heatmap]
    B --> L[MJPEG preview stream]
```

### Why frame sampling is used

Running inference on every frame can be unnecessarily slow on CPU-only systems. The detection service samples frames for inference while still producing a watchable preview stream. This keeps the dashboard responsive on ordinary development machines.

### Visitor IDs and re-entry

The platform uses two layers:

1. Short-term tracking keeps a visitor ID stable while the person remains visible across nearby frames.
2. Lightweight color histogram matching helps associate a returning person with an earlier identity.

This is intentionally a lightweight development implementation. A production deployment can replace it with a dedicated person re-identification model when accuracy across cameras, lighting changes, and clothing similarity becomes important.

### Zone handling

Zones are defined in a normalized reference layout and scaled to the actual video dimensions. Each tracked visitor is mapped to a zone from their position in the frame. A zone exit means that a visitor moved between store areas; it does not automatically mean the visitor left the store.

## Event-Driven Data Flow

```mermaid
sequenceDiagram
    participant CV as Detection Service
    participant K as Kafka
    participant S as Session Service
    participant M as Metrics Service
    participant A as Anomaly Service
    participant I as Ingestion Service
    participant R as Redis
    participant G as API Gateway
    participant B as Browser

    CV->>K: Publish store-events
    K->>S: Consume store-events
    S->>K: Publish session-events
    K->>M: Consume session-events
    M->>R: Store live metrics and leaderboard
    M->>K: Publish metrics-events
    K->>A: Consume metrics-events
    A->>K: Publish anomaly-events
    K->>I: Consume all persisted event topics
    I->>I: Deduplicate and write PostgreSQL history
    R->>G: Publish dashboard update
    G->>B: Send WebSocket message
    B->>G: Poll overlay and heatmap endpoints
    G->>CV: Proxy detection endpoints
```

## Kafka Topics

| Topic              | Purpose                                                                                |
| ------------------ | -------------------------------------------------------------------------------------- |
| `raw-detections`   | Reserved stream for lower-level detection records.                                     |
| `store-events`     | Zone, queue, re-entry, crowd, and behavioral events emitted by the detection pipeline. |
| `session-events`   | Visitor session lifecycle events produced by the session service.                      |
| `metrics-events`   | Aggregated operational metrics used by the anomaly service.                            |
| `anomaly-events`   | Operational anomaly records.                                                           |
| `dashboard-events` | Reserved topic for dashboard-oriented event delivery.                                  |

Kafka is configured with separate listeners:

- Containers use `kafka:29092`.
- Tools running on the host machine use `localhost:9092`.

This avoids the common Docker networking issue where containers receive an unreachable host address from Kafka.

## Live State In Redis

| Key or channel              | Lifetime        | Purpose                                 |
| --------------------------- | --------------- | --------------------------------------- |
| `session:{store}:{visitor}` | 2 hours         | Current visitor session state.          |
| `metrics:live:{store}`      | 30 seconds      | Latest live metrics snapshot.           |
| `leaderboard:{store}`       | 30 seconds      | Current zone popularity leaderboard.    |
| `dashboard:updates`         | Pub/sub channel | Real-time browser update notifications. |

Redis values are intentionally short-lived because the dashboard represents the current store state. Historical records belong in PostgreSQL.

## Dashboard Metrics

| Metric     | Meaning                                                                                                            |
| ---------- | ------------------------------------------------------------------------------------------------------------------ |
| Visitors   | Current visible tracked visitors. The UI derives this from the latest overlay for immediate feedback.              |
| Queue      | Visitors currently resolved to the `BILLING` zone.                                                                 |
| Conversion | Estimated conversion metric calculated from live events and capped below `100%` until checkout integration exists. |
| Anomalies  | Count of anomaly events currently reported by the pipeline and anomaly service.                                    |
| Re-entry   | Visitors recognized after returning.                                                                               |
| Staff      | Reserved UI tile for a future staff-specific model or roster integration.                                          |

## Anomaly Detection

The platform has two anomaly layers.

### Computer vision anomalies

Generated close to the detection pipeline:

- Loitering behavior.
- Erratic movement.
- Crowd conditions when the visible track count crosses the configured threshold.

### Operational anomalies

Generated by the anomaly service:

- Queue spike when billing queue depth reaches the configured rule threshold.
- Suspicious zone patterns.
- Conversion drop.
- Stale camera feed.
- Dead zone with no recent activity.

Keeping these layers separate lets the CV pipeline focus on what is visible in the footage while the anomaly service focuses on business rules over time.

## API Endpoints

The frontend normally uses the API gateway at `http://localhost:8080`.

| Method | Endpoint                | Purpose                                                   |
| ------ | ----------------------- | --------------------------------------------------------- |
| `GET`  | `/health`               | API gateway health status.                                |
| `GET`  | `/metrics`              | Gateway metrics endpoint.                                 |
| `GET`  | `/ws`                   | WebSocket endpoint for live dashboard updates.            |
| `POST` | `/video/upload`         | Upload CCTV footage for analysis.                         |
| `GET`  | `/detection/overlay`    | Latest visitor boxes, IDs, confidence, and zone data.     |
| `GET`  | `/detection/heatmap`    | Current movement heatmap grid from the detection service. |
| `GET`  | `/system/health`        | Combined system health check.                             |
| `GET`  | `/stores/:id/metrics`   | Latest metrics snapshot for a store.                      |
| `GET`  | `/stores/:id/funnel`    | Store funnel API extension point.                         |
| `GET`  | `/stores/:id/heatmap`   | Store-level heatmap API extension point.                  |
| `GET`  | `/stores/:id/anomalies` | Store anomaly REST API extension point.                   |
| `POST` | `/events/ingest`        | Direct event ingestion endpoint.                          |

The detection service also exposes its internal FastAPI endpoints inside the Docker network, including the MJPEG video stream used by the frontend through the gateway.

## Technology Choices

| Technology             | Reason                                                                                        |
| ---------------------- | --------------------------------------------------------------------------------------------- |
| YOLOv8 Nano            | Fast person detection with a small model suitable for a development laptop.                   |
| OpenCV                 | Reliable video decoding, resizing, frame processing, and JPEG streaming.                      |
| FastAPI                | Lightweight Python API layer around the computer vision pipeline.                             |
| Go                     | Small, efficient microservices with clear concurrency behavior and fast startup.              |
| Kafka                  | Event backbone for loosely coupled processing and future horizontal scaling.                  |
| Redis                  | Low-latency state and pub/sub for a real-time dashboard.                                      |
| PostgreSQL             | Durable event history and future reporting queries.                                           |
| React                  | Component-driven browser interface that updates without page reloads.                         |
| Canvas overlays        | Draws detection boxes over streamed footage without re-encoding annotations into every frame. |
| Docker Compose         | Reproducible local environment for the full multi-service stack.                              |
| Prometheus and Grafana | Standard observability tooling for service monitoring.                                        |

## Quick Start

### Prerequisites

- Docker Desktop with Docker Compose.
- Python 3 installed on the host machine.
- At least 4 GB of free memory for the local stack.

The first computer vision startup can take longer because the YOLO model may need to be downloaded.

### Start the full system

```bash
python run_system.py
```

The startup script:

1. Builds and starts the Docker Compose services.
2. Waits for Kafka to become healthy.
3. Waits for required Kafka topics to exist.
4. Checks application health endpoints.
5. Opens the dashboard unless disabled.

### Useful startup options

```bash
python run_system.py --no-build
python run_system.py --no-open
python run_system.py --logs-only
python run_system.py --down
```

### Open the application

| Page                   | URL                                   |
| ---------------------- | ------------------------------------- |
| Retail dashboard       | `http://localhost:3000`               |
| API gateway health     | `http://localhost:8080/health`        |
| Combined system health | `http://localhost:8080/system/health` |
| Grafana                | `http://localhost:3001`               |
| Prometheus             | `http://localhost:9090`               |

Upload an `.mp4`, `.avi`, `.mov`, `.mkv`, or `.webm` CCTV clip from the dashboard to start analysis.

## Manual Docker Commands

Use these commands when you want direct control over the stack:

```bash
docker compose up -d --build
docker compose ps
docker compose logs -f detection-service
docker compose down
```

To reuse already-built images:

```bash
docker compose up -d --no-build
```

## Configuration

Copy `.env.example` to `.env` when you want to override defaults.

| Variable                   | Default                                                               | Purpose                                                         |
| -------------------------- | --------------------------------------------------------------------- | --------------------------------------------------------------- |
| `STORE_ID`                 | `STORE_BLR_002`                                                       | Store identifier attached to events.                            |
| `CAMERA_ID`                | `CAM_ENTRY_01`                                                        | Camera identifier attached to video events.                     |
| `KAFKA_BROKERS`            | `kafka:29092`                                                         | Kafka address used inside Docker.                               |
| `REDIS_URL`                | `redis://redis:6379/0`                                                | Redis connection string.                                        |
| `POSTGRES_DSN`             | `postgres://retail:retail@postgres:5432/retail_intel?sslmode=disable` | PostgreSQL connection string.                                   |
| `YOLO_MODEL`               | `yolov8n.pt`                                                          | Ultralytics YOLO model file.                                    |
| `DETECTION_CONF`           | `0.5`                                                                 | Minimum detection confidence.                                   |
| `REID_THRESHOLD`           | `0.65`                                                                | Similarity threshold for lightweight re-identification.         |
| `INFERENCE_SIZE`           | `320`                                                                 | YOLO input size. Smaller values improve speed.                  |
| `INFERENCE_FRAME_INTERVAL` | `5`                                                                   | Run inference once every N frames. Larger values improve speed. |
| `EVENT_INTERVAL_SEC`       | `1.0`                                                                 | Minimum interval between repeated event emissions.              |
| `STREAM_FPS`               | `10`                                                                  | Target preview stream frame rate.                               |
| `STREAM_WIDTH`             | `1280`                                                                | Maximum preview stream width.                                   |
| `MAX_VIDEO_FPS`            | `15`                                                                  | Maximum offline footage processing rate.                        |
| `TORCH_NUM_THREADS`        | `2`                                                                   | CPU threads used by PyTorch inference.                          |

## Performance Tuning

The defaults favor a responsive CPU-based demo. When inference is slow:

1. Increase `INFERENCE_FRAME_INTERVAL` from `5` to `8` or `10`.
2. Reduce `INFERENCE_SIZE` from `320` to a smaller supported size.
3. Reduce `STREAM_WIDTH`.
4. Reduce `STREAM_FPS`.
5. Use a CUDA-enabled environment for GPU inference in a production deployment.

Increasing speed can reduce detection accuracy or visual smoothness, so tune against the actual camera angle and store layout.

## Startup Architecture

```mermaid
flowchart TD
    A[Docker Compose starts infrastructure] --> B[ZooKeeper]
    B --> C[Kafka broker]
    C --> D{Kafka health check passes?}
    D -->|No| C
    D -->|Yes| E[kafka-init creates required topics]
    E --> F{Topic initialization complete?}
    F -->|No| E
    F -->|Yes| G[Start application services]
    G --> H[run_system.py verifies service health]
    H --> I[Dashboard ready]
```

The application services wait for Kafka health and topic initialization before they start. This prevents intermittent failures during local startup.

## Project Structure

```text
.
|-- frontend/                         React dashboard
|   |-- src/components/               Video, overlays, analytics panel
|   `-- src/hooks/                    WebSocket and polling hooks
|-- services/
|   |-- api-gateway/                  Browser-facing Go API
|   |-- detection-service/            Python CV pipeline
|   |-- session-service/              Visitor session state
|   |-- metrics-service/              Live store metrics
|   |-- anomaly-service/              Operational anomaly rules
|   `-- ingestion-service/            Durable event ingestion
|-- shared/
|   |-- events/                       Shared event contracts
|   |-- kafka/                        Kafka helpers
|   |-- redis/                        Redis helpers
|   `-- observability/                Metrics and tracing helpers
|-- database/migrations/              PostgreSQL schema migrations
|-- observability/                    Prometheus and Grafana config
|-- kubernetes/                       Starter Kubernetes manifests
|-- docker-compose.yml                Local full-stack orchestration
|-- run_system.py                     Reliable local startup script
`-- .env.example                      Runtime tuning examples
```

## Data Ownership

```mermaid
flowchart LR
    Video[Uploaded footage] --> Detection[Detection Service]
    Detection --> Live[Current overlay and heatmap memory]
    Detection --> Kafka[(Kafka event stream)]
    Kafka --> Redis[(Redis live state)]
    Kafka --> Postgres[(PostgreSQL history)]
    Redis --> Dashboard[Live dashboard]
    Postgres --> Reports[Historical reporting extension]
```

- The detection service owns the active video source and current overlay state.
- Redis owns fast live state used by the dashboard.
- PostgreSQL owns durable history.
- Kafka owns event delivery between services, not long-term business reporting.

## Observability

The stack includes:

- Health endpoints for each application service.
- A combined system health endpoint at `/system/health`.
- Prometheus configuration for service scraping.
- Grafana for infrastructure and service dashboards.
- Container logs through Docker Compose.

Useful commands:

```bash
docker compose ps
docker compose logs -f api-gateway
docker compose logs -f detection-service
docker compose logs -f kafka
```

## Kubernetes

The `kubernetes/` directory contains starter deployment assets such as namespace, configuration, secrets, ingress, and selected service manifests. Treat these as a foundation for deployment packaging. A production rollout should add the complete set of microservice deployments, persistent volumes, resource limits, autoscaling policies, TLS, and managed Kafka, Redis, and PostgreSQL where appropriate.

## Troubleshooting

### The upload screen remains visible

Check the gateway and detection logs:

```bash
docker compose logs -f api-gateway
docker compose logs -f detection-service
```

The upload screen stays visible until the backend confirms that inference has started.

### Video is visible but analytics take time to populate

The first few frames establish tracks and zone state. CPU inference is intentionally sampled. Use the performance tuning settings above if updates are too slow.

### Kafka is not ready

Run:

```bash
docker compose logs kafka
docker compose logs kafka-init
docker compose ps
```

The Compose configuration waits for Kafka health and required topics before starting application services. If an old local Kafka state causes a broker ID conflict during development, stop the stack and remove only the project-specific Docker volumes before restarting.

### The YOLO model is missing

The detection container downloads `yolov8n.pt` on first use when network access is available. A production image should bundle an approved model artifact instead of downloading at runtime.

### Browser metrics look stale

Refresh the page and check `http://localhost:8080/system/health`. The frontend reconnects WebSockets automatically, but a hard refresh is useful after rebuilding frontend assets.

## Current Limitations And Production Next Steps

This repository is a working end-to-end retail analytics platform and a strong development prototype. Before a production rollout:

- Replace lightweight color histogram re-identification with a dedicated model.
- Add camera-specific zone configuration through an admin interface.
- Add checkout or POS integration for true conversion measurement.
- Train or integrate staff classification before using the staff metric.
- Add authentication, authorization, upload limits, and secure object storage.
- Add full multi-camera identity handling.
- Add complete Kubernetes manifests and managed infrastructure.
- Expand automated integration and load tests using representative footage.
- Review privacy, retention, and consent requirements for the deployment location.

## Author

Vedula Uday Easwar
