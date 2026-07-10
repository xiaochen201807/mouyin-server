package server

import (
    "bytes"
    "crypto/sha256"
    "encoding/json"
    "fmt"
    "io"
    "log"
    "net/http"
    "net/url"
    "os"
    "path/filepath"
    "strconv"
    "strings"
    "time"

    sodalib "github.com/guohuiyuan/music-lib/soda"
)

type App struct { upstream Upstream }

func New(upstream Upstream) *App { return &App{upstream: upstream} }

func (a *App) Routes() http.Handler {
    mux := http.NewServeMux()
    mux.HandleFunc("/api/health", a.health)
    mux.HandleFunc("/api/app/version", a.version)
    mux.HandleFunc("/api/splash", a.splash)
    mux.HandleFunc("/api/genres", a.genres)
    mux.HandleFunc("/api/listen_mode/modes", a.listenModes)
    mux.HandleFunc("/api/search", a.search)
    mux.HandleFunc("/api/recommend", a.recommend)
    mux.HandleFunc("/api/recommend/pool", a.recommend)
    mux.HandleFunc("/api/recommend/played", a.okPost)
    mux.HandleFunc("/api/recommend/location", a.okPost)
    mux.HandleFunc("/api/song/", a.song)
    mux.HandleFunc("/api/proxy/audio/", a.proxyAudio)
    mux.HandleFunc("/api/debug/song/", a.debugSong)
    mux.HandleFunc("/api/discover/playlists", a.discoverPlaylists)
    mux.HandleFunc("/api/playlist/", a.playlist)
    mux.HandleFunc("/api/mv/list", a.emptyList)
    mux.HandleFunc("/api/video_feed", a.emptyList)
    mux.HandleFunc("/api/video_feed/pool", a.emptyList)
    mux.HandleFunc("/account/me", a.accountMe)
    mux.HandleFunc("/account/login_password", a.accountLogin)
    mux.HandleFunc("/account/login_code", a.accountLogin)
    return withMiddleware(mux)
}

func withMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Access-Control-Allow-Origin", "*")
        w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Mouyin-Version, X-Mouyin-Signature, X-Mouyin-Timestamp")
        w.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
        if r.Method == http.MethodOptions { w.WriteHeader(http.StatusNoContent); return }
        log.Printf("%s %s", r.Method, r.URL.String())
        next.ServeHTTP(w, r)
    })
}

func writeJSON(w http.ResponseWriter, v interface{}) {
    w.Header().Set("Content-Type", "application/json; charset=utf-8")
    _ = json.NewEncoder(w).Encode(v)
}
func ok(w http.ResponseWriter, data interface{}) { writeJSON(w, BaseResponse{Code: 200, Msg: "ok", Data: data, Timestamp: nowMS()}) }
func list(w http.ResponseWriter, data interface{}, hasMore bool) { writeJSON(w, ListResponse{Code: 200, Msg: "ok", Data: data, HasMore: hasMore, Timestamp: nowMS()}) }

func (a *App) health(w http.ResponseWriter, r *http.Request) { ok(w, map[string]interface{}{"status":"ok", "service":"mouyin-server"}) }
func (a *App) version(w http.ResponseWriter, r *http.Request) { ok(w, map[string]interface{}{"version_code":151, "version_name":"1.6.1", "force_update":false}) }
func (a *App) splash(w http.ResponseWriter, r *http.Request) { ok(w, map[string]interface{}{"enabled":false, "items":[]interface{}{}}) }
func (a *App) genres(w http.ResponseWriter, r *http.Request) { ok(w, []map[string]string{{"id":"cn","name":"华语"},{"id":"pop","name":"流行"},{"id":"hot","name":"热歌"},{"id":"en","name":"英文"}}) }
func (a *App) listenModes(w http.ResponseWriter, r *http.Request) { ok(w, []map[string]string{{"id":"normal","name":"默认模式"},{"id":"focus","name":"专注模式"}}) }
func (a *App) okPost(w http.ResponseWriter, r *http.Request) { ok(w, map[string]bool{"ok":true}) }
func (a *App) emptyList(w http.ResponseWriter, r *http.Request) { list(w, []interface{}{}, false) }

func (a *App) search(w http.ResponseWriter, r *http.Request) {
    keyword := firstNonEmpty(r.URL.Query().Get("keyword"), r.URL.Query().Get("q"), "华语热歌")
    page := atoiDefault(firstNonEmpty(r.URL.Query().Get("page"), "1"), 1)
    size := atoiDefault(firstNonEmpty(r.URL.Query().Get("page_size"), r.URL.Query().Get("limit"), "20"), 20)
    tracks, hasMore, err := a.upstream.Search(keyword, page, size)
    if err != nil || len(tracks) == 0 { tracks = mockTracks(keyword); hasMore = false }
    list(w, tracks, hasMore)
}

func (a *App) recommend(w http.ResponseWriter, r *http.Request) {
    keywords := []string{"华语热歌", "流行", "周杰伦", "林俊杰", "邓紫棋"}
    keyword := keywords[int(nowMS())%len(keywords)]
    tracks, hasMore, err := a.upstream.Search(keyword, 1, 20)
    if err != nil || len(tracks) == 0 { tracks = mockTracks(keyword); hasMore = false }
    list(w, tracks, hasMore)
}

func (a *App) song(w http.ResponseWriter, r *http.Request) {
    id := strings.TrimPrefix(r.URL.Path, "/api/song/")
    if id == "" { ok(w, nil); return }
    tr, err := a.upstream.Song(id)
    if err != nil || tr == nil { tr = &mockTracks("fallback")[0]; tr.ID = id }
    if r.URL.Query().Get("direct") != "1" && tr.AudioURL != "" {
        proxyURL := proxyURLForRequest(r, id)
        tr.Extra = ensureExtra(tr.Extra)
        tr.Extra["direct_url"] = tr.AudioURL
        tr.PlayURL = proxyURL
        tr.AudioURL = proxyURL
    }
    ok(w, tr)
}

func (a *App) proxyAudio(w http.ResponseWriter, r *http.Request) {
    id := strings.TrimPrefix(r.URL.Path, "/api/proxy/audio/")
    if id == "" { http.Error(w, "missing id", http.StatusBadRequest); return }
    sources, err := a.audioSources(id)
    if err != nil || len(sources) == 0 {
        http.Error(w, "audio url unavailable", http.StatusBadGateway)
        return
    }

    var lastErr error
    for _, source := range sources {
        if source.URL == "" { continue }
        if source.PlayAuth != "" {
            if data, err := cachedDecryptedAudio(r, id, source.URL, source.PlayAuth); err == nil && len(data) > 0 {
                w.Header().Set("Content-Type", "audio/mp4")
                w.Header().Set("Accept-Ranges", "bytes")
                w.Header().Set("Cache-Control", "public, max-age=3600")
                http.ServeContent(w, r, id+".m4a", time.Now(), bytes.NewReader(data))
                return
            } else {
                lastErr = err
                log.Printf("decrypt/cache audio failed for %s via %s: %v; trying next source", id, source.Label, err)
            }
        }
        if err := a.transparentProxyAudio(w, r, source.URL); err == nil {
            return
        } else {
            lastErr = err
            log.Printf("transparent audio proxy failed for %s via %s: %v; trying next source", id, source.Label, err)
        }
    }
    if lastErr != nil { http.Error(w, lastErr.Error(), http.StatusBadGateway); return }
    http.Error(w, "audio proxy failed", http.StatusBadGateway)
}

func (a *App) transparentProxyAudio(w http.ResponseWriter, r *http.Request, directURL string) error {
    req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, directURL, nil)
    if err != nil { return err }
    req.Header.Set("User-Agent", "Mozilla/5.0 (Linux; Android 10) AppleWebKit/537.36")
    req.Header.Set("Accept", "*/*")
    if rng := r.Header.Get("Range"); rng != "" { req.Header.Set("Range", rng) }
    if ref := r.Header.Get("Referer"); ref != "" { req.Header.Set("Referer", ref) }

    resp, err := audioHTTPClient().Do(req)
    if err != nil { return err }
    defer resp.Body.Close()
    if resp.StatusCode < 200 || resp.StatusCode >= 300 {
        return fmt.Errorf("upstream audio status %s", resp.Status)
    }

    for _, h := range []string{"Content-Type", "Content-Length", "Content-Range", "Accept-Ranges", "ETag", "Last-Modified"} {
        if v := resp.Header.Get(h); v != "" { w.Header().Set(h, v) }
    }
    w.Header().Set("Cache-Control", "no-store")
    w.WriteHeader(resp.StatusCode)
    _, _ = io.Copy(w, resp.Body)
    return nil
}

func (a *App) audioSources(id string) ([]AudioSource, error) {
    if resolver, ok := a.upstream.(AudioCandidateResolver); ok {
        sources, err := resolver.AudioCandidates(id)
        if err == nil && len(sources) > 0 { return sources, nil }
    }
    resolver, ok := a.upstream.(AudioResolver)
    if !ok { return nil, fmt.Errorf("audio resolver unsupported") }
    directURL, playAuth, err := resolver.Audio(id)
    if err != nil { return nil, err }
    return []AudioSource{{URL: directURL, PlayAuth: playAuth, Label: "direct"}}, nil
}

func (a *App) debugSong(w http.ResponseWriter, r *http.Request) {
    id := strings.TrimPrefix(r.URL.Path, "/api/debug/song/")
    if id == "" { ok(w, map[string]string{"error":"missing id"}); return }
    tr, songErr := a.upstream.Song(id)
    sources, sourceErr := a.audioSources(id)
    ok(w, map[string]interface{}{
        "song": tr,
        "sources": sources,
        "song_error": errString(songErr),
        "source_error": errString(sourceErr),
    })
}

func errString(err error) string {
    if err == nil { return "" }
    return err.Error()
}

func cachedDecryptedAudio(r *http.Request, id, directURL, playAuth string) ([]byte, error) {
    dir := firstNonEmpty(os.Getenv("MOUYIN_CACHE_DIR"), filepath.Join(os.TempDir(), "mouyin-server-cache"))
    if err := os.MkdirAll(dir, 0755); err != nil { return nil, err }
    key := fmt.Sprintf("%x", sha256.Sum256([]byte(id+"|"+directURL+"|"+playAuth)))
    cacheFile := filepath.Join(dir, key+".m4a")
    if data, err := os.ReadFile(cacheFile); err == nil && len(data) > 0 {
        return data, nil
    }

    req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, directURL, nil)
    if err != nil { return nil, err }
    req.Header.Set("User-Agent", "Mozilla/5.0 (Linux; Android 10) AppleWebKit/537.36")
    req.Header.Set("Accept", "*/*")
    resp, err := audioHTTPClient().Do(req)
    if err != nil { return nil, err }
    defer resp.Body.Close()
    if resp.StatusCode < 200 || resp.StatusCode >= 300 {
        return nil, fmt.Errorf("download encrypted audio status %s", resp.Status)
    }
    encrypted, err := io.ReadAll(io.LimitReader(resp.Body, 80<<20))
    if err != nil { return nil, err }
    if len(encrypted) == 0 { return nil, fmt.Errorf("empty encrypted audio") }
    decrypted, err := sodalib.DecryptAudio(encrypted, playAuth)
    if err != nil || len(decrypted) == 0 {
        return nil, fmt.Errorf("decrypt failed: %w", err)
    }
    _ = os.WriteFile(cacheFile, decrypted, 0644)
    return decrypted, nil
}

func audioHTTPClient() *http.Client {
    transport := &http.Transport{Proxy: http.ProxyFromEnvironment}
    if p := firstNonEmpty(os.Getenv("UPSTREAM_PROXY"), os.Getenv("HTTPS_PROXY"), os.Getenv("HTTP_PROXY")); p != "" {
        if u, err := url.Parse(p); err == nil {
            transport.Proxy = http.ProxyURL(u)
        }
    }
    return &http.Client{Timeout: 60 * time.Second, Transport: transport}
}

func proxyURLForRequest(r *http.Request, id string) string {
    scheme := "http"
    if r.TLS != nil { scheme = "https" }
    if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" { scheme = proto }
    return scheme + "://" + r.Host + "/api/proxy/audio/" + id
}

func ensureExtra(m map[string]interface{}) map[string]interface{} {
    if m != nil { return m }
    return map[string]interface{}{}
}

func (a *App) discoverPlaylists(w http.ResponseWriter, r *http.Request) {
    data := []PlaylistSummary{
        {ID:"hot_cn", Name:"华语热歌", CoverURL:"https://picsum.photos/seed/mouyin-hot-cn/512/512", TrackCount:20},
        {ID:"daily", Name:"每日推荐", CoverURL:"https://picsum.photos/seed/mouyin-daily/512/512", TrackCount:20},
        {ID:"pop", Name:"流行精选", CoverURL:"https://picsum.photos/seed/mouyin-pop/512/512", TrackCount:20},
    }
    ok(w, data)
}

func (a *App) playlist(w http.ResponseWriter, r *http.Request) {
    id := strings.TrimPrefix(r.URL.Path, "/api/playlist/")
    keyword := map[string]string{"hot_cn":"华语热歌", "daily":"每日推荐", "pop":"流行"}[id]
    if keyword == "" { keyword = id }
    tracks, _, err := a.upstream.Search(keyword, 1, 20)
    if err != nil || len(tracks) == 0 { tracks = mockTracks(keyword) }
    ok(w, PlaylistDetail{ID:id, PlaylistID:id, Title:keyword, OwnerName:"Mouyin Local", Desc:"本地兼容歌单", TrackCount:len(tracks), Tracks:tracks, HasMore:false, NextCursor:0})
}

func (a *App) accountMe(w http.ResponseWriter, r *http.Request) { ok(w, adminUser()) }
func (a *App) accountLogin(w http.ResponseWriter, r *http.Request) { ok(w, map[string]interface{}{"token":"mock-admin-token", "user":adminUser()}) }
func adminUser() map[string]interface{} { return map[string]interface{}{"id":"admin", "username":"admin", "nickname":"Admin", "avatar":"", "vip":true} }

func atoiDefault(s string, def int) int { v, err := strconv.Atoi(s); if err != nil || v <= 0 { return def }; return v }
func firstNonEmpty(vals ...string) string { for _, v := range vals { if strings.TrimSpace(v) != "" { return strings.TrimSpace(v) } }; return "" }
