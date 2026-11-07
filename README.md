# FlyCast

Lekka aplikacja wideo typu Plex/Emby bez zbędnych wodotrysków.

## Cechy
- Direct Play + opcjonalne transkodowanie (FFmpeg)
- Zmiana prędkości odtwarzania (0.5x–2.0x)
- Przewijanie +10s / +30s / -10s / -30s
- Profile użytkowników
- Albumy / foldery z miniaturkami generowanymi przy skanowaniu
- Podstawowy agent metadata (TMDB – modułowy pod dalsze serwisy)
- SQLite, jeden binarny Go + SPA Vue 3
- Port HTTP 8100

## Szybki start (lokalny)
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
bash build/cross.sh
bash scripts/build_qpkg.sh
```

## Zmienne środowiskowe
- FLYCAST_PORT=8100
- FLYCAST_HOME=/share/CACHEDEV1_DATA/.qpkg/FlyCast/shared
- FLYCAST_WEBROOT=/share/CACHEDEV1_DATA/.qpkg/FlyCast/app/web
- FLYCAST_LIBRARY=/path/to/movies,/path/to/series
- FFMPEG_PATH=ffmpeg
- FFPROBE_PATH=ffprobe
