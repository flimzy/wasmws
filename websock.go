package wasmws

import "errors"

const (
	debugVerbose = false // Set to true if you are debugging issues, this gates many prints that would kill performance
)

var (
	// ErrWebsocketClosed is returned when operations are performed on a closed Websocket
	ErrWebsocketClosed = errors.New("WebSocket: Web socket is closed")
)

// timeoutErr is a net.Addr implementation for the websocket to use when fufilling
// the net.Conn interface
type timeoutError struct{}

func (timeoutError) Error() string { return "deadline exceeded" }

func (timeoutError) Timeout() bool { return true }

func (timeoutError) Temporary() bool { return true }

// wsAddr is a net.Addr implementation for the websocket to use when fufilling
// the net.Conn interface
type wsAddr string

func (wsAddr) Network() string { return "websocket" }

func (url wsAddr) String() string { return string(url) }
