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

