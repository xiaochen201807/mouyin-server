package qishui

import (
    "context"
    "encoding/json"
    "errors"
    "fmt"
    "io"
    "net/http"
    "net/url"
    "strconv"
    "strings"
    "sync"
    "time"

    "mouyin-server/internal/server"
)

type Config struct {
    Timeout time.Duration
    XHelios string
    XMedusa string
    Cookie  string
    Proxy   string
}

type Client struct {
    http  *http.Client
    cfg   Config
    mu    sync.RWMutex
    cache map[string]server.Track
}

func NewClient(cfg Config) *Client {
    if cfg.Timeout == 0 { cfg.Timeout = 12 * time.Second }
    return &Client{http: &http.Client{Timeout: cfg.Timeout}, cfg: cfg, cache: map[string]server.Track{}}
}

func (c *Client) Search(keyword string, page, pageSize int) ([]server.Track, bool, error) {
    if page < 1 { page = 1 }
    if pageSize < 1 || pageSize > 20 { pageSize = 20 }
    cursor := (page - 1) * pageSize
    q := url.Values{}
    q.Set("aid", "386088")
    q.Set("app_name", "luna_pc")
    q.Set("region", "cn")
    q.Set("geo_region", "cn")
    q.Set("os_region", "cn")
    q.Set("device_id", "3753066532709850")
    q.Set("iid", "3753066532713946")
    q.Set("version_name", "3.5.1")
    q.Set("version_code", "30050100")
    q.Set("channel", "official")
    q.Set("build_mode", "master")
    q.Set("ac", "wifi")
    q.Set("tz_name", "Asia/Shanghai")
    q.Set("device_platform", "windows")
    q.Set("device_type", "Windows")
    q.Set("os_version", "Windows 11")
    q.Set("fp", "3753066532709850")
    q.Set("q", keyword)
    q.Set("cursor", strconv.Itoa(cursor))
    q.Set("search_id", fmt.Sprintf("mouyin-%d", time.Now().UnixNano()))
    q.Set("search_method", "input")
    endpoint := "https://api.qishui.com/luna/pc/search/track?" + q.Encode()
    var root map[string]interface{}
    if err := c.getJSON(context.Background(), endpoint, &root); err != nil { return nil, false, err }
    data := asSlice(firstPath(root, "result_groups", 0, "data"))
    tracks := make([]server.Track, 0, len(data))
    for _, item := range data {
        m, _ := item.(map[string]interface{})
        tr := c.trackFromSearch(m)
        if tr.ID != "" {
            tracks = append(tracks, tr)
            c.putCache(tr)
        }
    }
    return tracks, len(tracks) >= pageSize, nil
}

func (c *Client) Song(id string) (*server.Track, error) {
    if strings.TrimSpace(id) == "" { return nil, errors.New("empty id") }
    endpoint := "https://api.qishui.com/luna/h5/track?track_id=" + url.QueryEscape(id)
    var root map[string]interface{}
    if err := c.getJSON(context.Background(), endpoint, &root); err != nil { return nil, err }
    trackObj, _ := firstPath(root, "track").(map[string]interface{})
    tr := c.trackFromTrackObj(trackObj)
    if tr.ID == "" { tr.ID = id }
    tr.LyricsLRC = lyricToLRC(str(firstPath(root, "lyric", "content")))
    tr.PlayURL = c.extractPlayURL(root)
    tr.AudioURL = tr.PlayURL
    tr.Source = "qishui"
    tr.Type = "audio"
    tr.MediaKind = "audio"
    tr.Stats = map[string]int{"collect_count":0,"comment_count":0,"share_count":0}
    tr.Tags = []string{"qishui"}
    if cached, ok := c.getCache(id); ok {
        tr = mergeTrack(cached, tr)
    }
    c.putCache(tr)
    return &tr, nil
}

func (c *Client) putCache(t server.Track) {
    if t.ID == "" { return }
    c.mu.Lock()
    c.cache[t.ID] = t
    c.mu.Unlock()
}

func (c *Client) getCache(id string) (server.Track, bool) {
    c.mu.RLock()
    t, ok := c.cache[id]
    c.mu.RUnlock()
    return t, ok
}

func mergeTrack(base, detail server.Track) server.Track {
    if detail.ID == "" { detail.ID = base.ID }
    if detail.Title == "" { detail.Title = base.Title }
    if detail.Type == "" { detail.Type = base.Type }
    if detail.MediaKind == "" { detail.MediaKind = base.MediaKind }
    if detail.Artist == "" { detail.Artist = base.Artist }
    if len(detail.Artists) == 0 { detail.Artists = base.Artists }
    if detail.Album == "" { detail.Album = base.Album }
    if detail.Duration == 0 { detail.Duration = base.Duration }
    if detail.Pic == "" { detail.Pic = base.Pic }
    if detail.PicBG == "" { detail.PicBG = base.PicBG }
    if detail.PlayURL == "" { detail.PlayURL = base.PlayURL }
    if detail.AudioURL == "" { detail.AudioURL = base.AudioURL }
    if detail.LyricsLRC == "" { detail.LyricsLRC = base.LyricsLRC }
    if detail.LyricsType == "" { detail.LyricsType = base.LyricsType }
    if detail.Source == "" { detail.Source = base.Source }
    if detail.Stats == nil { detail.Stats = base.Stats }
    if len(detail.Tags) == 0 { detail.Tags = base.Tags }
    if detail.Description == "" { detail.Description = base.Description }
    return detail
}

func (c *Client) trackFromSearch(m map[string]interface{}) server.Track {
    trackObj, _ := firstPath(m, "entity", "track").(map[string]interface{})
    return c.trackFromTrackObj(trackObj)
}

func (c *Client) trackFromTrackObj(track map[string]interface{}) server.Track {
    artists := artists(track["artists"])
    cover := coverURL(firstPath(track, "album", "url_cover"))
    if cover == "" { cover = coverURL(firstPath(track, "url_cover")) }
    return server.Track{
        ID: str(track["id"]), Title: str(track["name"]), Type:"audio", MediaKind:"audio",
        Artist: strings.Join(artists, ", "), Artists: artists, Album: str(firstPath(track, "album", "name")),
        Duration: int64(num(track["duration"])), Pic: cover, PicBG: cover, LyricsType:"lrc", Source:"qishui",
        Stats: map[string]int{"collect_count":0,"comment_count":0,"share_count":0}, Tags: []string{"qishui"},
    }
}

func (c *Client) extractPlayURL(root map[string]interface{}) string {
    for _, path := range [][]interface{}{{"track_player", "video_model"}, {"track_player", "url_player_info"}} {
        s := str(firstPath(root, path...))
        if s == "" { continue }
        if strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") {
            var remote map[string]interface{}
            if err := c.getJSON(context.Background(), s, &remote); err == nil {
                if u := str(firstPath(remote, "Result", "Data", "PlayInfoList", 0, "MainPlayUrl")); u != "" { return u }
                if u := str(firstPath(remote, "Result", "Data", "PlayInfoList", 0, "BackupPlayUrl")); u != "" { return u }
            }
            continue
        }
        var parsed map[string]interface{}
        if json.Unmarshal([]byte(s), &parsed) == nil {
            if u := str(firstPath(parsed, "video_list", 0, "main_url")); u != "" { return u }
            if u := str(firstPath(parsed, "video_list", 0, "backup_url")); u != "" { return u }
            if u := str(firstPath(parsed, "Result", "Data", "PlayInfoList", 0, "MainPlayUrl")); u != "" { return u }
            if u := str(firstPath(parsed, "Result", "Data", "PlayInfoList", 0, "BackupPlayUrl")); u != "" { return u }
        }
    }
    return ""
}

func (c *Client) getJSON(ctx context.Context, endpoint string, out interface{}) error {
    req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
    if err != nil { return err }
    req.Header.Set("User-Agent", "LunaPC/3.5.1(408871041)")
    req.Header.Set("Content-Type", "application/json; charset=utf-8")
    req.Header.Set("Accept", "application/json,text/plain,*/*")
    if c.cfg.XHelios != "" { req.Header.Set("X-Helios", c.cfg.XHelios) }
    if c.cfg.XMedusa != "" { req.Header.Set("X-Medusa", c.cfg.XMedusa) }
    if c.cfg.Cookie != "" { req.Header.Set("Cookie", c.cfg.Cookie) }
    resp, err := c.http.Do(req)
    if err != nil { return err }
    defer resp.Body.Close()
    b, _ := io.ReadAll(io.LimitReader(resp.Body, 20<<20))
    if resp.StatusCode < 200 || resp.StatusCode >= 300 { return fmt.Errorf("upstream %s: %s", resp.Status, string(b[:min(len(b), 300)])) }
    return json.Unmarshal(b, out)
}

func firstPath(v interface{}, path ...interface{}) interface{} { cur := v; for _, p := range path { switch key := p.(type) { case string: m, ok := cur.(map[string]interface{}); if !ok { return nil }; cur = m[key]; case int: a := asSlice(cur); if key < 0 || key >= len(a) { return nil }; cur = a[key] } }; return cur }
func asSlice(v interface{}) []interface{} { if a, ok := v.([]interface{}); ok { return a }; return nil }
func str(v interface{}) string { switch x := v.(type) { case string: return x; case float64: return strconv.FormatInt(int64(x),10); case int: return strconv.Itoa(x); case int64: return strconv.FormatInt(x,10); default: return "" } }
func num(v interface{}) float64 { switch x := v.(type) { case float64: return x; case int: return float64(x); case int64: return float64(x); case string: f,_ := strconv.ParseFloat(x,64); return f; default: return 0 } }
func min(a,b int) int { if a < b { return a }; return b }
func artists(v interface{}) []string { out:=[]string{}; for _, it := range asSlice(v) { if name := str(firstPath(it, "name")); name != "" { out = append(out, name) } }; return out }
func coverURL(v interface{}) string { base := str(firstPath(v, "urls", 0)); uri := str(firstPath(v, "uri")); if base == "" { return "" }; if uri == "" { return base }; return base + uri + "~c5_500x500.jpg" }
func lyricToLRC(s string) string { if s == "" { return "" }; if strings.Contains(s, "[") && strings.Contains(s, "]") { return s }; return "[00:00.00]" + strings.ReplaceAll(s, "\n", "\n[00:00.00]") }
