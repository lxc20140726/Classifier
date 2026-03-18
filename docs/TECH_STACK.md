# Technology Recommendations / 技术栈推荐

## Backend

- Go: simpler deployment, strong concurrency model, good fit for NAS services.
- Rust: higher performance and memory safety, better when deep optimization matters.

## Frontend

- React: mature ecosystem, broad tooling support, flexible component model.
- Vue: approachable structure, fast development flow, strong community support.

## Compression

- ZIP libraries for compatibility with common archive tools.
- Zstandard for higher throughput when performance is more important than universal compatibility.

## Video Processing

- FFmpeg for thumbnail extraction, probing, and future media transformation features.

## Deployment

- Docker for packaging and NAS deployment.
- `docker-compose` for local orchestration and multi-service setups.

## Suggested Baseline

If the priority is shipping quickly on NAS, a practical starting point is Go + React + ZIP + FFmpeg + Docker.
