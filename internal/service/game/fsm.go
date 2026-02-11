package game

import (
	"time"

	"go.uber.org/zap"
)

// GameMachine 是游戏状态机，负责管理游戏状态和事件循环
type GameMachine struct {
	ctx     *GameContext
	handler StageHandler
	// 这是所有的用户的请求汇总的通道
	reqCh chan RequestWrapper
	// 结束通道，用于通知游戏状态机退出事件循环
	doneCh chan struct{}

	createdAt time.Time
}

func NewGameMachine(roomID string, doneCh chan struct{}) *GameMachine {
	ctx := &GameContext{
		RoomID: roomID,
		TmoCh:  make(chan RequestWrapper, 64),
	}

	reqCh := make(chan RequestWrapper, 64)

	gm := &GameMachine{
		ctx:       ctx,
		handler:   NewWaitStageHandler(),
		reqCh:     reqCh,
		doneCh:    doneCh,
		createdAt: time.Now(),
	}

	// 设置 onSwitch 回调
	onSwitch := func(nextStage string) {
		gm.ctx.GameStage = nextStage
	}

	gm.handler.SetOnSwitch(onSwitch)

	return gm
}

func (gm *GameMachine) GetReqCh() chan RequestWrapper {
	return gm.reqCh
}

func (gm *GameMachine) Start() {
	// 执行初始 handler 的 OnEnter
	gm.handler.OnEnter(gm.ctx)

	// 进入事件循环
	for {
		// 从请求通道或超时通道接收事件
		var req RequestWrapper

		select {
		case req = <-gm.reqCh:
			zap.L().Debug(
				"接收到客户端请求",
				zap.String("room_id", gm.ctx.RoomID),
				zap.Any("request", req),
			)
		case req = <-gm.ctx.TmoCh:
			zap.L().Debug(
				"接收到超时事件",
				zap.String("room_id", gm.ctx.RoomID),
			)
		case <-gm.doneCh:
			zap.L().Info(
				"收到退出信号，结束游戏状态机",
				zap.String("room_id", gm.ctx.RoomID),
			)
			return
		}

		// 处理请求
		err := gm.handler.OnHandle(gm.ctx, req)
		if err != nil {
			zap.L().Debug(
				"处理请求失败",
				zap.Error(err),
				zap.String("stage", gm.handler.Stage()),
				zap.Any("request", req),
			)
		}

		// 检查状态是否发生变化
		if gm.ctx.GameStage != gm.handler.Stage() {
			// 状态发生变化，执行切换
			gm.switchStage()

			// 如果切换到了结束阶段，退出循环
			if gm.ctx.GameStage == STAGE_FINISHED {
				// 执行结束阶段的 OnEnter
				gm.handler.OnEnter(gm.ctx)
				break
			}

			// 执行新阶段的 OnEnter
			gm.handler.OnEnter(gm.ctx)
		}
	}

	// 游戏结束后，协程应当自动退出，释放资源
	zap.L().Info(
		"游戏状态机已结束",
		zap.String("room_id", gm.ctx.RoomID),
	)
}

func (gm *GameMachine) switchStage() {
	// 执行当前 handler 的 OnExit
	gm.handler.OnExit(gm.ctx)

	// 根据新状态创建对应的 handler
	var newHandler StageHandler

	switch gm.ctx.GameStage {
	case STAGE_WAITING:
		newHandler = NewWaitStageHandler()
	case STAGE_PREPARING:
		newHandler = NewPrepStageHandler()
	case STAGE_SPEAKING:
		newHandler = NewSpeakStageHandler()
	case STAGE_VOTING:
		newHandler = NewVoteStageHandler()
	case STAGE_JUDGING:
		newHandler = NewJudgeStageHandler()
	case STAGE_FINISHED:
		newHandler = NewFinishStageHandler()
	default:
		zap.L().Error(
			"未知的游戏阶段",
			zap.String("stage", gm.ctx.GameStage),
		)
		return
	}

	// 设置 onSwitch 回调
	onSwitch := func(nextStage string) {
		gm.ctx.GameStage = nextStage
	}

	newHandler.SetOnSwitch(onSwitch)

	// 更新当前 handler
	gm.handler = newHandler
}

func (gm *GameMachine) IsFinished() bool {
	return gm.ctx.GameStage == STAGE_FINISHED
}

func (gm *GameMachine) CreatedAt() time.Time {
	return gm.createdAt
}
