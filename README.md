FlyCast — Lightweight video streaming app with no bloat. Built for QNAP servers with Intel processors.



![FlyCast - Library View](/docs/screenshot.jpg)

## Features
- Direct Play + optional transcoding (FFmpeg)
- Quality selection with bitrate display (dropdown: Original, 25%, 50%, 75%, 100%)
- Playback speed control (0.5x–2.0x)
- Seek ±10s (works for transcoded streams)
- Progress bar with correct total duration (for transcoded streams)
- User profiles
- Albums / folders with thumbnails generated during scanning
- Basic metadata agent (TMDB – modular for additional services)
- SQLite, single Go binary + Vue 3 SPA
- HTTP port 8100

## Quick Start (local)
```bash
# backend
cd backend
go mod tidy
go run ./cmd/flycast

# frontend (dev)
cd frontend
npm i
npm run dev
```

## Build QPKG
```bash
bash build.sh
```

## Environment Variables
- `FLYCAST_PORT=8100`
- `FLYCAST_HOME=/share/CACHEDEV1_DATA/.qpkg/FlyCast/shared`
- `FLYCAST_WEBROOT=/share/CACHEDEV1_DATA/.qpkg/FlyCast/app/web`
- `FLYCAST_LIBRARY=/path/to/movies,/path/to/series`
- `FFMPEG_PATH=ffmpeg`
- `FFPROBE_PATH=ffprobe`
- `FLYCAST_JWT_SECRET` — randomly generated on first startup if not set (printed to stderr)

## Build
```bash
bash build.sh          # frontend + Go + FFmpeg + QPKG
```

Generates `FlyCast_<version>.qpkg` ready to install on QNAP.

## Project Structure
```
FlyCast/
├── backend/            # Go (Fiber) backend
│   ├── cmd/flycast/    # entrypoint
│   └── internal/       # api, auth, config, db, scanner
├── frontend/           # Vue 3 + Vite + Video.js
│   └── src/
├── build/              # build scripts and QPKG
├── QPKG/               # QNAP package configuration
└── scripts/
```
