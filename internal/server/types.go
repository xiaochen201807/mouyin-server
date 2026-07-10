package server

import "time"

type BaseResponse struct {
	Code      int         `json:"code"`
	Msg       string      `json:"msg"`
	Data      interface{} `json:"data"`
	Timestamp int64       `json:"timestamp"`
}

type ListResponse struct {
	Code      int         `json:"code"`
	Msg       string      `json:"msg"`
	Data      interface{} `json:"data"`
	HasMore   bool        `json:"has_more"`
	Timestamp int64       `json:"timestamp"`
}

type Track struct {
	ID          string                 `json:"id"`
	Title       string                 `json:"title"`
	Type        string                 `json:"type"`
	MediaKind   string                 `json:"media_kind"`
	Artist      string                 `json:"artist"`
	Artists     []string               `json:"artists"`
	Album       string                 `json:"album"`
	Duration    int64                  `json:"duration"`
	Pic         string                 `json:"pic"`
	PicBG       string                 `json:"pic_bg"`
	PlayURL     string                 `json:"play_url"`
	AudioURL    string                 `json:"audio_url"`
	LyricsLRC   string                 `json:"lyrics_lrc"`
	LyricsType  string                 `json:"lyrics_type"`
	Source      string                 `json:"source"`
	Stats       map[string]int         `json:"stats"`
	Tags        []string               `json:"tags"`
	Description string                 `json:"description"`
	Extra       map[string]interface{} `json:"extra,omitempty"`
}

type PlaylistSummary struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	CoverURL   string `json:"cover_url"`
	TrackCount int    `json:"track_count"`
}

type PlaylistDetail struct {
	ID         string  `json:"id"`
	PlaylistID string  `json:"playlist_id"`
	Title      string  `json:"title"`
	OwnerName  string  `json:"owner_name"`
	Desc       string  `json:"desc"`
	TrackCount int     `json:"track_count"`
	Tracks     []Track `json:"tracks"`
	HasMore    bool    `json:"has_more"`
	NextCursor int     `json:"next_cursor"`
}

type Upstream interface {
	Search(keyword string, page, pageSize int) ([]Track, bool, error)
	Song(id string) (*Track, error)
}

type AudioResolver interface {
	Audio(id string) (directURL string, playAuth string, err error)
}

type AudioSource struct {
	URL      string `json:"url"`
	PlayAuth string `json:"play_auth,omitempty"`
	Label    string `json:"label,omitempty"`
}

type AudioProbe struct {
	Index           int    `json:"index"`
	Label           string `json:"label,omitempty"`
	URL             string `json:"url"`
	HasPlayAuth     bool   `json:"has_play_auth"`
	Status          string `json:"status,omitempty"`
	StatusCode      int    `json:"status_code,omitempty"`
	ContentType     string `json:"content_type,omitempty"`
	ContentLength   string `json:"content_length,omitempty"`
	ContentRange    string `json:"content_range,omitempty"`
	AcceptRanges    string `json:"accept_ranges,omitempty"`
	BodySampleBytes int64  `json:"body_sample_bytes"`
	RangeOK         bool   `json:"range_ok"`
	DecryptOK       bool   `json:"decrypt_ok,omitempty"`
	DecryptedBytes  int    `json:"decrypted_bytes,omitempty"`
	DecryptError    string `json:"decrypt_error,omitempty"`
	Error           string `json:"error,omitempty"`
}

type AudioCandidateResolver interface {
	AudioCandidates(id string) ([]AudioSource, error)
}

func nowMS() int64 { return time.Now().UnixMilli() }
