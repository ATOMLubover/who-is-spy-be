package game

import (
	"errors"
	"math/rand/v2"
)

// 游戏总体分为 6 个阶段，分别是：
// 1. 等待阶段（Waiting）：玩家可以加入房间，等待管理员开始游戏
// 2. 准备阶段（Preparing）：管理员选择词语，准备开始游戏
// 3. 发言阶段（Speaking）：每个玩家轮流发言，其他玩家可以进行猜测
// 4. 投票阶段（Voting）：玩家对发言者进行投票，选出卧底
// 5. 判定阶段（Judging）：根据投票结果判定游戏结果，宣布胜利方
// 6. 结束阶段（Finished）：游戏结束，玩家将离开房间
const (
	STATUS_WAITING   = "Waiting"
	STATUS_PREPARING = "Preparing"
	STATUS_SPEAKING  = "Speaking"
	STATUS_VOTING    = "Voting"
	STATUS_JUDGING   = "Judging"
	STATUS_FINISHED  = "Finished"
)

type StageHandler interface {
	Stage() string

	OnEnter(ctx *GameContext)
	OnHandle(ctx *GameContext, req RequestWrapper) error
	OnExit(ctx *GameContext)

	SetOnSwitch(func(nextStage string))
}

// 等待阶段是整个游戏最初始的阶段
type waitStageHandler struct {
	onSwitch func(string)
}

func (wsh *waitStageHandler) Stage() string {
	return STATUS_WAITING
}

func (wsh *waitStageHandler) OnEnter(ctx *GameContext) {
	// 初始化上下文
	ctx.RoomID = GenID()[:8] // 生成一个简短的房间 ID
	ctx.GameStatus = STATUS_WAITING
	ctx.Players = make(map[string]*Player, 0)

	ctx.Answer = ""
	ctx.WordList = make([]string, 0)
}

func (wsh *waitStageHandler) OnHandle(ctx *GameContext, req RequestWrapper) error {
	// 在等待阶段只处理 JoinGame、SetWords 和 StartGame 请求
	if req := TryUnwrapJoinGameRequest(req); req != nil {
		playerID := GenID()[:8]

		player := Player{
			ID:     playerID,
			Name:   req.JoinerName,
			ReqCh:  req.ReqCh,
			RespCh: req.RespCh,
		}

		onPlayerJoin(ctx, player)

		return nil
	}

	if req := TryUnwrapSetWordsRequest(req); req != nil {
		adminPlayer := ctx.GetAdmin()
		if adminPlayer == nil {
			return errors.New("无法设置词库：当前没有管理员")
		}

		if adminPlayer.ID != req.SetPlayerID {
			return errors.New("无法设置词库：只有管理员可以设置词库")
		}

		// 更新词库
		ctx.WordList = req.WordList

		// 发送通知
		resp := WrapResponse(
			RESP_SET_WORDS,
			SetWordsResponse{
				WordList: ctx.WordList,
			},
		)

		ctx.BroadcastResp(resp)

		return nil
	}

	if req := TryUnwrapStartGameRequest(req); req != nil {
		adminPlayer := ctx.GetAdmin()
		if adminPlayer == nil {
			return errors.New("无法开始游戏：当前没有管理员")
		}

		if adminPlayer.ID != req.StartPlayerID {
			return errors.New("无法开始游戏：只有管理员可以开始游戏")
		}

		// 切换到准备阶段
		wsh.onSwitch(STATUS_PREPARING)

		return nil
	}

	return errors.New("无法处理请求：当前阶段不支持该请求类型")
}

func assignRolesAndWords(ctx *GameContext) {
	// 根据词库，分配角色和词语
	// 使用随机数，抽出一个谜底词和一个卧底词，剩下的玩家分配普通角色
	var (
		answer     string
		undercover string
	)

	answerIndex := rand.IntN(len(ctx.WordList))
	answer = ctx.WordList[answerIndex]

	// 去除谜底词，重新随机抽取一个卧底词
	andyWordList := append(
		ctx.WordList[:answerIndex],
		ctx.WordList[answerIndex+1:]...,
	)

	undercoverIndex := rand.IntN(len(andyWordList))
	undercover = andyWordList[undercoverIndex]

	// 抽选一个白板，一个卧底，其次为普通玩家
	slicedPlayers := make([]*Player, 0, len(ctx.Players))
	for _, p := range ctx.Players {
		slicedPlayers = append(slicedPlayers, p)
	}

	blankIndex := rand.IntN(len(ctx.Players))
	blankPlayer := slicedPlayers[blankIndex]

	andyPlayers := append(
		slicedPlayers[:blankIndex],
		slicedPlayers[blankIndex+1:]...,
	)

	undercoverPlayerIndex := rand.IntN(len(andyPlayers))
	undercoverPlayer := andyPlayers[undercoverPlayerIndex]

	// 最后分配角色和词语
	ctx.Answer = answer
	ctx.Undercover = undercover

	blankPlayer.Role = ROLE_BLANK
	blankPlayer.Word = ""

	undercoverPlayer.Role = ROLE_SPY
	undercoverPlayer.Word = undercover
}

func (wsh *waitStageHandler) OnExit(ctx *GameContext) {
	// 将游戏阶段切换为准备阶段
	ctx.GameStatus = STATUS_PREPARING
}

type speakStageHandler struct {
}

func onPlayerJoin(ctx *GameContext, player Player) {
	// 一局游戏最多支持 8 个正常玩家（不包括管理员和观察者）
	const PLAYER_THRESHOLD = 8

	// 检查当前的游戏是否已经满员
	if len(ctx.Players) >= PLAYER_THRESHOLD+1 {
		// 超过玩家上限，默认变为观察者身份
		player.Role = ROLE_OBSERVER

		ctx.Players[player.ID] = &player

		return
	}

	// 没有超出上限，则检查是否是等待阶段
	if ctx.GameStatus == STATUS_WAITING {
		// 如果是等待阶段，则玩家可以直接进入游戏，默认身份为 UNSET
		player.Role = ROLE_UNSET

		ctx.Players[player.ID] = &player

		return
	}

	// 否则，玩家只能以观察者身份加入游戏
	player.Role = ROLE_OBSERVER

	ctx.Players[player.ID] = &player
}
