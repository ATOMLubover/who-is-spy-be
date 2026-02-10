package websocket

import (
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		// NOTE: 暂时允许所有来源
		return true
	},
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

const (
	// 心跳间隔，单位秒
	HEARTBEAT_INTERVAL = 30 * time.Second
	// 心跳超时时间，单位秒
	HEARTBEAT_TIMEOUT = 45 * time.Second
)

var heartbeatHandler = func(conn *websocket.Conn) func(string) error {
	return func(string) error {
		conn.SetReadDeadline(time.Now().Add(HEARTBEAT_TIMEOUT))
		return nil
	}
}
