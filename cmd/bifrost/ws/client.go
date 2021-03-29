package ws

import (
	"net/http"
	"net/http/httputil"
	"sync"
	"time"

	"github.com/Wenchy/bifrost/internal/atom"
	"github.com/Wenchy/bifrost/internal/packet"
	"github.com/gorilla/websocket"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size(64k) allowed from peer.
	maxMessageSize = 64 * 1024
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

type Responser struct {
	done chan bool // indicate if http response is recieved
	req  *http.Request
	rw   http.ResponseWriter
}

// Messager is messager for client and msg pair.
type Messager struct {
	client *Client
	msg    []byte
}

// Client is a middleman between the websocket connection and the hub.
type Client struct {
	sync.RWMutex
	// client ID
	ID uint64
	// The websocket connection.
	conn *websocket.Conn
	// Buffered channel of outbound messages.
	sendCh       chan []byte
	sendChClosed bool
	// packet seq -> Responser
	responsers map[uint32]*Responser

	// server addr
	addr string
}

func BuildNewTunnel(addr string) {
	c := NewClient(addr)
	if err := c.Dial(); err == nil {
		c.Run()
	}
	go c.autoReconnect()
}

func NewClient(addr string) *Client {
	return &Client{
		ID:           0,
		conn:         nil,
		sendCh:       nil,
		sendChClosed: true,
		responsers:   map[uint32]*Responser{},
		addr:         addr,
	}
}

func (c *Client) Dial() error {
	conn, rsp, err := websocket.DefaultDialer.Dial(c.addr, nil)
	if err != nil {
		atom.Log.Errorf("websocket dial failed: %v", err)
		return err
	}
	rawrsp, err := httputil.DumpResponse(rsp, true)
	if err != nil {
		atom.Log.Warnf("http DumpResponse failed: %v", err)
	}
	atom.Log.Debugf("websocket dial rsp: %v", string(rawrsp))

	// below must be updated
	c.conn = conn
	c.sendCh = make(chan []byte, 256)
	c.sendChClosed = false

	return nil
}

func (c *Client) Run() {
	// Allow collection of memory referenced by the caller by doing all work in
	// new goroutines.
	go c.writePump()
	go c.readPump()

	Hub.register(c)
}

func (c *Client) autoReconnect() {
	statTicker := time.NewTicker(1 * time.Second)
	defer func() {
		statTicker.Stop()
	}()

	for {
		select {
		case <-statTicker.C:
			connected := true
			if c.conn == nil || c.sendChClosed {
				connected = false
			}

			if !connected {
				if err := c.Dial(); err == nil {
					c.Run()
				}
			}
		}
	}
}

func (c *Client) SendPacket(pkt *packet.Packet, rsper *Responser) error {
	buf, err := packet.Encode(pkt)
	if err != nil {
		return err
	}
	c.Lock()
	c.responsers[pkt.Header.Seq] = rsper
	c.Unlock()

	c.send(buf)
	return nil
}

// readPump pumps messages from the websocket connection to the hub.
//
// The application runs readPump in a per-connection goroutine. The application
// ensures that there is at most one reader on a connection by executing all
// reads from this goroutine.
func (c *Client) readPump() {
	defer func() {
		c.conn.Close()
		Hub.unregister(c)
	}()

	// Problem: websocket: close 1009 (message too big)
	// Resolution: The application is calling SetReadLimit. Increase the limit or remove the call.
	// refer: https://github.com/gorilla/websocket/issues/283
	// c.conn.SetReadLimit(maxMessageSize)

	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error { c.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				atom.Log.Warnf("%v|unexpected close error: %v", c.ID, err)
			}
			atom.Log.Warnf("%v|read message error: %v", c.ID, err)
			break
		}
		messager := Messager{client: c, msg: message}
		Hub.ingress <- messager
		atom.Log.Debugf("read a message")
	}
}

// writePump pumps messages from the hub to the websocket connection.
//
// A goroutine running writePump is started for each connection. The
// application ensures that there is at most one writer to a connection by
// executing all writes from this goroutine.
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
		Hub.unregister(c)
	}()
	for {
		select {
		case message, ok := <-c.sendCh:
			atom.Log.Debugf("write a message")
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The hub closed the channel.
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				atom.Log.Warnf("%v|the hub closed the channel", c.ID)
				return
			}

			w, err := c.conn.NextWriter(websocket.BinaryMessage)
			if err != nil {
				atom.Log.Warnf("%v|conn's next writer error: %v", c.ID, err)
				return
			}
			w.Write(message)

			// Add queued messages to the current websocket message.
			// TODO(wenchyzhu): client see a frame as a packet, later need to be optimized.
			// n := len(c.sendCh)
			// for i := 0; i < n; i++ {
			// 	w.Write(<-c.sendCh)
			// }

			if err := w.Close(); err != nil {
				atom.Log.Warnf("%d|close writer error: %v", c.ID, err)
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				atom.Log.Warnf("%v|write ping message error: %v", c.ID, err)
				return
			}
		}
	}
}

func (c *Client) send(msg []byte) {
	c.Lock()
	defer c.Unlock()

	if c.sendChClosed {
		atom.Log.Warnf("%v|sendCh channel already closed: %v", c.ID)
		return
	}
	c.sendCh <- msg
}

func (c *Client) close() {
	c.Lock()
	defer c.Unlock()
	if c.sendChClosed {
		atom.Log.Warnf("%v|sendCh channel already closed: %v", c.ID)
		return
	}
	c.sendChClosed = true
	close(c.sendCh)
}
