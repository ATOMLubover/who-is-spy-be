package game

import (
	"time"

	"go.uber.org/zap"
)

type GameContext struct {
	RoomID     string
	GameStatus string
	Players    map[string]*Player

	Answer     string
	Undercover string
	WordList   []string

	Timer *time.Timer
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
		select {
		case p.RespCh <- resp:
			zap.L().Debug(
				"成功发送广播响应",
				zap.String("player_id", p.ID),
				zap.Any("response", resp),
			)
		default:
			zap.L().Warn(
				"发送广播响应失败：玩家响应通道已满",
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
