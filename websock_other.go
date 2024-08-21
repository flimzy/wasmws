//go:build !js

package wasmws

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
)

// WebSocket is a Go struct that wraps the web browser's JavaScript websocket object and provides a net.Conn interface
type WebSocket struct {
	ctx       context.Context
	ctxCancel context.CancelFunc

	URL string
	ws  *websocket.Conn

	readLock  sync.Mutex
	remaining io.Reader

	cleanup []func()
}

// New returns a new WebSocket using the provided dial context and websocket URL.
// The URL should be in the form of "ws://host/path..." for unsecured websockets
// and "wss://host/path..." for secured websockets. If tunnel a TLS based protocol
// over a "wss://..." websocket you will get TLS twice, once on the websocket using
// the browsers TLS stack and another using the Go (or other compiled) TLS stack.
func New(dialCtx context.Context, URL string, opts ...DialOption) (*WebSocket, error) {
	ctx, cancel := context.WithCancel(context.Background())
	dialOpts := &dialOptions{
		client: &http.Client{
			Timeout: time.Minute,
		},
	}
	for _, opt := range opts {
		opt.apply(dialOpts)
	}
	conn, _, err := websocket.Dial(dialCtx, URL, &websocket.DialOptions{
		HTTPClient: dialOpts.client,
	})
	if err != nil {
		cancel()
		return nil, err
	}
	ws := &WebSocket{
		ctx:       ctx,
		ctxCancel: cancel,

		URL: URL,
		ws:  conn,

		cleanup: make([]func(), 0, 3),
	}

	go func() { // handle shutdown
		<-ws.ctx.Done()
		if debugVerbose {
			println("Websocket: Shutdown")
		}

		ws.ws.Close(websocket.StatusNormalClosure, "Go context closed")
		for _, cleanup := range ws.cleanup {
			cleanup()
		}
	}()

	return ws, nil
}

// Close shuts the websocket down
func (ws *WebSocket) Close() error {
	if debugVerbose {
		println("Websocket: Internal close")
	}
	ws.ctxCancel()
	return nil
}

// LocalAddr returns a dummy websocket address to satisfy net.Conn, see: wsAddr
func (ws *WebSocket) LocalAddr() net.Addr {
	return wsAddr(ws.URL)
}

// RemoteAddr returns a dummy websocket address to satisfy net.Conn, see: wsAddr
func (ws *WebSocket) RemoteAddr() net.Addr {
	return wsAddr(ws.URL)
}

// Write implements the standard [io.Writer] interface.
func (ws *WebSocket) Write(buf []byte) (int, error) {
	err := ws.ws.Write(ws.ctx, websocket.MessageBinary, buf)
	if err != nil {
		return 0, err
	}
	return len(buf), nil
}

// Read implements the standard [io.Reader] interface.
func (ws *WebSocket) Read(buf []byte) (int, error) {
	// Check for noop
	if len(buf) < 1 {
		return 0, nil
	}

	// Lock
	ws.readLock.Lock()
	defer ws.readLock.Unlock()

	// Check for close
	select {
	case <-ws.ctx.Done():
		return 0, ErrWebsocketClosed
	default:
	}

	for {
		// Get next chunk
		if ws.remaining == nil {
			if debugVerbose {
				println("Websocket: Read wait on queue-")
			}

			tp, buf, err := ws.ws.Read(ws.ctx)
			if tp == websocket.MessageText {
				continue
			}
			if err != nil {
				if debugVerbose {
					println("Websocket: Read error", err)
				}
				return 0, err
			}
			ws.remaining = bytes.NewReader(buf)
		}

		// Read from chunk
		if debugVerbose {
			println("Websocket: Reading")
		}
		n, err := ws.remaining.Read(buf)
		if err == io.EOF {
			if closer, hasClose := ws.remaining.(io.Closer); hasClose {
				closer.Close()
			}
			ws.remaining, err = nil, nil
			if n < 1 {
				continue
			}
		}
		if debugVerbose {
			println("Websocket: Read", n, "bytes", "(content: "+fmt.Sprintf("%q", buf[:n])+")")
		}
		return n, err
	}
}

// SetDeadline implements the SetDeadline method, but is a no-op.
func (*WebSocket) SetDeadline(time.Time) (err error) {
	return nil
}

// SetWriteDeadline implements the SetWriteDeadline method, but is a no-op.
func (*WebSocket) SetWriteDeadline(time.Time) error {
	return nil
}

// SetReadDeadline implements the SetReadDeadline method but is a no-op.
func (*WebSocket) SetReadDeadline(time.Time) error {
	return nil
}

// setDeadline is used internally; Only call from New or Read!
func (ws *WebSocket) setDeadline(timer *time.Timer, future time.Time) error {
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
	if !future.IsZero() {
		timer.Reset(future.Sub(time.Now()))
	}
	return nil
}
