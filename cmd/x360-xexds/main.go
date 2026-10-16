// Command x360-xexds is OpenPak's Xbox 360 XEXDS endpoint. It answers the
// dashboard's MTU and NAT probes on UDP 3074; nothing else yet.
package main

import (
	"context"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	"openpak/x360-xexds/internal/config"
	"openpak/x360-xexds/internal/xexds"
)

func main() {
	cfg, err := config.Load(os.Getenv)
	level := slog.LevelInfo
	if err == nil && cfg.Debug {
		level = slog.LevelDebug
	}
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
	if err != nil {
		log.Error("config", "err", err)
		os.Exit(2)
	}
	key := []byte(cfg.Key)
	if len(key) == 0 {
		key = xexds.NewKey()
	}
	addr, err := net.ResolveUDPAddr("udp4", cfg.Addr)
	if err != nil {
		log.Error("address", "err", err)
		os.Exit(2)
	}
	conn, err := net.ListenUDP("udp4", addr)
	if err != nil {
		log.Error("listen", "err", err)
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	log.Info("x360-xexds listening", "addr", conn.LocalAddr(), "debug", cfg.Debug)
	srv := &xexds.Server{Responder: &xexds.Responder{Key: key}, Log: log}
	if err := srv.Serve(ctx, conn); err != nil {
		log.Error("serve", "err", err)
		os.Exit(1)
	}
}
