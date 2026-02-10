package service

import (
	"errors"
	"sync"
	"time"
	"undercover-be/internal/service/dto"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type RoomService struct {
	state *roomServiceState
}

type roomServiceState struct {
	mu sync.RWMutex

	// 均为从 ID 到实体的映射
	rooms             map[string]*dto.Room
	roomReqChList     map[string]chan RoomRequestAction
	roomResChList     map[string]chan struct{}
	roomJoinResChList map[string]chan joinRoomResponseWrapper

	cleanUpDone chan struct{}
}

func NewRoomService() *RoomService {
	cleanUpDone := make(chan struct{})

	state := &roomServiceState{
		rooms:             make(map[string]*dto.Room),
		roomReqChList:     make(map[string]chan RoomRequestAction),
		roomResChList:     make(map[string]chan struct{}),
		roomJoinResChList: make(map[string]chan joinRoomResponseWrapper),
		cleanUpDone:       cleanUpDone,
	}

	// 启动一个 goroutine 定期清理过期的房间
	go startCleanupLoop(state)


	return &RoomService{
		state: state,
	}
}

func startCleanupLoop(state *roomServiceState) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-state.cleanUpDone:
			return

		case <-ticker.C:
			state.mu.Lock()

			for roomID, room := range state.rooms {
				if !isRoomValid(room) {
					zap.S().Infof("房间 %s 状态失效，开始清理", roomID)

					// 通知对应的房间 goroutine 退出
					reqCh := state.roomReqChList[roomID]
					reqCh <- RoomRequestAction{
						Done: &struct{}{},
					}

					zap.S().Debugf("房间 %s 已发送关闭请求", roomID)

					delete(state.rooms, roomID)

					close(state.roomReqChList[roomID])
					delete(state.roomReqChList, roomID)

					close(state.roomResChList[roomID])
					delete(state.roomResChList, roomID)

					close(state.roomJoinResChList[roomID])
					delete(state.roomJoinResChList, roomID)
				}
			}

			state.mu.Unlock()
		}
	}
}

func (rs *RoomService) Close() {
	close(rs.state.cleanUpDone)
}

func (rs *RoomService) CreateRoom(req dto.CreateRoomRequest) (dto.CreateRoomResponse, error) {
	if req.RoomName == "" {
		return dto.CreateRoomResponse{}, errors.New("房间名称不能为空")
	}
	if req.CreatorName == "" {
		return dto.CreateRoomResponse{}, errors.New("创建者名称不能为空")
	}

	// 构建 UUID
	roomID := uuid.New().String()[:8]
	creatorID := uuid.New().String()[:8]

	rs.state.mu.Lock()

	// 创建玩家
	player := &dto.Player{
		ID:   creatorID,
		Name: req.CreatorName,
		Role: dto.ROLE_ADMIN,
	}

	joinedPlayers := make([]dto.Player, 0, 1)
	joinedPlayers = append(joinedPlayers, *player)

	// 添加房间
	room := &dto.Room{
		ID:            roomID,
		Name:          req.RoomName,
		AdminID:       creatorID,
		JoinedPlayers: joinedPlayers,
		Words:         make([]string, 0),
	}

	rs.state.rooms[roomID] = room

	// 创建对应的独立 goroutine 来处理这个房间的定时任务
	recvCh := make(chan RoomRequestAction)
	rs.state.roomReqChList[roomID] = recvCh

	notifyCh := make(chan struct{})
	rs.state.roomResChList[roomID] = notifyCh

	joinResCh := make(chan joinRoomResponseWrapper)
	rs.state.roomJoinResChList[roomID] = joinResCh

	go rs.roomLoop(room, recvCh, notifyCh, joinResCh)

	rs.state.mu.Unlock()

	zap.S().Infof("房间 %s 由 %s 创建", roomID, req.CreatorName)

	return dto.CreateRoomResponse{
		RoomID:  roomID,
		Creator: *player,
	}, nil
}

func (rs *RoomService) JoinRoom(req dto.JoinRoomRequest) (dto.JoinRoomResponse, error) {
	if req.RoomID == "" {
		return dto.JoinRoomResponse{}, errors.New("房间 ID 不能为空")
	}
	if req.JoinerName == "" {
		return dto.JoinRoomResponse{}, errors.New("加入者名称不能为空")
	}

	rs.state.mu.RLock()
	defer rs.state.mu.RUnlock()

	room := rs.state.rooms[req.RoomID]
	if room == nil {
		return dto.JoinRoomResponse{}, errors.New("房间不存在")
	}

	// 发送加入房间的请求给对应的房间 goroutine
	reqCh := rs.state.roomReqChList[req.RoomID]
	joinResCh := rs.state.roomJoinResChList[req.RoomID]

	joinReqAction := RoomRequestAction{
		JoinRoomReq: &req,
	}

	zap.S().Debugf("房间 %s 收到加入请求：%s", req.RoomID, req.JoinerName)

	reqTimer := time.NewTimer(5 * time.Second)

	select {
	case reqCh <- joinReqAction:
		if !reqTimer.Stop() {
			select {
			case <-reqTimer.C:
			default:
			}
		}

	case <-reqTimer.C:
		zap.S().Warnf("房间 %s 无法及时处理加入请求，%s 发送失败", req.RoomID, req.JoinerName)
		return dto.JoinRoomResponse{}, errors.New("加入房间失败")
	}

	resTimer := time.NewTimer(5 * time.Second)
	select {
	case res, ok := <-joinResCh:
		if !resTimer.Stop() {
			select {
			case <-resTimer.C:
			default:
			}
		}

		if !ok {
			zap.S().Warnf("房间 %s 已经关闭，%s 无法加入", req.RoomID, req.JoinerName)
			return dto.JoinRoomResponse{}, errors.New("加入房间失败")
		}

		if res.Err != nil {
			zap.S().Warnf("房间 %s 处理 %s 加入失败：%v", req.RoomID, req.JoinerName, res.Err)
		} else {
			zap.S().Infof("房间 %s 接纳玩家 %s", req.RoomID, req.JoinerName )
		}

		return res.JoinRoomResp, res.Err

	case <-resTimer.C:
		zap.S().Warnf("房间 %s 加入请求响应超时：%s", req.RoomID, req.JoinerName)
		return dto.JoinRoomResponse{}, errors.New("加入房间失败")
	}
}

func (*RoomService) roomLoop(
	room *dto.Room,
	reqCh <-chan RoomRequestAction,
	resCh chan<- struct{},
	joinResCh chan<- joinRoomResponseWrapper,
) {
	defer func() {
		close(joinResCh)
		close(resCh)

		zap.S().Infof("房间 %s 协程退出", room.ID)
	}()

	for req := range reqCh {
		if req.Done != nil {
			zap.S().Infof("房间 %s 收到关闭指令", room.ID)
			return
		}

		if req.JoinRoomReq != nil {
			zap.S().Debugf("房间 %s 处理加入请求：%s", room.ID, req.JoinRoomReq.JoinerName)
			res, err := handleJoinRoom(req.JoinRoomReq, room)
			if err != nil {
				zap.S().Warnf("房间 %s 处理 %s 加入失败：%v", room.ID, req.JoinRoomReq.JoinerName, err)
				joinResCh <- joinRoomResponseWrapper{Err: err}
				continue
			}

			joinResCh <- joinRoomResponseWrapper{JoinRoomResp: res}
			zap.S().Infof("房间 %s 玩家 %s(%s) 加入", room.ID, res.Joiner.Name, res.Joiner.Role)
		}
	}

	zap.S().Infof("房间 %s 请求通道已关闭", room.ID)
}

func handleJoinRoom(req *dto.JoinRoomRequest, room *dto.Room) (dto.JoinRoomResponse, error) {
	const MAX_PLAYERS = 8

	playerID := uuid.New().String()[:8]

	player := dto.Player{
		ID:   playerID,
		Name: req.JoinerName,
	}

	// 如果超过 9 个人（一个 admin + 八个普通玩家）
	// 则自动变成观众
	if len(room.JoinedPlayers) >= MAX_PLAYERS {
		player.Role = dto.ROLE_OBSERVER
	}

	room.JoinedPlayers = append(room.JoinedPlayers, player)

	return dto.JoinRoomResponse{
		Joiner: player,
	}, nil
}
