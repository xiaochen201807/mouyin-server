package server

import (
    "encoding/json"
    "log"
    "net/http"
    "strconv"
    "strings"
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
    ok(w, tr)
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
