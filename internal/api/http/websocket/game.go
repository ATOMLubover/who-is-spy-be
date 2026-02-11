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

		// Buffer responses so server-side joins can read an ack without swallowing it from the client.
		respCh := make(chan game.ResponseWrapper, 64)

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

		// 等待并读取加入确认响应，获取玩家ID
		var playerID string
		var playerName string

		select {
		case joinResp := <-respCh:
			if joinResp.RespType == game.RESP_JOIN_GAME {
				// 提取玩家ID
				if respData, ok := joinResp.Data.(game.JoinGameResponse); ok {
					playerID = respData.Joiner.ID
					playerName = respData.Joiner.Name
				}

				// 将响应放回通道供写协程发送
				select {
				case respCh <- joinResp:
				default:
					zap.L().Warn("无法回放加入响应")
				}
			}
		case <-time.After(3 * time.Second):
			zap.L().Error("等待加入响应超时", zap.String("client_ip", ctx.RemoteAddr()))
			return
		}

		if playerID == "" {
			zap.L().Error("未能获取玩家ID", zap.String("client_ip", ctx.RemoteAddr()))
			return
		}

		zap.L().Info(
			"玩家成功加入房间",
			zap.String("client_ip", ctx.RemoteAddr()),
			zap.String("player_id", playerID),
			zap.String("player_name", playerName),
		)

		// 写协程的退出信号
		writeDoneCh := make(chan struct{})
		defer close(writeDoneCh)

		clientIP := ctx.RemoteAddr()

		// 写入协程
		go func() {
			ticker := time.NewTicker(HEARTBEAT_INTERVAL)
			defer ticker.Stop()

			for {
				select {
				case <-writeDoneCh:
					zap.L().Info(
						"WebSocket写入协程退出",
						zap.String("client_ip", clientIP),
					)
					return

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

				case resp, ok := <-respCh:
					// 检测到channel已关闭（玩家退出时状态机关闭了通道）
					if !ok {
						zap.L().Info(
							"响应通道已关闭，退出写协程",
							zap.String("client_ip", clientIP),
						)
						return
					}

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

		// 读循环退出，表示客户端断开连接
		// 发送 ExitGame 请求通知游戏状态机清理玩家
		zap.L().Info(
			"客户端连接断开，发送退出请求",
			zap.String("client_ip", clientIP),
			zap.String("player_id", playerID),
		)

		exitReq := game.ExitGameRequest{
			PlayerID: playerID,
			RespCh:   respCh,
		}

		exitWrapper := game.RequestWrapper{
			ReqType:    game.REQ_EXIT_GAME,
			NativeData: &exitReq,
		}

		// 发送退出请求
		select {
		case reqCh <- exitWrapper:
			zap.L().Debug(
				"发送退出请求成功",
				zap.String("player_id", playerID),
			)
		default:
			zap.L().Warn(
				"发送退出请求失败：请求通道已满",
				zap.String("player_id", playerID),
			)
			// 即使发送失败也继续等待，确保资源回收
		}

		// 等待退出确认响应或超时
		select {
		case resp, ok := <-respCh:
			if !ok {
				// 通道已关闭，说明状态机已处理退出
				zap.L().Info(
					"响应通道已关闭，玩家退出完成",
					zap.String("player_id", playerID),
				)
			} else if resp.RespType == game.RESP_EXIT_GAME {
				zap.L().Info(
					"收到退出确认响应",
					zap.String("player_id", playerID),
				)
			} else {
				// 可能是其他响应，继续等待
				zap.L().Debug(
					"收到非退出响应，继续等待",
					zap.String("player_id", playerID),
					zap.String("resp_type", resp.RespType),
				)
			}
		case <-time.After(3 * time.Second):
			zap.L().Warn(
				"等待退出确认超时，强制退出",
				zap.String("player_id", playerID),
			)
		}

		zap.L().Info(
			"WebSocket连接处理完成",
			zap.String("client_ip", clientIP),
			zap.String("player_id", playerID),
		)
	}
}

func mustMarshal(v interface{}) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		zap.L().Error("marshal failed", zap.Error(err))
		return nil
	}
	return data
}
