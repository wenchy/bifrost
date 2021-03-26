package ws

import (
	"net/http"

	"github.com/Wenchy/bifrost/internal/atom"
)

// serveWS handles websocket requests from the peer.
func ServeWS(w http.ResponseWriter, r *http.Request) {
	upgrader.CheckOrigin = func(r *http.Request) bool {
		return true
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		atom.Log.Warnf("websocket upgrade failed: %s", err)
		return
	}
	// When a new client connect in, ID is 0. After successfully login, response packet will give the client's ID.
	client := &Client{
		ID:           0, // TODO: generate unique ID
		conn:         conn,
		sendCh:       make(chan []byte, 256),
		sendChClosed: false,
		responsers:   map[uint32]*Responser{},
	}
	atom.Log.Debugf("new client: %p", client)

	client.Run()
}
