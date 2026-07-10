# mouyin-server

Mouyin-compatible local backend prototype. It exposes the API shape expected by the patched Mouyin APK and uses Qishui/Soda Music as an upstream provider where possible, with local mock fallbacks for UI stability.

## Run locally

```powershell
go run ./cmd/mouyin-server
```

Default listen address:

```text
:8000
```

Override:

```powershell
$env:ADDR=':8000'
go run ./cmd/mouyin-server
```

Optional Qishui headers/cookies:

```powershell
$env:QISHUI_X_HELIOS='...'
$env:QISHUI_X_MEDUSA='...'
$env:QISHUI_COOKIE='...'
```

If the computer cannot directly access Qishui/Douyin CDN, route upstream requests through v2rayN or another local proxy:

```powershell
$env:UPSTREAM_PROXY='http://127.0.0.1:10808'
.\mouyin-server-windows-amd64.exe
```

Audio playback is returned to the app as a local proxy URL:

```text
http://<server-host>:8000/api/proxy/audio/{track_id}
```

The proxy supports HTTP `Range` requests. If Qishui returns encrypted audio plus `play_auth`/`spade_a`, the server downloads the upstream m4a, decrypts it with `music-lib/soda.DecryptAudio`, caches it under `%TEMP%\mouyin-server-cache` by default, and serves the decrypted audio with `http.ServeContent`.

The proxy tries multiple upstream candidates when available:

- `url_player_info` main URL
- `url_player_info` backup URL
- `video_model` main URL
- `video_model` backup URL

Debug a track and its candidate audio URLs:

```powershell
Invoke-RestMethod 'http://127.0.0.1:8000/api/debug/song/7146240707408168993'
```

Probe each candidate audio URL with `Range: bytes=0-1023`:

```powershell
Invoke-RestMethod 'http://127.0.0.1:8000/api/debug/proxy/7146240707408168993'
```

Also verify full encrypted-audio download, decrypt, and cache:

```powershell
Invoke-RestMethod 'http://127.0.0.1:8000/api/debug/proxy/7146240707408168993?decrypt=1'
```

Override cache directory:

```powershell
$env:MOUYIN_CACHE_DIR='D:\mouyin-cache'
```

## Run with Docker Compose

The published Docker image is:

```text
ghcr.io/xiaochen201807/mouyin-server:latest
```

Because this GitHub repository is private, pull it from the server after logging in to GHCR:

```bash
echo '<github_pat_or_token>' | docker login ghcr.io -u xiaochen201807 --password-stdin
docker pull ghcr.io/xiaochen201807/mouyin-server:latest
```

Create `compose.yaml` on the server:

```yaml
services:
  mouyin-server:
    image: ghcr.io/xiaochen201807/mouyin-server:latest
    container_name: mouyin-server
    restart: unless-stopped
    ports:
      - "8000:8000"
    environment:
      ADDR: ":8000"
      MOUYIN_CACHE_DIR: "/var/cache/mouyin"
      UPSTREAM_PROXY: "${UPSTREAM_PROXY:-}"
      TZ: "Asia/Shanghai"
    volumes:
      - ./data/cache:/var/cache/mouyin
```

Start it:

```bash
docker compose up -d
```

The service listens on the server at:

```text
http://<server-ip>:8000/
```

Health check:

```bash
curl http://127.0.0.1:8000/api/health
```

Playback proxy check:

```bash
curl -I -H 'Range: bytes=0-1023' \
  http://127.0.0.1:8000/api/proxy/audio/7146240707408168993
```

If the server also needs an upstream HTTP proxy, create `.env` before starting Compose:

```bash
UPSTREAM_PROXY=http://host.docker.internal:10808
```

`host.docker.internal` is mapped by `compose.yaml` to the Docker host gateway. Do not use `127.0.0.1` for a proxy running on the host, because inside the container `127.0.0.1` points to the container itself.

If you want to build from source instead of pulling GHCR:

```bash
git clone https://github.com/xiaochen201807/mouyin-server.git
cd mouyin-server
docker compose -f compose.yaml -f compose.build.yaml up -d --build
```

Useful Compose commands:

```bash
docker compose logs -f
docker compose restart
docker compose down
```

## Endpoints

- `GET /api/health`
- `GET /api/app/version`
- `GET /api/splash`
- `GET /api/genres`
- `GET /api/listen_mode/modes`
- `GET /api/search?keyword=周杰伦&page=1&page_size=20`
- `GET /api/recommend`
- `GET /api/song/{id}`
- `GET /api/proxy/audio/{id}`
- `GET /api/debug/song/{id}`
- `GET /api/debug/proxy/{id}`
- `GET /api/discover/playlists`
- `GET /api/playlist/{id}`
- `GET /api/mv/list`
- `GET /api/video_feed`
- `POST /account/login_password`
- `GET /account/me`

## GitHub Actions

`.github/workflows/build.yml` builds Windows/Linux/macOS binaries for amd64/arm64. Normal pushes only compile and test. Because this account currently has full GitHub Actions artifact storage, the workflow intentionally does not use `actions/upload-artifact`; on tags like `v0.1.0`, it uploads binaries directly to GitHub Releases.

