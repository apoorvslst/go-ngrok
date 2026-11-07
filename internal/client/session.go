package client

import (
	"fmt"
	"net"
	"time"

	"github.com/hashicorp/yamux"
)

// NewYamuxClientSession wraps an established TCP connection into a multiplexed Yamux client session.
func NewYamuxClientSession(conn net.Conn) (*yamux.Session, error) {
	// 1. Client configuration matching server keepalive settings
	cfg := yamux.DefaultConfig()
	//internal me server me jaake padh lo yar same cheez likhi hai
	cfg.EnableKeepAlive = true
	/*Dono sides pe matching keepalive interval hone se connection automatically
	 alive rehta hai chahe koi visitor request na bhej raha ho.*/
	cfg.KeepAliveInterval = 15 * time.Second
	cfg.ConnectionWriteTimeout = 10 * time.Second

	// 2. Physical TCP connection ko Yamux Client session mein elevate karo
	session, err := yamux.Client(conn, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create yamux client session: %w", err)
	}

	return session, nil
}