package server
// Day 3 -- multiplexing 
/**/
import (
	"fmt"
	"net"
	"time"
	"github.com/hashicorp/yamux"
)

// NewYamuxServerSession wraps an authenticated TCP connection into a multiplexed Yamux server session.
func NewYamuxServerSession(conn net.Conn) (*yamux.Session, error) {
	// 1. Production-ready Yamux configuration
	cfg := yamux.DefaultConfig()

	// 2. Built-in KeepAlive enable karo (Dead Peer Detection)
	cfg.EnableKeepAlive = true
	//Har 15 second mein Yamux background mein ping bhejta hai. Agar client ka laptop band ho jaye aur 30 second tak reply na aaye, toh session automatically clean ho jati hai.
	cfg.KeepAliveInterval = 15 * time.Second
	// Agar network slow ho ya packets drop ho rahe hon, toh socket forever hang na ho.
	cfg.ConnectionWriteTimeout = 10 * time.Second

	// 3. Physical TCP socket ko Yamux Server session mein elevate karo


	/*Yeh raw TCP conn ko leke Yamux multiplexer engine start kar deta hai.
	 Ab is session ke upar hum hazaro virtual streams open ya accept kar sakte hain.*/
	session, err := yamux.Server(conn, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create yamux server session: %w", err)
	}

	return session, nil
}