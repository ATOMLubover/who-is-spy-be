package game

import (
	"errors"
	"math/rand/v2"
	"time"

	"go.uber.org/zap"
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
	ctx.RoomID = GenID()[len(GenID())-8:] // Generate a short room ID
	ctx.GameStage = STAGE_WAITING
	ctx.Players = make(map[string]*Player, 0)

	ctx.Answer = ""
	ctx.WordList = make([]string, 0)
}

func (wsh *waitStageHandler) OnHandle(ctx *GameContext, req RequestWrapper) error {
	// 在等待阶段只处理 JoinGame、SetWords、StartGame 和 ExitGame 请求
	if req := TryUnwrapJoinGameRequest(req); req != nil {
		playerID := req.PlayerID
		if playerID == "" {
			playerID = GenID()[len(GenID())-8:]
		}

		player := Player{
			ID:     playerID,
			Name:   req.JoinerName,
			RespCh: req.RespCh,
		}

		// 如果客户端显式请求作为观察者，优先保留该身份
		if req.Observer {
			player.Role = ROLE_OBSERVER
		}

		onPlayerJoin(ctx, player)

		return nil
	}

	if req := TryUnwrapExitGameRequest(req); req != nil {
		onPlayerExit(ctx, req.PlayerID, req.RespCh)
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

		// 验证词库必须至少包含两个词：WordList[0] = 正常词，WordList[1] = 卧底词
		if len(req.WordList) < 2 {
			return errors.New("无法设置词库：必须提供至少两个词（索引0为正常词，索引1为卧底词）")
		}

		// 验证词语不能为空
		if req.WordList[0] == "" || req.WordList[1] == "" {
			return errors.New("无法设置词库：正常词和卧底词不能为空")
		}

		// 更新词库
		ctx.WordList = req.WordList

		// 发送通知（不广播实际的词语，只通知设置成功）
		resp := WrapResponse(
			RESP_SET_WORDS,
			SetWordsResponse{
				WordList: []string{}, // 不返回实际词语，保持游戏悬念
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

		// 检查词库是否已设置（必须至少包含两个词）
		if len(ctx.WordList) < 2 || ctx.WordList[0] == "" || ctx.WordList[1] == "" {
			return errors.New("无法开始游戏：管理员必须先设置正常词和卧底词")
		}

		// 检查玩家数量（按存活计数，排除管理员/观察者）
		if ctx.CountAlive() < 8 {
			return errors.New("无法开始游戏：玩家数量不足 8 人")
		}

		// 切换到准备阶段
		wsh.onSwitch(STAGE_PREPARING)

		return nil
	}

	return errors.New("无法处理请求：当前阶段不支持该请求类型")
}

func assignRolesAndWords(ctx *GameContext) {
	// 根据管理员设置的词库，分配角色和词语
	// WordList[0] = 正常玩家的词（谜底词）
	// WordList[1] = 卧底玩家的词
	var (
		answer  string
		spyWord string
	)

	// 使用管理员设置的确定性词语
	if len(ctx.WordList) < 2 {
		zap.L().Error("词库未正确设置，无法分配角色", zap.String("roomID", ctx.RoomID))
		// 这种情况不应该发生，因为 StartGame 已经验证过
		return
	}

	// 确定性分配：索引 0 为正常词，索引 1 为卧底词
	answer = ctx.WordList[0]
	spyWord = ctx.WordList[1]

	// 抽选一个白板，一个卧底，其次为普通玩家
	slicedPlayers := make([]*Player, 0, len(ctx.Players))
	for _, p := range ctx.Players {
		if p.Role == ROLE_UNSET {
			slicedPlayers = append(slicedPlayers, p)
		}
	}

	if len(slicedPlayers) < 2 {
		zap.L().Error("参与分配的玩家不足，无法分配角色", zap.String("roomID", ctx.RoomID))
		return
	}

	blankIndex := rand.IntN(len(slicedPlayers))
	blankPlayer := slicedPlayers[blankIndex]

	tempPlayers := append(
		slicedPlayers[:blankIndex],
		slicedPlayers[blankIndex+1:]...,
	)

	if len(tempPlayers) == 0 {
		zap.L().Error("剩余玩家为空，无法分配卧底", zap.String("roomID", ctx.RoomID))
		return
	}

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

	// 向所有玩家广播游戏开始信息（非参与者的 role 和 word 留空；管理员额外收到 players 列表）
	for _, p := range ctx.Players {
		var resp ResponseWrapper
		if p.Role != ROLE_ADMIN && p.Role != ROLE_OBSERVER {
			// 参与者：发送完整的角色和词语
			resp = WrapResponse(
				RESP_START_GAME,
				StartGameResponse{
					AssignedRole: p.Role,
					AssignedWord: p.Word,
				},
			)
		} else {
			// 管理员/观察者：role 和 word 留空
			if p.Role == ROLE_ADMIN {
				// 管理员需要拿到所有玩家信息用于单播显示
				players := make([]Player, 0, len(ctx.Players))
				for _, gp := range ctx.Players {
					// 复制值（会复制 RespCh 但该字段 json:"-"，不会被序列化）
					players = append(players, *gp)
				}

				resp = WrapResponse(
					RESP_START_GAME,
					StartGameResponse{
						AssignedRole: "",
						AssignedWord: "",
						Players:      players,
					},
				)
			} else {
				// 非管理员观察者不需要玩家列表
				resp = WrapResponse(
					RESP_START_GAME,
					StartGameResponse{
						AssignedRole: "",
						AssignedWord: "",
					},
				)
			}
		}
		ctx.UnicastResp(p.ID, resp)
	}

	// 30 秒后自动切换到发言阶段
	ctx.SetTimeout(30 * time.Second)
}

func (psh *prepStageHandler) OnHandle(ctx *GameContext, req RequestWrapper) error {
	// 允许在任何阶段接受 JoinGame 请求（作为观察者或重连）
	if jreq := TryUnwrapJoinGameRequest(req); jreq != nil {
		playerID := jreq.PlayerID
		if playerID == "" {
			playerID = GenID()[len(GenID())-8:]
		}

		player := Player{
			ID:     playerID,
			Name:   jreq.JoinerName,
			RespCh: jreq.RespCh,
		}

		if jreq.Observer {
			player.Role = ROLE_OBSERVER
		}

		onPlayerJoin(ctx, player)
		return nil
	}
	// 处理超时请求
	if req := TryUnwrapTimeoutRequest(req); req != nil {
		if req.Stage == STAGE_PREPARING {
			// 超时，切换到发言阶段
			psh.onSwitch(STAGE_SPEAKING)
			return nil
		}
	}

	// 处理退出请求
	if req := TryUnwrapExitGameRequest(req); req != nil {
		onPlayerExit(ctx, req.PlayerID, req.RespCh)
		return nil
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

	// 如果第一个是白板（blank），将其移动到 index=4 以避免白板成为首位发言者
	if len(ctx.SpeakingOrder) > 1 {
		firstID := ctx.SpeakingOrder[0]
		if p, ok := ctx.Players[firstID]; ok && p.Role == ROLE_BLANK {
			blankID := firstID
			// 准备除去首位后的剩余顺序副本
			rest := append([]string{}, ctx.SpeakingOrder[1:]...)
			// 目标插入索引（不超过最后一位）
			insertIdx := 4
			if insertIdx >= len(ctx.SpeakingOrder) {
				insertIdx = len(ctx.SpeakingOrder) - 1
			}
			newOrder := make([]string, 0, len(ctx.SpeakingOrder))
			newOrder = append(newOrder, rest[:insertIdx]...)
			newOrder = append(newOrder, blankID)
			newOrder = append(newOrder, rest[insertIdx:]...)
			ctx.SpeakingOrder = newOrder
		}
	}

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

	// 设置 40 秒超时
	ctx.SetTimeout(40 * time.Second)
}

func (ssh *speakStageHandler) OnHandle(ctx *GameContext, req RequestWrapper) error {
	// 允许在任何阶段接受 JoinGame 请求（作为观察者或重连）
	if jreq := TryUnwrapJoinGameRequest(req); jreq != nil {
		playerID := jreq.PlayerID
		if playerID == "" {
			playerID = GenID()[len(GenID())-8:]
		}

		player := Player{
			ID:     playerID,
			Name:   jreq.JoinerName,
			RespCh: jreq.RespCh,
		}

		if jreq.Observer {
			player.Role = ROLE_OBSERVER
		}

		onPlayerJoin(ctx, player)
		return nil
	}
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

	// 处理退出请求
	if req := TryUnwrapExitGameRequest(req); req != nil {
		onPlayerExit(ctx, req.PlayerID, req.RespCh)
		return nil
	}

	return errors.New("发言阶段只接受 Describe 和 ExitGame 请求")
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
	// 允许在任何阶段接受 JoinGame 请求（作为观察者或重连）
	if jreq := TryUnwrapJoinGameRequest(req); jreq != nil {
		playerID := jreq.PlayerID
		if playerID == "" {
			playerID = GenID()[len(GenID())-8:]
		}

		player := Player{
			ID:     playerID,
			Name:   jreq.JoinerName,
			RespCh: jreq.RespCh,
		}

		if jreq.Observer {
			player.Role = ROLE_OBSERVER
		}

		onPlayerJoin(ctx, player)
		return nil
	}
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
		if _, alreadyVoted := ctx.Votes[req.VoterID]; alreadyVoted {
			return errors.New("你已投票，不能重复投票")
		}
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

	// 处理退出请求
	if req := TryUnwrapExitGameRequest(req); req != nil {
		onPlayerExit(ctx, req.PlayerID, req.RespCh)
		return nil
	}

	return errors.New("投票阶段只接受 Vote 和 ExitGame 请求")
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

	// 如果没有玩家被淘汰（无投票或票数相同），跳过淘汰逻辑
	if eliminatedID == "" {
		zap.L().Info("裁判阶段：无玩家被淘汰", zap.String("roomID", ctx.RoomID))
		// 直接进行胜利条件判断
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

		// 四轮上限：先检查胜负再执行轮数限制
		if ctx.Round > 4 {
			jsh.onSwitch(STAGE_FINISHED)
			return
		}

		ctx.CurrentSpeakerIdx = 0
		jsh.onSwitch(STAGE_SPEAKING)
		return
	}

	// 淘汰该玩家
	eliminated := ctx.Players[eliminatedID]
	if eliminated == nil {
		zap.L().Error("裁判阶段：被淘汰玩家不存在", zap.String("roomID", ctx.RoomID), zap.String("eliminatedID", eliminatedID))
		return
	}

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

	// 四轮上限：先检查胜负再执行轮数限制
	if ctx.Round > 4 {
		jsh.onSwitch(STAGE_FINISHED)
		return
	}

	// 10 秒后进入下一轮 Speaking
	ctx.SetTimeout(10 * time.Second)
}

func (jsh *judgeStageHandler) OnHandle(ctx *GameContext, req RequestWrapper) error {
	// 允许在任何阶段接受 JoinGame 请求（作为观察者或重连）
	if jreq := TryUnwrapJoinGameRequest(req); jreq != nil {
		playerID := jreq.PlayerID
		if playerID == "" {
			playerID = GenID()[len(GenID())-8:]
		}

		player := Player{
			ID:     playerID,
			Name:   jreq.JoinerName,
			RespCh: jreq.RespCh,
		}

		if jreq.Observer {
			player.Role = ROLE_OBSERVER
		}

		onPlayerJoin(ctx, player)
		return nil
	}
	// 处理超时请求
	if req := TryUnwrapTimeoutRequest(req); req != nil {
		if req.Stage == STAGE_JUDGING {
			// 超时，进入下一轮发言
			jsh.onSwitch(STAGE_SPEAKING)
			return nil
		}
	}
	// 处理退出请求
	if req := TryUnwrapExitGameRequest(req); req != nil {
		onPlayerExit(ctx, req.PlayerID, req.RespCh)
		return nil
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
	// 允许在任何阶段接受 JoinGame 请求（作为观察者或重连）
	if jreq := TryUnwrapJoinGameRequest(req); jreq != nil {
		playerID := jreq.PlayerID
		if playerID == "" {
			playerID = GenID()[len(GenID())-8:]
		}

		player := Player{
			ID:     playerID,
			Name:   jreq.JoinerName,
			RespCh: jreq.RespCh,
		}

		onPlayerJoin(ctx, player)
		return nil
	}
	// 处理退出请求
	if req := TryUnwrapExitGameRequest(req); req != nil {
		onPlayerExit(ctx, req.PlayerID, req.RespCh)
		return nil
	}

	// 结束阶段不处理其他任何请求
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

	// 如果存在相同的玩家 ID，则视为按 ID 重连：替换 RespCh 并发送快照
	if existingPlayer, exists := ctx.Players[player.ID]; exists {
		zap.L().Info(
			"检测到相同 player ID，执行按 ID 重连",
			zap.String("player_id", player.ID),
			zap.String("player_name", player.Name),
		)

		// 关闭旧连接的响应通道，让旧的写协程退出
		if existingPlayer.RespCh != nil {
			close(existingPlayer.RespCh)
			zap.L().Debug(
				"已关闭旧连接的响应通道（按 ID 重连）",
				zap.String("player_id", player.ID),
			)
		}

		// 更新为新连接的响应通道，保留原有的玩家ID和角色等信息
		existingPlayer.RespCh = player.RespCh

		// 1. 先给重连者私发完整信息（包含自己的 word 和 role）
		privateResp := WrapResponse(
			RESP_JOIN_GAME,
			JoinGameResponse{
				RoomID:   ctx.RoomID,
				Stage:    ctx.GameStage,
				Joiner:   *existingPlayer, // 完整信息
				Players:  buildPublicPlayersList(ctx),
				MasterID: ctx.GetAdmin().ID,
			},
		)

		select {
		case existingPlayer.RespCh <- privateResp:
			zap.L().Debug(
				"成功发送按 ID 重连者私有快照",
				zap.String("player_id", player.ID),
			)
		default:
			zap.L().Warn("发送按 ID 重连者私有快照失败：通道已满")
		}

		// 2. 广播给所有人公开版本（隐藏重连者的敏感信息）
		publicJoiner := sanitizePlayer(existingPlayer)
		publicBroadcast := WrapResponse(
			RESP_JOIN_GAME,
			JoinGameResponse{
				RoomID:   ctx.RoomID,
				Stage:    ctx.GameStage,
				Joiner:   publicJoiner, // 清理后的公开信息
				Players:  buildPublicPlayersList(ctx),
				MasterID: ctx.GetAdmin().ID,
			},
		)

		ctx.BroadcastResp(publicBroadcast)

		zap.L().Info(
			"按 ID 断线重连成功",
			zap.String("player_id", player.ID),
			zap.String("player_name", player.Name),
		)

		return
	}

	// 检查是否存在相同的玩家 Name（断线重连场景）
	for existingID, existingPlayer := range ctx.Players {
		if existingPlayer.Name == player.Name {
			zap.L().Info(
				"检测到同名玩家，执行断线重连",
				zap.String("player_id", existingID),
				zap.String("player_name", player.Name),
			)

			// 关闭旧连接的响应通道，让旧的写协程退出
			if existingPlayer.RespCh != nil {
				close(existingPlayer.RespCh)
				zap.L().Debug(
					"已关闭旧连接的响应通道",
					zap.String("player_id", existingID),
				)
			}

			// 更新为新连接的响应通道，保留原有的玩家ID和角色等信息
			existingPlayer.RespCh = player.RespCh

			// 1. 先给重连者私发完整信息（包含自己的 word 和 role）
			privateResp := WrapResponse(
				RESP_JOIN_GAME,
				JoinGameResponse{
					RoomID:   ctx.RoomID,
					Stage:    ctx.GameStage,
					Joiner:   *existingPlayer, // 完整信息
					Players:  buildPublicPlayersList(ctx),
					MasterID: ctx.GetAdmin().ID,
				},
			)

			select {
			case existingPlayer.RespCh <- privateResp:
				zap.L().Debug(
					"成功发送重连者私有快照",
					zap.String("player_id", existingID),
				)
			default:
				zap.L().Warn("发送重连者私有快照失败：通道已满")
			}

			// 2. 广播给所有人公开版本（隐藏重连者的敏感信息）
			publicJoiner := sanitizePlayer(existingPlayer)
			publicBroadcast := WrapResponse(
				RESP_JOIN_GAME,
				JoinGameResponse{
					RoomID:   ctx.RoomID,
					Stage:    ctx.GameStage,
					Joiner:   publicJoiner, // 清理后的公开信息
					Players:  buildPublicPlayersList(ctx),
					MasterID: ctx.GetAdmin().ID,
				},
			)

			ctx.BroadcastResp(publicBroadcast)

			zap.L().Info(
				"断线重连成功",
				zap.String("player_id", existingID),
				zap.String("player_name", player.Name),
			)

			return
		}
	}

	// 检查当前房间是否没有玩家，如果没有玩家，则加入的玩家成为管理员
	if len(ctx.Players) == 0 {
		player.Role = ROLE_ADMIN

		ctx.Players[player.ID] = &player

		// 广播玩家加入消息（首位玩家，包含完整房间状态）
		joinResp := WrapResponse(
			RESP_JOIN_GAME,
			JoinGameResponse{
				RoomID:   ctx.RoomID,
				Stage:    ctx.GameStage,
				Joiner:   player,
				Players:  buildPublicPlayersList(ctx),
				MasterID: player.ID,
			},
		)

		ctx.BroadcastResp(joinResp)

		return
	}

	// 检查当前的游戏是否已经满员（按存活玩家计数，不含管理员/观察者）
	if ctx.CountAlive() >= PLAYER_THRESHOLD {
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
			// 如果客户端显式请求观察者，优先保留；否则成为未设置的普通玩家
			if player.Role != ROLE_OBSERVER {
				player.Role = ROLE_UNSET
			}
		}

		ctx.Players[player.ID] = &player

		// 广播玩家加入消息（等待阶段，包含完整房间状态）
		joinResp := WrapResponse(
			RESP_JOIN_GAME,
			JoinGameResponse{
				RoomID:   ctx.RoomID,
				Stage:    ctx.GameStage,
				Joiner:   player,
				Players:  buildPublicPlayersList(ctx),
				MasterID: ctx.GetAdmin().ID,
			},
		)

		ctx.BroadcastResp(joinResp)

		return
	}

	// 否则，玩家只能以观察者身份加入游戏
	player.Role = ROLE_OBSERVER

	ctx.Players[player.ID] = &player

	// 广播玩家加入消息（观察者加入，包含完整房间状态）
	joinResp := WrapResponse(
		RESP_JOIN_GAME,
		JoinGameResponse{
			RoomID:   ctx.RoomID,
			Stage:    ctx.GameStage,
			Joiner:   player,
			Players:  buildPublicPlayersList(ctx),
			MasterID: ctx.GetAdmin().ID,
		},
	)

	ctx.BroadcastResp(joinResp)
}

func onPlayerExit(ctx *GameContext, playerID string, reqRespCh chan ResponseWrapper) {
	player, exists := ctx.Players[playerID]
	if !exists {
		zap.L().Warn(
			"玩家不存在，无法退出",
			zap.String("player_id", playerID),
		)
		return
	}

	playerName := player.Name

	// 检查RespCh是否匹配，不匹配说明已经被顶替重连
	if player.RespCh != reqRespCh {
		zap.L().Info(
			"检测到旧连接退出（已被顶替），只关闭旧通道不删除玩家",
			zap.String("player_id", playerID),
			zap.String("player_name", playerName),
		)

		// 发送退出确认给旧连接
		exitResp := WrapResponse(
			RESP_EXIT_GAME,
			ExitGameResponse{
				LeftPlayerID:   playerID,
				LeftPlayerName: playerName,
			},
		)

		select {
		case reqRespCh <- exitResp:
			zap.L().Debug("发送退出确认给旧连接成功")
		default:
			zap.L().Debug("旧连接通道已满或关闭")
		}

		// 旧连接的通道可能已经被顶替逻辑关闭了，这里安全检查
		// 不关闭 reqRespCh，因为可能已经被关闭了（会 panic）

		return
	}

	// RespCh 匹配，这是正常的退出流程
	// 先发送退出确认响应给该玩家
	exitResp := WrapResponse(
		RESP_EXIT_GAME,
		ExitGameResponse{
			LeftPlayerID:   playerID,
			LeftPlayerName: playerName,
		},
	)

	select {
	case player.RespCh <- exitResp:
		zap.L().Debug(
			"发送退出确认响应成功",
			zap.String("player_id", playerID),
		)
	default:
		zap.L().Warn(
			"发送退出确认响应失败：响应通道已满",
			zap.String("player_id", playerID),
		)
	}

	// 关闭该玩家的响应通道，通知写协程退出
	close(player.RespCh)

	// 将玩家标记为观察者以保留信息，防止误删导致状态不一致
	player.Role = ROLE_OBSERVER
	player.Word = ""

	zap.L().Info(
		"玩家已退出游戏（标记为观察者）",
		zap.String("player_id", playerID),
		zap.String("player_name", playerName),
	)

	// 向其他玩家广播离开消息
	leftNotif := WrapResponse(
		RESP_EXIT_GAME,
		ExitGameResponse{
			LeftPlayerID:   playerID,
			LeftPlayerName: playerName,
		},
	)

	ctx.BroadcastResp(leftNotif)
}
