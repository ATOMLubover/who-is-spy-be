package game

import (
	"time"

	"go.uber.org/zap"
)

type GameContext struct {
	RoomID    string
	GameStage string
	Players   map[string]*Player

	Answer     string
	SpyWord    string
	AnswerWord string
	WordList   []string

	Round             int
	SpeakingOrder     []string
	CurrentSpeakerIdx int
	Votes             map[string]string

	Timer *time.Timer
	TmoCh chan RequestWrapper
}

func (gc *GameContext) GetAdmin() *Player {
	for _, p := range gc.Players {
		if p.Role == ROLE_ADMIN {
			return p
		}
	}

	return nil
}

func (gc *GameContext) BroadcastResp(resp ResponseWrapper) {
	for _, p := range gc.Players {
		// skip players without a response channel (disconnected / cleaned-up)
		if p.RespCh == nil {
			continue
		}

		select {
		case p.RespCh <- resp:
			if resp.RespType == RESP_GAME_RESULT {
				zap.L().Info(
					"广播 GameResult 成功",
					zap.String("player_id", p.ID),
				)
			} else {
				zap.L().Debug(
					"成功发送广播响应",
					zap.String("player_id", p.ID),
					zap.Any("response", resp),
				)
			}
		default:
			zap.L().Warn(
				"发送广播响应失败：玩家响应通道已满",
				zap.String("player_id", p.ID),
			)
		}
	}
}

func (gc *GameContext) UnicastResp(playerID string, resp ResponseWrapper) {
	player, ok := gc.Players[playerID]
	if !ok {
		zap.L().Warn(
			"无法找到玩家进行单播响应",
			zap.String("player_id", playerID),
		)
	}

	select {
	case player.RespCh <- resp:
		zap.L().Debug(
			"发送单播响应成功",
			zap.String("player_id", playerID),
			zap.Any("response", resp),
		)
	default:
		zap.L().Warn(
			"发送单播响应失败：玩家响应通道已满",
			zap.String("player_id", playerID),
		)
	}
}

func (gc *GameContext) GetAlivePlayers() []*Player {
	alivePlayers := make([]*Player, 0)
	for _, p := range gc.Players {
		if !isObserverLike(p.Role) && p.Role != ROLE_ADMIN {
			alivePlayers = append(alivePlayers, p)
		}
	}

	return alivePlayers
}

func (gc *GameContext) CountAlive() int {
	count := 0
	for _, p := range gc.Players {
		if !isObserverLike(p.Role) && p.Role != ROLE_ADMIN {
			count++
		}
	}

	return count
}

func (gc *GameContext) IsSpyAlive() bool {
	for _, p := range gc.Players {
		if p.Role == ROLE_SPY {
			return true
		}
	}

	return false
}

func (gc *GameContext) IsBlankAlive() bool {
	for _, p := range gc.Players {
		if p.Role == ROLE_BLANK {
			return true
		}
	}

	return false
}

func (gc *GameContext) SetTimeout(duration time.Duration) {
	// 清除之前的定时器
	gc.ClearTimeout()

	// 创建新的定时器
	gc.Timer = time.AfterFunc(duration, func() {
		// 构造超时请求
		timeoutReq := TimeoutRequest{
			Stage: gc.GameStage,
		}

		// 将超时请求包装并发送到请求通道
		wrapper := RequestWrapper{
			ReqType: REQ_TIMEOUT,
			Data:    mustMarshal(timeoutReq),
		}

		select {
		case gc.TmoCh <- wrapper:
			zap.L().Debug(
				"超时事件已发送",
				zap.String("stage", gc.GameStage),
			)
		default:
			zap.L().Warn(
				"超时事件发送失败：请求通道已满",
				zap.String("stage", gc.GameStage),
			)
		}
	})
}

func (gc *GameContext) ClearTimeout() {
	if gc.Timer != nil {
		gc.Timer.Stop()
		gc.Timer = nil
	}
}
