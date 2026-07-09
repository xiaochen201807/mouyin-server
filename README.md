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

## Endpoints

- `GET /api/health`
- `GET /api/app/version`
- `GET /api/splash`
- `GET /api/genres`
- `GET /api/listen_mode/modes`
- `GET /api/search?keyword=周杰伦&page=1&page_size=20`
- `GET /api/recommend`
- `GET /api/song/{id}`
- `GET /api/discover/playlists`
- `GET /api/playlist/{id}`
- `GET /api/mv/list`
- `GET /api/video_feed`
- `POST /account/login_password`
- `GET /account/me`

## GitHub Actions

`.github/workflows/build.yml` builds Windows/Linux/macOS binaries for amd64/arm64. Normal pushes only compile and test. Because this account currently has full GitHub Actions artifact storage, the workflow intentionally does not use `actions/upload-artifact`; on tags like `v0.1.0`, it uploads binaries directly to GitHub Releases.

