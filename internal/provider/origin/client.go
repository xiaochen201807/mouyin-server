package origin

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"mouyin-server/internal/server"
)

const (
	defaultBaseURL = "http://39.104.86.142:5050/"
	defaultSecret  = "mY7c3F9kQ2xR8pL4vN6tZ1aB5eH0jW3sD"
)

type Config struct {
	BaseURL        string
	Secret         string
	Version        string
	AppVersionName string
	Proxy          string
	Timeout        time.Duration
}

type Client struct {
	http           *http.Client
	baseURL        string
	secret         string
	version        string
	appVersionName string
}

func NewClient(cfg Config) *Client {
	if cfg.Timeout == 0 {
		cfg.Timeout = 12 * time.Second
	}
	base := firstNonEmpty(cfg.BaseURL, defaultBaseURL)
	if !strings.HasSuffix(base, "/") {
		base += "/"
	}
	transport := &http.Transport{Proxy: http.ProxyFromEnvironment}
	if cfg.Proxy != "" {
		if proxyURL, err := url.Parse(cfg.Proxy); err == nil {
			transport.Proxy = http.ProxyURL(proxyURL)
		}
	}
	return &Client{
		http:           &http.Client{Timeout: cfg.Timeout, Transport: transport},
		baseURL:        base,
		secret:         firstNonEmpty(cfg.Secret, defaultSecret),
		version:        firstNonEmpty(cfg.Version, "151"),
		appVersionName: firstNonEmpty(cfg.AppVersionName, "1.6.1"),
	}
}

func (c *Client) Search(keyword string, page, pageSize int) ([]server.Track, bool, error) {
	return nil, false, errors.New("origin search intentionally disabled; local search should be used")
}

func (c *Client) Song(id string) (*server.Track, error) {
	if strings.TrimSpace(id) == "" {
		return nil, errors.New("empty id")
	}
	var root map[string]interface{}
	if err := c.getJSON(context.Background(), "api/song/"+url.PathEscape(id), url.Values{}, &root); err != nil {
		return nil, err
	}
	data, _ := root["data"].(map[string]interface{})
	if data == nil {
		return nil, errors.New("origin song response missing data")
	}
	tr := trackFromMap(data)
	if tr.ID == "" {
		tr.ID = id
	}
	tr.Source = firstNonEmpty(tr.Source, "mouyin-origin")
	tr.Type = firstNonEmpty(tr.Type, "audio")
	tr.MediaKind = firstNonEmpty(tr.MediaKind, "audio")
	if tr.Stats == nil {
		tr.Stats = map[string]int{"collect_count": 0, "comment_count": 0, "share_count": 0}
	}
	if len(tr.Tags) == 0 {
		tr.Tags = []string{"mouyin-origin"}
	}
	tr.Extra = ensureExtraMap(tr.Extra)
	tr.Extra["origin_base_url"] = c.baseURL
	return &tr, nil
}

func (c *Client) Audio(id string) (string, string, error) {
	sources, err := c.AudioCandidates(id)
	if err != nil {
		return "", "", err
	}
	if len(sources) == 0 {
		return "", "", errors.New("origin audio url unavailable")
	}
	return sources[0].URL, sources[0].PlayAuth, nil
}

func (c *Client) AudioCandidates(id string) ([]server.AudioSource, error) {
	tr, err := c.Song(id)
	if err != nil {
		return nil, err
	}
	var sources []server.AudioSource
	if tr.PlayURL != "" {
		sources = appendAudioSource(sources, server.AudioSource{URL: tr.PlayURL, Label: "origin_play_url"})
	}
	if tr.AudioURL != "" {
		sources = appendAudioSource(sources, server.AudioSource{URL: tr.AudioURL, Label: "origin_audio_url"})
	}
	if raw, ok := tr.Extra["playback_info"].([]interface{}); ok {
		for i, it := range raw {
			m, _ := it.(map[string]interface{})
			if u := str(m["url"]); u != "" {
				sources = appendAudioSource(sources, server.AudioSource{URL: u, Label: fmt.Sprintf("origin_playback_%d", i)})
			}
		}
	}
	if len(sources) == 0 {
		return nil, errors.New("origin song detail has no playable url")
	}
	return sources, nil
}

func (c *Client) getJSON(ctx context.Context, path string, q url.Values, out interface{}) error {
	u, err := url.Parse(c.baseURL)
	if err != nil {
		return err
	}
	rel, err := url.Parse(path)
	if err != nil {
		return err
	}
	u = u.ResolveReference(rel)
	u.RawQuery = q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return err
	}
	c.sign(req)
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 20<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("origin upstream %s: %s", resp.Status, string(b[:min(len(b), 500)]))
	}
	return json.Unmarshal(b, out)
}

func (c *Client) sign(req *http.Request) {
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	mac := hmac.New(sha256.New, []byte(c.secret))
	_, _ = mac.Write([]byte(ts))
	sig := hex.EncodeToString(mac.Sum(nil))
	req.Header.Set("X-Mouyin-Client", "android")
	req.Header.Set("X-Mouyin-Version", c.version)
	req.Header.Set("X-Mouyin-App-Version", c.appVersionName)
	req.Header.Set("User-Agent", "MouyinApp/"+c.appVersionName+" (Android; com.mouyin.app)")
	req.Header.Set("X-MY-T", ts)
	req.Header.Set("X-MY-S", sig)
	req.Header.Set("Accept", "application/json,text/plain,*/*")
}

func trackFromMap(m map[string]interface{}) server.Track {
	artists := stringsSlice(m["artists"])
	artist := str(m["artist"])
	if artist == "" && len(artists) > 0 {
		artist = strings.Join(artists, ", ")
	}
	stats := map[string]int{}
	if sm, ok := m["stats"].(map[string]interface{}); ok {
		for _, k := range []string{"collect_count", "comment_count", "share_count"} {
			stats[k] = int(num(sm[k]))
		}
	}
	if len(stats) == 0 {
		stats = nil
	}
	extra := map[string]interface{}{}
	for _, k := range []string{"direct_url", "play_auth", "audio_sources", "playback_info"} {
		if v, ok := m[k]; ok {
			extra[k] = v
		}
	}
	if rawExtra, ok := m["extra"].(map[string]interface{}); ok {
		for k, v := range rawExtra {
			extra[k] = v
		}
	}
	return server.Track{
		ID:          str(m["id"]),
		Title:       str(m["title"]),
		Type:        str(m["type"]),
		MediaKind:   str(m["media_kind"]),
		Artist:      artist,
		Artists:     artists,
		Album:       str(m["album"]),
		Duration:    int64(num(m["duration"])),
		Pic:         str(m["pic"]),
		PicBG:       str(m["pic_bg"]),
		PlayURL:     firstNonEmpty(str(m["play_url"]), str(m["url"])),
		AudioURL:    str(m["audio_url"]),
		LyricsLRC:   str(m["lyrics_lrc"]),
		LyricsType:  str(m["lyrics_type"]),
		Source:      str(m["source"]),
		Stats:       stats,
		Tags:        stringsSlice(m["tags"]),
		Description: str(m["description"]),
		Extra:       extra,
	}
}

func appendAudioSource(sources []server.AudioSource, src server.AudioSource) []server.AudioSource {
	if src.URL == "" {
		return sources
	}
	for _, existing := range sources {
		if existing.URL == src.URL {
			return sources
		}
	}
	return append(sources, src)
}

func stringsSlice(v interface{}) []string {
	arr, ok := v.([]interface{})
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, it := range arr {
		if s := str(it); s != "" {
			out = append(out, s)
			continue
		}
		if m, ok := it.(map[string]interface{}); ok {
			if s := str(m["name"]); s != "" {
				out = append(out, s)
			}
		}
	}
	return out
}

func str(v interface{}) string {
	switch x := v.(type) {
	case string:
		return strings.TrimSpace(x)
	case float64:
		return strconv.FormatInt(int64(x), 10)
	case int:
		return strconv.Itoa(x)
	case int64:
		return strconv.FormatInt(x, 10)
	default:
		return ""
	}
}

func num(v interface{}) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case int:
		return float64(x)
	case int64:
		return float64(x)
	case string:
		f, _ := strconv.ParseFloat(x, 64)
		return f
	default:
		return 0
	}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func ensureExtraMap(m map[string]interface{}) map[string]interface{} {
	if m != nil {
		return m
	}
	return map[string]interface{}{}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
