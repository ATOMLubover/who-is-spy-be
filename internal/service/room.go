package service

import (
	"encoding/json"
	"errors"
	"sync"
	"time"

	"who-is-spy-be/internal/service/dto"
	"who-is-spy-be/internal/service/game"

	"go.uber.org/zap"
)

type RoomService struct {
	mu         sync.Mutex
	gameHndMap map[string]*gameHandle
}

func NewRoomService() *RoomService {
	gameHndMap := make(map[string]*gameHandle)

	return &RoomService{
		gameHndMap: gameHndMap,
	}
}

type gameHandle struct {
	reqCh  chan game.RequestWrapper
	doneCh chan struct{}
}

func (rs *RoomService) CreateRoom(
	args dto.CreateRoomRequest,
) (
	*dto.CreateRoomResponse,
	error,
) {
	if args.RoomName == "" {
		return nil, errors.New("房间名称不能为空")
	}

	// 生成房间 ID
	roomID := game.GenID()[:8]

	// 创建房间对应的游戏状态机
	doneCh := make(chan struct{})

	gm := game.NewGameMachine(roomID, doneCh)

	// 将游戏状态机保存到 map 中
	if gm.GetReqCh() == nil {
		zap.L().Error(
			"游戏状态机的请求通道未能正确初始化",
			zap.String("room_id", roomID),
		)
		return nil, errors.New("游戏状态机未能初始化")
	}

	rs.mu.Lock()

	rs.gameHndMap[roomID] = &gameHandle{
		reqCh:  gm.GetReqCh(),
		doneCh: doneCh,
	}

	// 释放协程，启动游戏状态机的事件循环
	go func() {
		zap.L().Info(
			"启动游戏状态机协程",
			zap.String("room_id", roomID),
		)

		gm.Start()

		zap.L().Info(
			"游戏状态机协程已退出",
			zap.String("room_id", roomID),
		)
	}()

	rs.mu.Unlock()

	// 返回创建成功的响应
	resp := &dto.CreateRoomResponse{
		RoomID: roomID,
	}

	return resp, nil
}

// JoinRoom 等价于 Websocket 连接建立的初始化函数
func (rs *RoomService) JoinRoom(
	args *game.JoinGameRequest,
	respCh chan game.ResponseWrapper,
) (chan game.RequestWrapper, error) {
	if args.RoomID == "" || args.JoinerName == "" {
		return nil, errors.New("房间 ID 和加入者名称不能为空")
	}

	gameHnd, ok := rs.gameHndMap[args.RoomID]
	if !ok {
		return nil, errors.New("房间不存在")
	}

	// 构造加入请求
	req := game.JoinGameRequest{
		JoinerName: args.JoinerName,
		RespCh:     respCh,
	}

	rawReq, err := json.Marshal(req)
	if err != nil {
		zap.L().Error(
			"无法构造加入房间请求",
			zap.Error(err),
			zap.Any("args", args),
		)
		return nil, errors.New("无法构造加入房间请求")
	}

	wrapper := game.RequestWrapper{
		ReqType: game.REQ_JOIN_GAME,
		Data:    json.RawMessage(rawReq),
	}

	// 发送加入请求到游戏状态机
	select {
	case gameHnd.reqCh <- wrapper:
		zap.L().Info(
			"发送加入房间请求到游戏状态机",
			zap.String("room_id", args.RoomID),
			zap.String("joiner_name", args.JoinerName),
		)
	default:
		zap.L().Error(
			"发送加入房间请求失败：游戏状态机请求通道已满",
			zap.String("room_id", args.RoomID),
		)
		return nil, errors.New("房间繁忙，请稍后再试")
	}

	// 检查对应的 resp 是否接受到成功的响应
	select {
	case resp := <-respCh:
		// 如果是错误响应，直接返回错误
		if resp.ErrMsg != "" {
			zap.L().Error(
				"加入房间收到错误响应",
				zap.String("room_id", args.RoomID),
				zap.String("err", resp.ErrMsg),
			)
			return nil, errors.New(resp.ErrMsg)
		}

		// 期望收到的是 JoinGame 类型的响应
		if resp.RespType != game.RESP_JOIN_GAME {
			zap.L().Warn(
				"收到非加入类型响应",
				zap.String("room_id", args.RoomID),
				zap.String("resp_type", resp.RespType),
			)
			return nil, errors.New("加入房间失败：未收到加入确认")
		}

	case <-time.After(3 * time.Second):
		zap.L().Error(
			"等待加入响应超时",
			zap.String("room_id", args.RoomID),
		)
		return nil, errors.New("加入房间超时，请稍后重试")
	}

	// 如果成功，则返回请求通道
	rs.mu.Lock()
	reqCh := gameHnd.reqCh
	rs.mu.Unlock()

	return reqCh, nil
}
