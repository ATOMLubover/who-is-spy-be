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
	return Player{
		ID:     p.ID,
		Name:   p.Name,
		Role:   p.Role,
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
