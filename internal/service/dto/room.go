package dto

const (
	STAGE_WAITING   = "Waiting"
	STAGE_PREPARING = "Preparing"
	STAGE_SPEAKING  = "Speaking"
	STAGE_VOTING    = "Voting"
	STAGE_JUDGING   = "Judging"
	// STAGE_FINISHED  = "Finished"
)

type Room struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	AdminID          string   `json:"admin_id"`
	Words            []string `json:"words"`
	JoinedPlayers    []Player `json:"joined_players"`
	SpeakingPlayerID string   `json:"speaking_player_id,omitempty"`
}

type CreateRoomRequest struct {
	RoomName string `json:"room_name"`
}

type CreateRoomResponse struct {
	RoomID string `json:"room_id"`
}

// 加入是一种很特别的请求，因为无论在比赛的什么阶段
// 都允许玩家加入，所以它的处理逻辑和其他请求不太一样
// 虽然是 HTTP 请求对应的内容，但是必须要在 WS 中
// 模拟进行处理，才能保证在任何阶段都能正确处理
type JoinRoomRequest struct {
	RoomID     string `json:"room_id"`
	JoinerName string `json:"joiner_name"`
}

type JoinRoomResponse struct {
	Joiner Player `json:"joiner"`
}
