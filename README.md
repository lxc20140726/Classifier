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

## Development Status

Early-stage design phase. Architecture and feature specs are being finalized before implementation.
