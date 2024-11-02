package websocketmanager

import (
	"errors"
	"net/url"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/neosouler7/bookstore-contango/tgmanager"
)

var (
	w            *websocket.Conn
	w2           *websocket.Conn
	once         sync.Once
	ErrReadMsg   = errors.New("reading msg on ws")
	SubscribeMsg = "%s websocket subscribed!\n"
	FilteredMsg  = "%s websocket msg filtered - %s\n"
)

const (
	bin string = "stream.binance.com:9443"
	bif string = "fstream.binance.com"
	byb string = "stream.bybit.com"
	byf string = "stream.bybit.com"
)

type hostPath struct {
	host string
	path string
}

type webSocketStruct struct {
	connections map[string]*websocket.Conn
	mutex       sync.Mutex
}

var wsManager = &webSocketStruct{
	connections: make(map[string]*websocket.Conn),
}

func Conn(exchange string) *websocket.Conn {
	wsManager.mutex.Lock()
	defer wsManager.mutex.Unlock()

	if conn, exists := wsManager.connections[exchange]; exists {
		return conn
	}

	h := &hostPath{}
	h.getHostPath(exchange)

	u := url.URL{Scheme: "wss", Host: h.host, Path: h.path}
	wPointer, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		tgmanager.HandleErr(exchange, err)
		return nil
	}

	wsManager.connections[exchange] = wPointer
	return wPointer
}

// func Conn(exchange string) *websocket.Conn {
// 	once.Do(func() {
// 		h := &hostPath{}
// 		h.getHostPath(exchange)

// 		u := url.URL{Scheme: "wss", Host: h.host, Path: h.path}
// 		wPointer, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
// 		tgmanager.HandleErr(exchange, err)
// 		w = wPointer
// 	})
// 	return w
// }

func (h *hostPath) getHostPath(exchange string) {
	switch exchange {
	case "bin":
		h.host = bin
		h.path = "/stream"
	case "bif":
		h.host = bif
		h.path = "/stream"
	case "byb":
		h.host = byb
		h.path = "/v5/public/spot"
	case "byf":
		h.host = byf
		h.path = "/v5/public/linear"
	}
}

func SendMsg(exchange, msg string) {
	err := Conn(exchange).WriteMessage(websocket.TextMessage, []byte(msg))
	tgmanager.HandleErr(exchange, err)
}

func Pong(exchange string) {
	err := Conn(exchange).WriteMessage(websocket.PongMessage, []byte{})
	tgmanager.HandleErr(exchange, err)
}
