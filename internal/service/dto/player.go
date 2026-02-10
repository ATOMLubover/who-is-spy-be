package dto


// 谁是卧底游戏中的玩家信息，在进入房间后有效
type Player struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Role string `json:"role"`
	// 可空，仅在 normal 和 spy 角色时有值
	Word string `json:"word,omitempty"`
}
