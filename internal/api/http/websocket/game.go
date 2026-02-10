package websocket

import (
	"encoding/json"
	"time"

	"who-is-spy-be/internal/service/game"
	"who-is-spy-be/internal/state"

	"github.com/gorilla/websocket"
	"github.com/kataras/iris/v12"
	"go.uber.org/zap"
)

func JoinGame(appState *state.AppState) iris.Handler {
	return func(ctx iris.Context) {
		conn, err := upgrader.Upgrade(
			ctx.ResponseWriter(),
			ctx.Request(),
			nil,
		)
		if err != nil {
			zap.L().Error("升级到WebSocket失败", zap.Error(err))
			ctx.StatusCode(iris.StatusBadRequest)
			return
		}

		defer conn.Close()

		conn.SetReadDeadline(time.Now().Add(HEARTBEAT_TIMEOUT))
		conn.SetPongHandler(heartbeatHandler(conn))

		respCh := make(chan game.ResponseWrapper)

		// 读取首次请求，获取必要的参数
		_, msg, err := conn.ReadMessage()
		if err != nil {
			zap.L().Error(
				"读取首次请求失败",
				zap.String("client_ip", ctx.RemoteAddr()),
				zap.Error(err),
			)
			return
		}

		var wrapper game.RequestWrapper

		if err := json.Unmarshal(msg, &wrapper); err != nil {
			zap.L().Error(
				"解析首次请求失败",
				zap.String("client_ip", ctx.RemoteAddr()),
				zap.Error(err),
			)

			return
		}

		req := game.TryUnwrapJoinGameRequest(wrapper)
		if req == nil {
			zap.L().Error(
				"首次请求不是JoinGame类型",
				zap.String("client_ip", ctx.RemoteAddr()),
				zap.Any("wrapper", wrapper),
			)

			return
		}

		// 先调用加入房间的接口，获取游戏状态机的请求通道
		reqCh, err := appState.RoomSvc.JoinRoom(req, respCh)
		if err != nil {
			zap.L().Error(
				"加入房间失败",
				zap.String("client_ip", ctx.RemoteAddr()),
				zap.Error(err),
			)

			return
		}

		// 写协程的退出信号
		writeDoneCh := make(chan struct{})
		defer close(writeDoneCh)

		clientIP := ctx.RemoteAddr()

		// 写入协程
		go func() {
			ticker := time.NewTicker(HEARTBEAT_INTERVAL)
			defer ticker.Stop()

			select {
			case <-writeDoneCh:
				zap.L().Info(
					"WebSocket写入协程退出",
					zap.String("client_ip", clientIP),
				)

			case <-ticker.C:
				if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					zap.L().Error(
						"发送心跳失败",
						zap.String("client_ip", clientIP),
						zap.Error(err),
					)
					return
				}

				conn.SetWriteDeadline(time.Now().Add(HEARTBEAT_TIMEOUT))

				zap.L().Debug(
					"发送心跳",
					zap.String("client_ip", clientIP),
				)

			case resp := <-respCh:
				if err := conn.WriteJSON(resp); err != nil {
					zap.L().Error(
						"发送消息失败",
						zap.String("client_ip", clientIP),
						zap.Error(err),
					)
					return
				}

				zap.L().Debug(
					"发送消息",
					zap.String("client_ip", clientIP),
					zap.Any("response", resp),
				)
			}
		}()

		// 读取协程（主协程）
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(
					err,
					websocket.CloseGoingAway,
					websocket.CloseAbnormalClosure,
				) {
					zap.L().Error(
						"读取消息失败",
						zap.String("client_ip", clientIP),
						zap.Error(err),
					)
				}

				break
			}

			// 解析消息
			var wrapper game.RequestWrapper

			if err := json.Unmarshal(msg, &wrapper); err != nil {
				zap.L().Error(
					"解析消息失败",
					zap.String("client_ip", clientIP),
					zap.Error(err),
				)

				// 解析石板，返回错误响应
				respCh <- game.WrapErrResponse("无效的请求格式")

				continue
			}

			// 将解析后的请求发送到游戏状态机
			select {
			case reqCh <- wrapper:
				zap.L().Debug(
					"发送请求到游戏状态机",
					zap.String("client_ip", clientIP),
					zap.Any("request_wrapper", wrapper),
				)
			default:
				zap.L().Error(
					"发送请求到游戏状态机失败：请求通道已满",
					zap.String("client_ip", clientIP),
				)

				// 返回错误响应
				respCh <- game.WrapErrResponse("房间繁忙，请稍后再试")
			}
		}
	}
}
