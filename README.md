# bifrost
A powerful bi-direction http proxy over websocket.

## Features
- [x] Support HTTP 1.x
- [x] Duplex communication
- [ ] Automatic reconnection
- [ ] Encryption and compress of content
- [ ] WebSocket Secure: wss, refer https://github.com/denji/golang-tls
- [ ] Chunked transfer encoding(specially for large file transfers)
- [ ] Support HTTP2
- [ ] Support websocket, which means websocket over websocket
- [ ] Mutiple websocket connection tunnels, improve transmission performance

## Installation
`go get -u github.com/Wenchy/bifrost`

## Usage

### Configuruation

**conf.yaml**
```
server:
  self_addr: :9098
  peer_addr: ws://localhost:9099/ws
proxies:
  - path: /*
    target: http://localhost
log:
  level: debug # debug, info, warn, error
  dir: ./logs # log directory  
```

### Extended custom HTTP Headers
#### `X-Bifrost-Target`
This field directs the forwarded target to the websocket tunnel's peer side, it is like the `proxy_pass` director in Nginx. If this header field is set, the `proxies` item in **conf.yaml** will not be taken into consideration.


e.g.: `X-Bifrost-Target: https://www.google.com`

### Run as daemon
- start: `./startstop start`
- stop: `./startstop stop`
- restart: `./startstop restart`


## References
- [Proxy servers and tunneling](https://developer.mozilla.org/en-US/docs/Web/HTTP/Proxy_servers_and_tunneling)
- [net/http/httputil/reverseproxy.go](https://golang.org/src/net/http/httputil/reverseproxy.go)
- [golang-tls](https://github.com/denji/golang-tls)
- [v2ray](https://github.com/v2fly/v2ray-core)
- [shadowsocks](https://github.com/shadowsocks/go-shadowsocks2)
- [go-proxyproto](https://github.com/pires/go-proxyproto)