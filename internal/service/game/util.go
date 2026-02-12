package game

import (
	"encoding/json"

	"github.com/google/uuid"
)

func GenID() string {
	id, err := uuid.NewV7()
	if err != nil {
		panic("Failed to generate UUID: " + err.Error())
	}

	// Return the last 8 characters of the UUID
	return id.String()[len(id.String())-8:]
}

func mustMarshal(v any) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		panic("Failed to marshal: " + err.Error())
	}

	return data
}

// sanitizePlayer 创建玩家的公开视图副本，清除敏感信息（Word）
// 注意：Role 字段保留，用于显示身份徽章（如 Admin/Observer）
func sanitizePlayer(p *Player) Player {
	// 将内部的 Ob* 角色对外统一显示为 Observer
	role := toPublicRole(p.Role)

	return Player{
		ID:     p.ID,
		Name:   p.Name,
		Role:   role,
		Word:   "", // 清空敏感字段
		RespCh: nil,
	}
}

// buildPublicPlayersList 构建所有玩家的公开列表（不含敏感信息）
func buildPublicPlayersList(ctx *GameContext) []Player {
	players := make([]Player, 0, len(ctx.Players))
	for _, p := range ctx.Players {
		players = append(players, sanitizePlayer(p))
	}
	return players
}

// toPublicRole 将内部 Ob* 角色统一映射为 Observer，用于对外广播的状态同步
func toPublicRole(role string) string {
	switch role {
	case ROLE_OB_NORMAL, ROLE_OB_SPY, ROLE_OB_BLANK:
		return ROLE_OBSERVER
	default:
		return role
	}
}

// toOriginalRole 将内部 Ob* 角色还原为被淘汰前的身份，用于 GameResult
func toOriginalRole(role string) string {
	switch role {
	case ROLE_OB_NORMAL:
		return ROLE_NORMAL
	case ROLE_OB_SPY:
		return ROLE_SPY
	case ROLE_OB_BLANK:
		return ROLE_BLANK
	default:
		return role
	}
}

// isObserverLike 标记所有观战/淘汰态的角色
func isObserverLike(role string) bool {
	switch role {
	case ROLE_OBSERVER, ROLE_OB_NORMAL, ROLE_OB_SPY, ROLE_OB_BLANK:
		return true
	default:
		return false
	}
}
