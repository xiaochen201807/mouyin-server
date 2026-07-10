package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"mouyin-server/internal/provider/hybrid"
	"mouyin-server/internal/provider/origin"
	"mouyin-server/internal/provider/qishui"
	"mouyin-server/internal/server"
)

func main() {
	addr := getenv("ADDR", ":8000")
	searchUpstream := qishui.NewClient(qishui.Config{
		Timeout: 12 * time.Second,
		XHelios: os.Getenv("QISHUI_X_HELIOS"),
		XMedusa: os.Getenv("QISHUI_X_MEDUSA"),
		Cookie:  os.Getenv("QISHUI_COOKIE"),
		Proxy:   firstNonEmpty(os.Getenv("UPSTREAM_PROXY"), os.Getenv("HTTPS_PROXY"), os.Getenv("HTTP_PROXY")),
	})
	playbackUpstream := origin.NewClient(origin.Config{
		BaseURL:        getenv("MOUYIN_ORIGIN_BASE_URL", "http://39.104.86.142:5050/"),
		Secret:         os.Getenv("MOUYIN_ORIGIN_SECRET"),
		Version:        getenv("MOUYIN_ORIGIN_VERSION", "151"),
		AppVersionName: getenv("MOUYIN_ORIGIN_APP_VERSION", "1.6.1"),
		Proxy:          firstNonEmpty(os.Getenv("UPSTREAM_PROXY"), os.Getenv("HTTPS_PROXY"), os.Getenv("HTTP_PROXY")),
		Timeout:        12 * time.Second,
	})
	upstream := hybrid.New(searchUpstream, playbackUpstream)
	app := server.New(upstream)
	log.Printf("mouyin-server listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, app.Routes()))
}

func getenv(k, fallback string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return fallback
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
