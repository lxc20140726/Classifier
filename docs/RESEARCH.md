# Technical Research / 技术研究

## Compression Findings

- `zstd` delivers the highest throughput in the current comparison at about `510 MB/s`.
- `zip` offers broad compatibility and balanced performance at about `105 MB/s`.
- `7z` achieves stronger compression ratios, but the trade-off is much slower execution.

## Classification Strategy

- Recommended pipeline: metadata-first, then content-based fallback.
- Metadata-first stage should inspect file extensions and magic bytes to avoid unnecessary deep scans.
- Content-based analysis can be reserved for ambiguous folders that need deeper inspection.

## Batch Processing Model

- Use a task queue to isolate folder-level jobs.
- Apply bounded concurrency to prevent NAS resource spikes.
- Emit structured progress events so the web UI can track state changes clearly.

## Reference Projects

- `filebrowser` - about `33k` GitHub stars, built with Go, strong reference for file management UX and deployment simplicity.
- `spacedrive` - about `37k` GitHub stars, built with Rust, useful reference for indexing, performance, and modern desktop-grade architecture.

## Recommendation

For this project, favor compatibility and operational simplicity first, then add deeper analysis only where metadata-based classification is not enough.
