# Classifier

Classifier is a web-based media folder organizer designed for NAS Docker deployments.

## Features

- Auto-classification: photo, video, mixed, manga (extension + ratio analysis)
- Node-based visual workflow editor (ComfyUI-style DAG)
- Batch renaming with token-based templates (no regex needed)
- Fast ZIP compression for image directories
- Emby-compatible video thumbnail generation (FFmpeg)
- File moving to target directories
- Undo/snapshot for all file operations
- Full audit log for all actions
- Concurrent folder processing
- NAS-friendly Docker deployment

## Documentation

### System Design
- [Architecture Overview](docs/ARCHITECTURE.md)
- [Technology Stack](docs/TECH_STACK.md)
- [Project Requirements](docs/REQUIREMENTS.md)

### Feature Specs
- [API Design](docs/API.md)
- [Data Models](docs/DATA_MODELS.md)
- [Frontend Design](docs/FRONTEND.md)
- [Node-based Workflow System](docs/WORKFLOW.md)
- [File Classification Algorithm](docs/CLASSIFICATION.md)
- [Rename Editor](docs/RENAME_EDITOR.md)
- [Emby Thumbnail Specs](docs/EMBY_THUMBNAILS.md)
- [Undo / Snapshot System](docs/SNAPSHOT.md)
- [Audit Log System](docs/AUDIT_LOG.md)

### Deployment
- [Docker Deployment Guide](docs/DEPLOYMENT.md)
- [极空间 Deployment Guide](docs/ZSPACE_DEPLOYMENT.md)

### Planning
- [Development Roadmap](docs/ROADMAP.md)
- [Technical Research](docs/RESEARCH.md)

## Local Development

### Prerequisites

- Go 1.23+
- Node.js 20+
- npm

### 1. Prepare local directories

```bash
mkdir -p .local/source .local/target .local/config
```

Put a few test folders under `.local/source/`, for example:

```text
.local/source/
  sample-album/
  sample-video/
```

### 2. Start backend

```bash
cd backend
CONFIG_DIR="$(pwd)/../.local/config" \
SOURCE_DIR="$(pwd)/../.local/source" \
TARGET_DIR="$(pwd)/../.local/target" \
PORT=8080 \
CGO_ENABLED=0 \
go run ./cmd/server
```

Backend endpoints:

- `http://localhost:8080/health`
- `http://localhost:8080/api/...`

### 3. Start frontend

Open another terminal:

```bash
cd frontend
npm install
npm run dev
```

Frontend dev URL:

- `http://localhost:5173`

Vite is already configured to proxy `/api` to `http://localhost:8080`.

### 4. Recommended local verification flow

1. Open `http://localhost:5173`
2. Go to `Settings`, confirm source and target paths
3. Return to `Folders`
4. Click `Scan source directory`
5. Check folder classification results in the table
6. Try category/status edits
7. Open snapshot drawer after move operations are available in your flow

### 5. Build commands

Backend:

```bash
cd backend
CGO_ENABLED=0 go build ./...
CGO_ENABLED=0 go test ./...
```

Frontend:

```bash
cd frontend
npm run typecheck
npm run lint
npm run build
```

## Development Status

Phase 1 MVP is implemented. Core backend, frontend, Docker deployment, snapshots, move flow, and 极空间 deployment docs are in place.
