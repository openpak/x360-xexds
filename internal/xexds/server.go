package xexds

import (
	"context"
	"errors"
	"log/slog"
	"net"
)

// Server reads datagrams from one UDP socket and answers them inline: the
// work per datagram is a few hundred bytes of HMAC, so nothing slow ever runs
// on the receive loop.
type Server struct {
	Responder *Responder
	Log       *slog.Logger
}

// Serve runs until ctx is cancelled or the socket fails.
func (s *Server) Serve(ctx context.Context, conn *net.UDPConn) error {
	go func() {
		<-ctx.Done()
		conn.Close()
	}()
	buf := make([]byte, 65535)
	for {
		n, src, err := conn.ReadFromUDPAddrPort(buf)
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
				return nil
			}
			return err
		}
		reply, kind, tagOK := s.Responder.Reply(buf[:n], src)
		if reply == nil {
			s.Log.Debug("ignored datagram", "src", src, "len", n)
			continue
		}
		if _, err := conn.WriteToUDPAddrPort(reply, src); err != nil {
			s.Log.Warn("reply failed", "src", src, "err", err)
			continue
		}
		attrs := []any{"kind", kind.String(), "src", src, "len", n, "reply_len", len(reply)}
		if kind == FollowUp {
			attrs = append(attrs, "cookie_ok", tagOK)
		}
		s.Log.Debug("answered", attrs...)
	}
}
