# Emby Thumbnail Specifications

## Naming Conventions

- Primary thumb: `{name}-thumb.jpg`
- Landscape image: `landscape.jpg`
- Backdrop image: `backdrop.jpg`

## Formats

- Preferred format: `JPEG`
- Use `PNG` only for assets that require transparency.

## Recommended Dimensions

- Thumb: `16:9`, commonly `1280x720`
- Backdrop: `1920x1080`

## FFmpeg Example

```bash
ffmpeg -i input.mp4 -ss 00:00:10 -frames:v 1 -vf "scale=1280:720:force_original_aspect_ratio=decrease,pad=1280:720:(ow-iw)/2:(oh-ih)/2" output-thumb.jpg
```

This example extracts a frame at 10 seconds and normalizes it to a 16:9 thumbnail size suitable for Emby-style media artwork.

## Folder Structure Guidance

- Movies: one movie per folder.
- TV series: organize by show, then by season subfolders.
- Keep artwork files next to the media files whenever possible so Emby can detect them automatically.
