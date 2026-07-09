package main

import (
    "log"
    "net/http"
    "os"
    "time"

    "mouyin-server/internal/provider/qishui"
    "mouyin-server/internal/server"
)

func main() {
    addr := getenv("ADDR", ":8000")
    upstream := qishui.NewClient(qishui.Config{
        Timeout: 12 * time.Second,
        XHelios: os.Getenv("QISHUI_X_HELIOS"),
        XMedusa: os.Getenv("QISHUI_X_MEDUSA"),
        Cookie:  os.Getenv("QISHUI_COOKIE"),
        Proxy:   os.Getenv("HTTP_PROXY"),
    })
    app := server.New(upstream)
    log.Printf("mouyin-server listening on %s", addr)
    log.Fatal(http.ListenAndServe(addr, app.Routes()))
}

func getenv(k, fallback string) string {
    if v := os.Getenv(k); v != "" { return v }
    return fallback
}
