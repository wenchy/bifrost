package ws

import (
	"bufio"
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
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

// Copy singleJoiningSlash from https://golang.org/src/net/http/httputil/reverseproxy.go
func singleJoiningSlash(a, b string) string {
	aslash := strings.HasSuffix(a, "/")
	bslash := strings.HasPrefix(b, "/")
	switch {
	case aslash && bslash:
		return a + b[1:]
	case !aslash && !bslash:
		return a + "/" + b
	}
	return a + b
}

// Copy joinURLPath from https://golang.org/src/net/http/httputil/reverseproxy.go
func joinURLPath(a, b *url.URL) (path, rawpath string) {
	if a.RawPath == "" && b.RawPath == "" {
		return singleJoiningSlash(a.Path, b.Path), ""
	}
	// Same as singleJoiningSlash, but uses EscapedPath to determine
	// whether a slash should be added
	apath := a.EscapedPath()
	bpath := b.EscapedPath()

	aslash := strings.HasSuffix(apath, "/")
	bslash := strings.HasPrefix(bpath, "/")

	switch {
	case aslash && bslash:
		return a.Path + b.Path[1:], apath + bpath[1:]
	case !aslash && !bslash:
		return a.Path + "/" + b.Path, apath + "/" + bpath
	}
	return a.Path + b.Path, apath + bpath
}

// DirectRequest routes URLs to the scheme, host, and base path
// provided in target. If the target's path is "/base" and the
// incoming request was for "/dir", the target request will be
// for /base/dir.
// Learn NewSingleHostReverseProxy from https://golang.org/src/net/http/httputil/reverseproxy.go
func DirectRequest(req *http.Request, target *url.URL) {
	targetQuery := target.RawQuery
	req.URL.Scheme = target.Scheme
	req.URL.Host = target.Host
	req.URL.Path, req.URL.RawPath = joinURLPath(target, req.URL)
	// NOTE(wenchy): replace `Host` header field
	req.Host = target.Host
	if targetQuery == "" || req.URL.RawQuery == "" {
		req.URL.RawQuery = targetQuery + req.URL.RawQuery
	} else {
		req.URL.RawQuery = targetQuery + "&" + req.URL.RawQuery
	}
	if _, ok := req.Header["User-Agent"]; !ok {
		// explicitly disable User-Agent so it's not set to default value
		req.Header.Set("User-Agent", "")
	}

	// We can't have this set. It is an error to set this field in
	// an HTTP client request. The reason why it is set is because
	// that is what ReadRequest does when parsing the request stream.
	req.RequestURI = ""
}

func copyHeader(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
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
		// decompress payload

		// decrypt
		zipReq, err := Decrypt(cipherKey, pkt.Payload)
		if err != nil {
			atom.Log.Errorf("decrypt failed: %s", err)
			return err
		}
		// decompress
		rawReq, err := DecompressByGzip(zipReq)
		if err != nil {
			atom.Log.Errorf("decompress failed: %s", err)
			return err
		}

		req, err := http.ReadRequest(bufio.NewReader(bytes.NewBuffer(rawReq)))
		if err != nil {
			atom.Log.Errorf("ReadRequest failed: %v", err)
			return err
		}

		target := req.Header.Get("X-Bifrost-Target")
		targetURL, _ := url.Parse(target)
		DirectRequest(req, targetURL)

		// Save a copy of this request for debugging.
		logRawReq, err := httputil.DumpRequest(req, true)
		if err != nil {
			atom.Log.Errorf("DumpRequest failed: %s", err)
			return err
		}

		atom.Log.Debugf("%d|recieve request: %s, %s, %s", pkt.Header.Seq, req.Method, req.URL.String(), string(logRawReq))

		client := &http.Client{
			Timeout: time.Second * 5,
			// NOTE(wenchy): shouldn't follow any redirects!!!
		    //
			// As a special case, if CheckRedirect returns ErrUseLastResponse,
			// then the most recent response is returned with its body
		    // unclosed, along with a nil error.
		    CheckRedirect: func(req *http.Request, via []*http.Request) error {
		        return http.ErrUseLastResponse
		    },
		}
		rsp, err := client.Do(req)
		if err != nil {
			atom.Log.Errorf("http client do failed: %v", err)
			rsp = &http.Response{
				StatusCode: http.StatusInternalServerError,
				Status:     err.Error(),
				ProtoMajor: 1,
				ProtoMinor: 1,
			}
		}

		// Save a copy of this request for debugging.
		rawRsp, err := httputil.DumpResponse(rsp, true)
		if err != nil {
			atom.Log.Errorf("DumpRequest failed: %s", err)
			return err
		}

		atom.Log.Debugf("%d|got response: %s, %s, %s", pkt.Header.Seq, req.Method, req.URL.String(), string(rawRsp))

		h.RLock()
		c, ok := h.Clients[0] // TODO: pick a proper client
		if !ok {
			h.RUnlock()
			atom.Log.Warnf("%v|ID not found", c.ID)
			return fmt.Errorf("ID not found")
		}
		h.RUnlock()

		// compress
		zipRsp, err := CompressByGzip(rawRsp)
		if err != nil {
			atom.Log.Errorf("compress failed: %s", err)
			return err
		}
		// encrypt
		cipherRsp, err := Encrypt(cipherKey, zipRsp)
		if err != nil {
			atom.Log.Errorf("encrypt failed: %s", err)
			return err
		}

		pkt.Header.Type = packet.PacketTypeResponse
		pkt.Header.Size = uint32(len(cipherRsp))
		pkt.Payload = cipherRsp
		c.SendPacket(pkt, nil)

		atom.Log.Debugf("%d|send response: %s", pkt.Header.Seq, rawRsp)

	case packet.PacketTypeResponse:
		if rsper, ok := c.responsers[pkt.Header.Seq]; ok {
			// decrypt
			zipRsp, err := Decrypt(cipherKey, pkt.Payload)
			if err != nil {
				atom.Log.Errorf("decrypt failed: %s", err)
				return err
			}
			// decompress
			rawRsp, err := DecompressByGzip(zipRsp)
			if err != nil {
				atom.Log.Errorf("decompress failed: %s", err)
				return err
			}

			atom.Log.Debugf("%d|recieve response: %s", pkt.Header.Seq, string(rawRsp))
			// refer: https://stackoverflow.com/questions/62387069/golang-parse-raw-http-2-response
			// TODO(wenchy): handle HTTP/2
			rsp, err := http.ReadResponse(bufio.NewReader(bytes.NewBuffer(rawRsp)), rsper.req)
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

			copyHeader(rsper.rw.Header(), rsp.Header)
			// NOTE(wenchyzhu): read docs of http.ResponseWriter and refer net/http/httputil/reverseproxy.go
			// Changing the header map after a call to WriteHeader (or
			// Write) has no effect unless the modified headers are
			// trailers.
			rsper.rw.WriteHeader(rsp.StatusCode)
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

func Forward(target string, rw http.ResponseWriter, req *http.Request) error {
	Hub.RLock()
	c, ok := Hub.Clients[0] // TODO: pick a proper client
	if !ok {
		Hub.RUnlock()
		atom.Log.Warnf("%v|ID not found", 0)
		return fmt.Errorf("ID not found")
	}
	Hub.RUnlock()
	// custom HTTP header field: X-Bifrost-Target
	req.Header.Set("X-Bifrost-Target", target)
	// Save a copy of this request for debugging.
	rawReq, err := httputil.DumpRequest(req, true)
	if err != nil {
		atom.Log.Errorf("DumpRequest failed: %s", err)
		return err
	}

	// compress
	zipReq, err := CompressByGzip(rawReq)
	if err != nil {
		atom.Log.Errorf("compress failed: %s", err)
		return err
	}
	// encrypt
	cipherReq, err := Encrypt(cipherKey, zipReq)
	if err != nil {
		atom.Log.Errorf("encrypt failed: %s", err)
		return err
	}

	pkt := packet.NewRequestPacket(cipherReq)
	rsper := &Responser{
		done: make(chan bool),
		req:  req,
		rw:   rw,
	}

	atom.Log.Debugf("%d|send request: %s, %s", pkt.Header.Seq, req.URL.String(), string(rawReq))

	err = c.SendPacket(pkt, rsper)
	if err != nil {
		atom.Log.Errorf("SendPacket failed: %s", err)
		return err
	}

	for {
		select {
		case <-rsper.done:
			atom.Log.Debugf("%d|end request: %s", pkt.Header.Seq, req.URL.String())
			// atom.Log.Debugf("%d|end request: %s", pkt.Header.Seq, string(rawReq))
			return nil
		}
	}
}
