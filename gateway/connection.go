package gateway

import (
	"sync"

	"github.com/gorilla/websocket"
	"github.com/spec-tacles/gateway/compression"
)

// Connection wraps a websocket connection
type Connection struct {
	ws         *websocket.Conn
	compressor compression.Compressor
	rmux       *sync.Mutex
	wmux       *sync.Mutex
}

// NewConnection creates a new ReadWriteCloser wrapper around a connection
func NewConnection(conn *websocket.Conn, compressor compression.Compressor) (c *Connection) {
	return &Connection{
		ws:         conn,
		compressor: compressor,
		rmux:       &sync.Mutex{},
		wmux:       &sync.Mutex{},
	}
}

// CloseWithCode closes the connection with the specified code
func (c *Connection) CloseWithCode(code int) error {
	return c.ws.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(code, "Normal Closure"))
}

// Close closes this connection
func (c *Connection) Close() error {
	return c.CloseWithCode(websocket.CloseNormalClosure)
}

func (c *Connection) Write(d []byte) (int, error) {
	// d = c.compressor.Compress(d)

	c.wmux.Lock()
	defer c.wmux.Unlock()

	return len(d), c.ws.WriteMessage(websocket.BinaryMessage, d)
}

func (c *Connection) Read() (d []byte, err error) {
	c.rmux.Lock()
	defer c.rmux.Unlock()

	t, d, err := c.ws.ReadMessage()
	if err != nil {
		return
	}

	if t == websocket.BinaryMessage {
		d, err = c.compressor.Decompress(d)
	}

	return
}
