package hybrid

import (
	"log"

	"mouyin-server/internal/server"
)

type Client struct {
	SearchUpstream   server.Upstream
	PlaybackUpstream server.Upstream
}

func New(searchUpstream, playbackUpstream server.Upstream) *Client {
	return &Client{SearchUpstream: searchUpstream, PlaybackUpstream: playbackUpstream}
}

func (c *Client) Search(keyword string, page, pageSize int) ([]server.Track, bool, error) {
	return c.SearchUpstream.Search(keyword, page, pageSize)
}

func (c *Client) Song(id string) (*server.Track, error) {
	tr, err := c.PlaybackUpstream.Song(id)
	if err == nil && tr != nil && (tr.PlayURL != "" || tr.AudioURL != "") {
		return tr, nil
	}
	if err != nil {
		log.Printf("origin playback song failed for %s: %v; fallback to search upstream", id, err)
	}
	return c.SearchUpstream.Song(id)
}

func (c *Client) Audio(id string) (string, string, error) {
	sources, err := c.AudioCandidates(id)
	if err != nil {
		return "", "", err
	}
	if len(sources) == 0 {
		return "", "", server.ErrAudioUnavailable
	}
	return sources[0].URL, sources[0].PlayAuth, nil
}

func (c *Client) AudioCandidates(id string) ([]server.AudioSource, error) {
	if resolver, ok := c.PlaybackUpstream.(server.AudioCandidateResolver); ok {
		sources, err := resolver.AudioCandidates(id)
		if err == nil && len(sources) > 0 {
			return sources, nil
		}
		if err != nil {
			log.Printf("origin playback audio candidates failed for %s: %v; fallback to search upstream", id, err)
		}
	}
	if resolver, ok := c.PlaybackUpstream.(server.AudioResolver); ok {
		directURL, playAuth, err := resolver.Audio(id)
		if err == nil && directURL != "" {
			return []server.AudioSource{{URL: directURL, PlayAuth: playAuth, Label: "origin_audio"}}, nil
		}
		if err != nil {
			log.Printf("origin playback audio failed for %s: %v; fallback to search upstream", id, err)
		}
	}
	if resolver, ok := c.SearchUpstream.(server.AudioCandidateResolver); ok {
		return resolver.AudioCandidates(id)
	}
	if resolver, ok := c.SearchUpstream.(server.AudioResolver); ok {
		directURL, playAuth, err := resolver.Audio(id)
		if err != nil {
			return nil, err
		}
		return []server.AudioSource{{URL: directURL, PlayAuth: playAuth, Label: "fallback_audio"}}, nil
	}
	return nil, server.ErrAudioUnavailable
}
