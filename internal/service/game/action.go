package game

type CreateRoomRequest struct {
	RoomName    string `json:"room_name"`
	CreatorName string `json:"creator_name"`
}

type CreateRoomResponse struct {
	RoomID string `json:"room_id"`
}

type JoinGameRequest struct {
	RoomID     string `json:"room_id"`
	JoinerName string `json:"joiner_name"`
	// Optional client-supplied player ID for reconnecting
	PlayerID string `json:"player_id,omitempty"`
	// Optional explicit observer intent from client
	Observer bool                 `json:"observer,omitempty"`
	RespCh   chan ResponseWrapper `json:"-"`
}

type JoinGameResponse struct {
	RoomID   string   `json:"room_id"`
	Stage    string   `json:"stage"`
	Joiner   Player   `json:"joiner"`
	Players  []Player `json:"players"`
	MasterID string   `json:"master_id"`
}

type SetWordsRequest struct {
	SetPlayerID string   `json:"set_player_id"`
	WordList    []string `json:"word_list"`
}

type SetWordsResponse struct {
	// 为保持游戏悬念，服务器不返回实际词语，只返回空数组表示设置成功
	WordList []string `json:"word_list"`
}

type StartGameRequest struct {
	StartPlayerID string `json:"start_player_id"`
}

type StartGameResponse struct {
	AssignedRole string   `json:"assigned_role"`
	AssignedWord string   `json:"assigned_word"`
	Players      []Player `json:"players,omitempty"`
}

type DescribeRequest struct {
	ReqPlayerID string `json:"req_player_id"`
	Message     string `json:"message"`
}

type DescribeResponse struct {
	SpeakerID   string `json:"speaker_id"`
	SpeakerName string `json:"speaker_name"`
	Message     string `json:"message"`
}

type VoteRequest struct {
	VoterID  string `json:"voter_id"`
	TargetID string `json:"target_id"`
}

type VoteResponse struct {
	VoterID    string `json:"voter_id"`
	VoterName  string `json:"voter_name"`
	TargetID   string `json:"target_id"`
	TargetName string `json:"target_name"`
}

type GameStateNotification struct {
	Stage           string `json:"stage"`
	CurrentTurnID   string `json:"current_turn_id,omitempty"`
	CurrentTurnName string `json:"current_turn_name,omitempty"`
	Round           int    `json:"round"`
}

type EliminateNotification struct {
	EliminatedID   string `json:"eliminated_id"`
	EliminatedName string `json:"eliminated_name"`
	EliminatedWord string `json:"eliminated_word"`
}

type GameResultResponse struct {
	Winner      string            `json:"winner"`
	AnswerWord  string            `json:"answer_word"`
	SpyWord     string            `json:"spy_word"`
	PlayerRoles map[string]string `json:"player_roles"`
	PlayerWords map[string]string `json:"player_words"`
}

type TimeoutRequest struct {
	Stage string `json:"stage"`
}

type ExitGameRequest struct {
	PlayerID string               `json:"player_id"`
	RespCh   chan ResponseWrapper `json:"-"`
}

type ExitGameResponse struct {
	LeftPlayerID   string `json:"left_player_id"`
	LeftPlayerName string `json:"left_player_name"`
}
