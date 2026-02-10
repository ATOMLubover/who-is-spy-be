package game

import (
	"errors"
	"math/rand/v2"
	"time"
)

// 游戏总体分为 6 个阶段，分别是：
// 1. 等待阶段（Waiting）：玩家可以加入房间，等待管理员开始游戏
// 2. 准备阶段（Preparing）：管理员选择词语，准备开始游戏
// 3. 发言阶段（Speaking）：每个玩家轮流发言，其他玩家可以进行猜测
// 4. 投票阶段（Voting）：玩家对发言者进行投票，选出卧底
// 5. 判定阶段（Judging）：根据投票结果判定游戏结果，宣布胜利方
// 6. 结束阶段（Finished）：游戏结束，玩家将离开房间
const (
	STAGE_WAITING   = "Waiting"
	STAGE_PREPARING = "Preparing"
	STAGE_SPEAKING  = "Speaking"
	STAGE_VOTING    = "Voting"
	STAGE_JUDGING   = "Judging"
	STAGE_FINISHED  = "Finished"
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

func NewWaitStageHandler() *waitStageHandler {
	return &waitStageHandler{}
}

func (wsh *waitStageHandler) Stage() string {
	return STAGE_WAITING
}

func (wsh *waitStageHandler) OnEnter(ctx *GameContext) {
	// 初始化上下文
	ctx.RoomID = GenID()[:8] // 生成一个简短的房间 ID
	ctx.GameStage = STAGE_WAITING
	ctx.Players = make(map[string]*Player, 0)

	ctx.Answer = ""
	ctx.WordList = make([]string, 0)
}

func (wsh *waitStageHandler) OnHandle(ctx *GameContext, req RequestWrapper) error {
	// 在等待阶段只处理 JoinGame、SetWords 和 StartGame 请求
	if req := TryUnwrapJoinGameRequest(req); req != nil {
		playerID := GenID()[:8]

		player := Player{
			ID:   playerID,
			Name: req.JoinerName,
			// ReqCh:  req.ReqCh,
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

		// 检查玩家数量
		normalPlayerCount := 0
		for _, p := range ctx.Players {
			if p.Role == ROLE_UNSET {
				normalPlayerCount++
			}
		}

		if normalPlayerCount < 8 {
			return errors.New("无法开始游戏：玩家数量不足 8 人")
		}

		// 切换到准备阶段
		wsh.onSwitch(STAGE_PREPARING)

		return nil
	}

	return errors.New("无法处理请求：当前阶段不支持该请求类型")
}

func assignRolesAndWords(ctx *GameContext) {
	// 根据词库，分配角色和词语
	// 使用随机数，抽出一个谜底词和一个卧底词，剩下的玩家分配普通角色
	var (
		answer  string
		spyWord string
	)

	answerIndex := rand.IntN(len(ctx.WordList))
	answer = ctx.WordList[answerIndex]

	// 去除谜底词，重新随机抽取一个卧底词
	tempWordList := append(
		ctx.WordList[:answerIndex],
		ctx.WordList[answerIndex+1:]...,
	)

	undercoverIndex := rand.IntN(len(tempWordList))
	spyWord = tempWordList[undercoverIndex]

	// 抽选一个白板，一个卧底，其次为普通玩家
	slicedPlayers := make([]*Player, 0, len(ctx.Players))
	for _, p := range ctx.Players {
		if p.Role == ROLE_UNSET {
			slicedPlayers = append(slicedPlayers, p)
		}
	}

	blankIndex := rand.IntN(len(slicedPlayers))
	blankPlayer := slicedPlayers[blankIndex]

	tempPlayers := append(
		slicedPlayers[:blankIndex],
		slicedPlayers[blankIndex+1:]...,
	)

	undercoverPlayerIndex := rand.IntN(len(tempPlayers))
	undercoverPlayer := tempPlayers[undercoverPlayerIndex]

	// 最后分配角色和词语
	ctx.AnswerWord = answer
	ctx.SpyWord = spyWord

	blankPlayer.Role = ROLE_BLANK
	blankPlayer.Word = ""

	undercoverPlayer.Role = ROLE_SPY
	undercoverPlayer.Word = spyWord

	// 为剩余的玩家分配普通角色
	for _, p := range slicedPlayers {
		if p.Role == ROLE_UNSET {
			p.Role = ROLE_NORMAL
			p.Word = answer
		}
	}
}

func (wsh *waitStageHandler) OnExit(ctx *GameContext) {
}

func (wsh *waitStageHandler) SetOnSwitch(onSwitch func(string)) {
	wsh.onSwitch = onSwitch
}

// 准备阶段处理器
type prepStageHandler struct {
	onSwitch func(string)
}

func NewPrepStageHandler() *prepStageHandler {
	return &prepStageHandler{}
}

func (psh *prepStageHandler) Stage() string {
	return STAGE_PREPARING
}

func (psh *prepStageHandler) OnEnter(ctx *GameContext) {
	// 分配角色和词语
	assignRolesAndWords(ctx)

	// 初始化游戏状态
	ctx.Round = 1
	ctx.SpeakingOrder = make([]string, 0)
	ctx.CurrentSpeakerIdx = 0
	ctx.Votes = make(map[string]string)

	// 向每个玩家发送其身份和词语
	for _, p := range ctx.Players {
		if p.Role != ROLE_ADMIN && p.Role != ROLE_OBSERVER {
			resp := WrapResponse(
				RESP_START_GAME,
				StartGameResponse{
					AssignedRole: p.Role,
					AssignedWord: p.Word,
				},
			)

			ctx.UnicastResp(p.ID, resp)
		}
	}

	// 10 秒后自动切换到发言阶段
	ctx.SetTimeout(10 * time.Second)
}

func (psh *prepStageHandler) OnHandle(ctx *GameContext, req RequestWrapper) error {
	// 处理超时请求
	if req := TryUnwrapTimeoutRequest(req); req != nil {
		if req.Stage == STAGE_PREPARING {
			// 超时，切换到发言阶段
			psh.onSwitch(STAGE_SPEAKING)
			return nil
		}
	}

	// 准备阶段不处理其他任何请求
	return errors.New("准备阶段不接受玩家请求")
}

func (psh *prepStageHandler) OnExit(ctx *GameContext) {
	ctx.ClearTimeout()
}

func (psh *prepStageHandler) SetOnSwitch(onSwitch func(string)) {
	psh.onSwitch = onSwitch
}

// 发言阶段处理器
type speakStageHandler struct {
	onSwitch func(string)
}

func NewSpeakStageHandler() *speakStageHandler {
	return &speakStageHandler{}
}

func (ssh *speakStageHandler) Stage() string {
	return STAGE_SPEAKING
}

func (ssh *speakStageHandler) OnEnter(ctx *GameContext) {
	// 初始化发言顺序（随机打乱存活玩家）
	alivePlayers := ctx.GetAlivePlayers()
	ctx.SpeakingOrder = make([]string, 0, len(alivePlayers))

	for _, p := range alivePlayers {
		ctx.SpeakingOrder = append(ctx.SpeakingOrder, p.ID)
	}

	// 随机打乱顺序
	rand.Shuffle(len(ctx.SpeakingOrder), func(i, j int) {
		ctx.SpeakingOrder[i], ctx.SpeakingOrder[j] = ctx.SpeakingOrder[j], ctx.SpeakingOrder[i]
	})

	ctx.CurrentSpeakerIdx = 0

	// 广播进入发言阶段
	currentPlayer := ctx.Players[ctx.SpeakingOrder[0]]
	stateNotif := WrapResponse(
		RESP_GAME_STATE,
		GameStateNotification{
			Stage:           STAGE_SPEAKING,
			CurrentTurnID:   currentPlayer.ID,
			CurrentTurnName: currentPlayer.Name,
			Round:           ctx.Round,
		},
	)

	ctx.BroadcastResp(stateNotif)

	// 设置 20 秒超时
	ctx.SetTimeout(20 * time.Second)
}

func (ssh *speakStageHandler) OnHandle(ctx *GameContext, req RequestWrapper) error {
	// 处理超时请求
	if req := TryUnwrapTimeoutRequest(req); req != nil {
		if req.Stage == STAGE_SPEAKING {
			// 当前发言者超时，移动到下一个发言者
			ctx.CurrentSpeakerIdx++

			// 检查是否所有人都已发言
			if ctx.CurrentSpeakerIdx >= len(ctx.SpeakingOrder) {
				// 所有人都已发言，切换到投票阶段
				ssh.onSwitch(STAGE_VOTING)
				return nil
			}

			// 通知下一位玩家发言
			nextPlayer := ctx.Players[ctx.SpeakingOrder[ctx.CurrentSpeakerIdx]]
			stateNotif := WrapResponse(
				RESP_GAME_STATE,
				GameStateNotification{
					Stage:           STAGE_SPEAKING,
					CurrentTurnID:   nextPlayer.ID,
					CurrentTurnName: nextPlayer.Name,
					Round:           ctx.Round,
				},
			)

			ctx.BroadcastResp(stateNotif)

			// 重新设置 20 秒超时
			ctx.SetTimeout(20 * time.Second)

			return nil
		}
	}

	if req := TryUnwrapDescribeRequest(req); req != nil {
		// 验证是否轮到该玩家
		currentSpeakerID := ctx.SpeakingOrder[ctx.CurrentSpeakerIdx]
		if req.ReqPlayerID != currentSpeakerID {
			return errors.New("当前不是你的发言轮次")
		}

		// 广播发言内容
		speaker := ctx.Players[req.ReqPlayerID]
		descResp := WrapResponse(
			RESP_DESCRIBE,
			DescribeResponse{
				SpeakerID:   speaker.ID,
				SpeakerName: speaker.Name,
				Message:     req.Message,
			},
		)

		ctx.BroadcastResp(descResp)

		// 移动到下一个发言者
		ctx.CurrentSpeakerIdx++

		// 检查是否所有人都已发言
		if ctx.CurrentSpeakerIdx >= len(ctx.SpeakingOrder) {
			// 所有人都已发言，切换到投票阶段
			ssh.onSwitch(STAGE_VOTING)
			return nil
		}

		// 通知下一位玩家发言
		nextPlayer := ctx.Players[ctx.SpeakingOrder[ctx.CurrentSpeakerIdx]]
		stateNotif := WrapResponse(
			RESP_GAME_STATE,
			GameStateNotification{
				Stage:           STAGE_SPEAKING,
				CurrentTurnID:   nextPlayer.ID,
				CurrentTurnName: nextPlayer.Name,
				Round:           ctx.Round,
			},
		)

		ctx.BroadcastResp(stateNotif)

		// 重新设置 20 秒超时
		ctx.SetTimeout(20 * time.Second)

		return nil
	}

	return errors.New("发言阶段只接受发言请求")
}

func (ssh *speakStageHandler) OnExit(ctx *GameContext) {
	ctx.ClearTimeout()
}

func (ssh *speakStageHandler) SetOnSwitch(onSwitch func(string)) {
	ssh.onSwitch = onSwitch
}

// 投票阶段处理器
type voteStageHandler struct {
	onSwitch func(string)
}

func NewVoteStageHandler() *voteStageHandler {
	return &voteStageHandler{}
}

func (vsh *voteStageHandler) Stage() string {
	return STAGE_VOTING
}

func (vsh *voteStageHandler) OnEnter(ctx *GameContext) {
	// 清空投票记录
	ctx.Votes = make(map[string]string)

	// 广播进入投票阶段
	stateNotif := WrapResponse(
		RESP_GAME_STATE,
		GameStateNotification{
			Stage: STAGE_VOTING,
			Round: ctx.Round,
		},
	)

	ctx.BroadcastResp(stateNotif)

	// 设置 30 秒超时
	ctx.SetTimeout(30 * time.Second)
}

func (vsh *voteStageHandler) OnHandle(ctx *GameContext, req RequestWrapper) error {
	// 处理超时请求
	if req := TryUnwrapTimeoutRequest(req); req != nil {
		if req.Stage == STAGE_VOTING {
			// 超时，切换到判定阶段
			vsh.onSwitch(STAGE_JUDGING)
			return nil
		}
	}

	if req := TryUnwrapVoteRequest(req); req != nil {
		// 验证投票者是否存活
		voter, ok := ctx.Players[req.VoterID]
		if !ok {
			return errors.New("投票者不存在")
		}

		if voter.Role == ROLE_OBSERVER || voter.Role == ROLE_ADMIN {
			return errors.New("观察者和管理员不能投票")
		}

		// 验证被投票者是否存活
		target, ok := ctx.Players[req.TargetID]
		if !ok {
			return errors.New("被投票者不存在")
		}

		if target.Role == ROLE_OBSERVER || target.Role == ROLE_ADMIN {
			return errors.New("不能投票给观察者或管理员")
		}

		// 记录投票
		ctx.Votes[req.VoterID] = req.TargetID

		// 广播投票信息
		voteResp := WrapResponse(
			RESP_VOTE,
			VoteResponse{
				VoterID:    voter.ID,
				VoterName:  voter.Name,
				TargetID:   target.ID,
				TargetName: target.Name,
			},
		)

		ctx.BroadcastResp(voteResp)

		// 检查是否所有存活玩家都已投票
		aliveCount := ctx.CountAlive()
		if len(ctx.Votes) >= aliveCount {
			// 所有人都已投票，切换到判定阶段
			vsh.onSwitch(STAGE_JUDGING)
		}

		return nil
	}

	return errors.New("投票阶段只接受投票请求")
}

func (vsh *voteStageHandler) OnExit(ctx *GameContext) {
	ctx.ClearTimeout()
}

func (vsh *voteStageHandler) SetOnSwitch(onSwitch func(string)) {
	vsh.onSwitch = onSwitch
}

// 判定阶段处理器
type judgeStageHandler struct {
	onSwitch func(string)
}

func NewJudgeStageHandler() *judgeStageHandler {
	return &judgeStageHandler{}
}

func (jsh *judgeStageHandler) Stage() string {
	return STAGE_JUDGING
}

func (jsh *judgeStageHandler) OnEnter(ctx *GameContext) {
	// 计票
	voteCount := make(map[string]int)
	for _, targetID := range ctx.Votes {
		voteCount[targetID]++
	}

	// 找出得票最多的玩家
	var eliminatedID string
	maxVotes := 0

	for targetID, count := range voteCount {
		if count > maxVotes {
			maxVotes = count
			eliminatedID = targetID
		}
	}

	// 淘汰该玩家
	eliminated := ctx.Players[eliminatedID]
	eliminatedWord := eliminated.Word
	eliminated.Role = ROLE_OBSERVER

	// 广播淘汰信息
	elimNotif := WrapResponse(
		RESP_ELIMINATE,
		EliminateNotification{
			EliminatedID:   eliminated.ID,
			EliminatedName: eliminated.Name,
			EliminatedWord: eliminatedWord,
		},
	)

	ctx.BroadcastResp(elimNotif)

	// 检查胜利条件
	aliveCount := ctx.CountAlive()
	spyAlive := ctx.IsSpyAlive()
	blankAlive := ctx.IsBlankAlive()

	// 卧底/白板方胜利：存活人数 < 4 且 卧底或白板尚在场
	if aliveCount < 4 && (spyAlive || blankAlive) {
		jsh.onSwitch(STAGE_FINISHED)
		return
	}

	// 平民方胜利：卧底和白板均已出局
	if !spyAlive && !blankAlive {
		jsh.onSwitch(STAGE_FINISHED)
		return
	}

	// 未分出胜负，继续下一轮
	ctx.Round++

	// 10 秒后进入下一轮 Speaking
	ctx.SetTimeout(10 * time.Second)
}

func (jsh *judgeStageHandler) OnHandle(ctx *GameContext, req RequestWrapper) error {
	// 处理超时请求
	if req := TryUnwrapTimeoutRequest(req); req != nil {
		if req.Stage == STAGE_JUDGING {
			// 超时，进入下一轮发言
			jsh.onSwitch(STAGE_SPEAKING)
			return nil
		}
	}

	// 判定阶段不处理其他任何请求
	return errors.New("判定阶段不接受玩家请求")
}

func (jsh *judgeStageHandler) OnExit(ctx *GameContext) {
	ctx.ClearTimeout()
}

func (jsh *judgeStageHandler) SetOnSwitch(onSwitch func(string)) {
	jsh.onSwitch = onSwitch
}

// 结束阶段处理器
type finishStageHandler struct {
	onSwitch func(string)
}

func NewFinishStageHandler() *finishStageHandler {
	return &finishStageHandler{}
}

func (fsh *finishStageHandler) Stage() string {
	return STAGE_FINISHED
}

func (fsh *finishStageHandler) OnEnter(ctx *GameContext) {
	// 确定胜利方
	var winner string
	spyAlive := ctx.IsSpyAlive()
	blankAlive := ctx.IsBlankAlive()

	if spyAlive || blankAlive {
		winner = "卧底方"
	} else {
		winner = "平民方"
	}

	// 收集所有玩家的身份和词语信息
	playerRoles := make(map[string]string)
	playerWords := make(map[string]string)

	for _, p := range ctx.Players {
		if p.Role != ROLE_ADMIN {
			playerRoles[p.Name] = p.Role
			playerWords[p.Name] = p.Word
		}
	}

	// 广播游戏结果
	resultResp := WrapResponse(
		RESP_GAME_RESULT,
		GameResultResponse{
			Winner:      winner,
			AnswerWord:  ctx.AnswerWord,
			SpyWord:     ctx.SpyWord,
			PlayerRoles: playerRoles,
			PlayerWords: playerWords,
		},
	)

	ctx.BroadcastResp(resultResp)
}

func (fsh *finishStageHandler) OnHandle(ctx *GameContext, req RequestWrapper) error {
	// 结束阶段不处理任何请求
	return errors.New("游戏已结束")
}

func (fsh *finishStageHandler) OnExit(ctx *GameContext) {
	// 强制确定为 FINISHED 阶段，防止出现异常状态
	ctx.GameStage = STAGE_FINISHED
}

func (fsh *finishStageHandler) SetOnSwitch(onSwitch func(string)) {
	fsh.onSwitch = onSwitch
}

func onPlayerJoin(ctx *GameContext, player Player) {
	// 一局游戏最多支持 8 个正常玩家（不包括管理员和观察者）
	const PLAYER_THRESHOLD = 8

	// 检查当前房间是否没有玩家，如果没有玩家，则加入的玩家成为管理员
	if len(ctx.Players) == 0 {
		player.Role = ROLE_ADMIN

		ctx.Players[player.ID] = &player

		// 广播玩家加入消息
		joinResp := WrapResponse(
			RESP_JOIN_GAME,
			JoinGameResponse{
				Joiner: player,
			},
		)

		ctx.BroadcastResp(joinResp)

		return
	}

	// 检查当前的游戏是否已经满员
	if len(ctx.Players) >= PLAYER_THRESHOLD+1 {
		// 超过玩家上限，默认变为观察者身份
		player.Role = ROLE_OBSERVER

		ctx.Players[player.ID] = &player

		return
	}

	// 没有超出上限，则检查是否是等待阶段
	if ctx.GameStage == STAGE_WAITING {
		// 如果是等待阶段，则玩家可以直接进入游戏
		// 第一个加入的玩家成为管理员
		if len(ctx.Players) == 0 {
			player.Role = ROLE_ADMIN
		} else {
			player.Role = ROLE_UNSET
		}

		ctx.Players[player.ID] = &player

		// 广播玩家加入消息
		joinResp := WrapResponse(
			RESP_JOIN_GAME,
			JoinGameResponse{
				Joiner: player,
			},
		)

		ctx.BroadcastResp(joinResp)

		return
	}

	// 否则，玩家只能以观察者身份加入游戏
	player.Role = ROLE_OBSERVER

	ctx.Players[player.ID] = &player

	// 广播玩家加入消息
	joinResp := WrapResponse(
		RESP_JOIN_GAME,
		JoinGameResponse{
			Joiner: player,
		},
	)

	ctx.BroadcastResp(joinResp)
}
