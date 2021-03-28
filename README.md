# bifrost
A powerful bi-direction http proxy over websocket.

## Features
- [x] Support HTTP 1.x
- [x] Duplex communication
- [ ] Automatic reconnection
- [ ] Encryption and compress of content
- [ ] WebSocket Secure: wss
- [ ] Chunked transfer encoding(specially for large file transfers)
- [ ] Support HTTP2
- [ ] Support websocket, which means websocket over websocket
- [ ] Mutiple websocket connection tunnels, improve transmission performance

## Installation
`go get -u github.com/Wenchy/bifrost`

## Usage

### conf.yaml
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

### Run as daemon
- start: `./startstop start`
- stop: `./startstop stop`
- restart: `./startstop restart`