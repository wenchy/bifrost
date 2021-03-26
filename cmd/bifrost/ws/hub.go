package ws

import (
	"bufio"
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"time"

	"github.com/Wenchy/bifrost/internal/atom"
	"github.com/Wenchy/bifrost/internal/packet"
)

var Hub *hub

func init() {
	Hub = NewDefaultHub()
}

// hub maintains the set of active clients and Broadcasts messages to the
// clients.
type hub struct {
	sync.RWMutex
	// Registered clients.
	Clients map[uint64]*Client

	// Inbound messages from the clients.
	ingress chan Messager
}

func NewDefaultHub() *hub {
	return &hub{
		Clients: make(map[uint64]*Client),
		ingress: make(chan Messager, 1024),
	}
}

func (h *hub) Run() {
	go h.dispatchIngress()
	statTicker := time.NewTicker(10 * time.Second)
	defer func() {
		statTicker.Stop()
	}()

	for {
		select {
		case <-statTicker.C:
			atom.Log.Infof("client count: %d", len(h.Clients))
		}
	}
}

func (h *hub) register(c *Client) {
	h.Lock()
	defer h.Unlock()

	atom.Log.Debugf("%v|register start, client: %p", c.ID, c)
	h.Clients[c.ID] = c
	atom.Log.Debugf("%v|register end, client: %p", c.ID, c)
}

func (h *hub) unregister(c *Client) {
	h.Lock()
	defer h.Unlock()

	atom.Log.Debugf("%v|unregister start, client: %p", c.ID, c)

	for id, _ := range h.Clients {
		if c.ID == id {
			delete(h.Clients, c.ID)
			c.close()
			return
		}
	}
	atom.Log.Warnf("%v|unregister end, client: %p, ID not found when unregister", c.ID, c)
}

func (h *hub) dispatchIngress() {
	for messager := range h.ingress {
		go h.handleIngress(messager.client, messager.msg)
	}
}

func (h *hub) handleIngress(c *Client, msg []byte) error {
	pkt, err := packet.Parse(msg)
	if err != nil {
		atom.Log.Warnf("decode err: %v", err)
		return err
	}
	atom.Log.Debugf("packet seq: %v", pkt.Header.Seq)

	switch pkt.Header.Type {
	case packet.PacketTypeRequest:
		// https://stackoverflow.com/questions/19595860/http-request-requesturi-field-when-making-request-in-go
		req, err := http.ReadRequest(bufio.NewReader(bytes.NewBuffer(pkt.Payload)))
		if err != nil {
			atom.Log.Errorf("ReadRequest failed: %v", err)
			return err
		}

		// Save a copy of this request for debugging.
		rawreq, err := httputil.DumpRequest(req, true)
		if err != nil {
			atom.Log.Errorf("DumpRequest failed: %s", err)
			return err
		}
		atom.Log.Debugf("%d|recieve request: %s", pkt.Header.Seq, string(rawreq))

		// We can't have this set. It is an error to set this field in
		// an HTTP client request. The reason why it is set is because
		// that is what ReadRequest does when parsing the request stream.
		req.RequestURI = ""

		// Since the req.URL will not have all the information set,
		// such as protocol scheme and host, we create a new URL
		directedURL, err := url.Parse("http://www.baidu.com")
		if err != nil {
			atom.Log.Errorf("Parse failed: %s", err)
			return err
		}
		req.URL = directedURL
		

		// for test
		req, err = http.NewRequest(http.MethodGet, "http://www.baidu.com", nil)
		if err != nil {
			atom.Log.Errorf("NewRequest failed: %s", err)
			return  err
		}
		client := &http.Client{}
		rsp, err := client.Do(req)
		if err != nil {
			atom.Log.Errorf("http client do failed: %v", err)
			return err
		}
		// Save a copy of this request for debugging.
		rawrsp, err := httputil.DumpResponse(rsp, true)
		if err != nil {
			atom.Log.Errorf("DumpRequest failed: %s", err)
			return err
		}

		h.RLock()
		c, ok := h.Clients[0] // TODO: pick a proper client
		if !ok {
			h.RUnlock()
			atom.Log.Warnf("%v|ID not found", c.ID)
			return fmt.Errorf("ID not found")
		}
		h.RUnlock()

		pkt.Header.Type = packet.PacketTypeResponse
		pkt.Header.Size = uint32(len(rawrsp))
		pkt.Payload = rawrsp
		c.SendPacket(pkt, nil)

	case packet.PacketTypeResponse:
		if rsper, ok := c.responsers[pkt.Header.Seq]; ok {
			atom.Log.Debugf("%d|recieve response: %s", pkt.Header.Seq, string(pkt.Payload))
			// refer: https://stackoverflow.com/questions/62387069/golang-parse-raw-http-2-response
			// TODO(wenchy): handle HTTP/2
			rsp, err := http.ReadResponse(bufio.NewReader(bytes.NewBuffer(pkt.Payload)), rsper.req)
			if err != nil {
				atom.Log.Errorf("ReadResponse failed: %v", err)
				return err
			}
			// Read body to buffer
			body, err := ioutil.ReadAll(rsp.Body)
			if err != nil {
				atom.Log.Errorf("Error reading body: %v", err)
				panic(err)
			}
			defer rsp.Body.Close()

			rsper.rw.Write(body)
			rsper.done <- true
		}
	case packet.PacketTypeNotice:
		atom.Log.Errorf("PacketTypeNotice not processed currently")
	default:
		atom.Log.Errorf("unknown packet type: %v", pkt.Header.Type)
	}
	return nil
}

func (h *hub) forward(ID uint64, msg []byte) error {
	h.RLock()
	defer h.RUnlock()
	c, ok := h.Clients[ID]
	if !ok {
		atom.Log.Warnf("%v|ID not found", ID)
		return fmt.Errorf("ID not found")
	}

	c.send(msg)
	return nil
}

func Forward(rw http.ResponseWriter, req *http.Request) error {
	Hub.RLock()
	c, ok := Hub.Clients[0] // TODO: pick a proper client
	if !ok {
		Hub.RUnlock()
		atom.Log.Warnf("%v|ID not found", 0)
		return fmt.Errorf("ID not found")
	}
	Hub.RUnlock()

	// Save a copy of this request for debugging.
	rawreq, err := httputil.DumpRequest(req, true)
	if err != nil {
		atom.Log.Errorf("DumpRequest failed: %s", err)
		return err
	}

	pkt := packet.NewRequestPacket(rawreq)
	rsper := &Responser{
		done: make(chan bool),
		req:  req,
		rw:   rw,
	}

	atom.Log.Debugf("%d|start request: %s", pkt.Header.Seq, string(rawreq))

	err = c.SendPacket(pkt, rsper)
	if err != nil {
		atom.Log.Errorf("SendPacket failed: %s", err)
		return err
	}

	for {
		select {
		case <-rsper.done:
			atom.Log.Debugf("%d|end request: %s", pkt.Header.Seq, string(rawreq))
			return nil
		}
	}
}
