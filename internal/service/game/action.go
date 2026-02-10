package game

type JoinGameRequest struct {
	RoomID     string               `json:"room_id"`
	JoinerName string               `json:"joiner_name"`
	RespCh     chan ResponseWrapper `json:"-"`
}

type JoinGameResponse struct {
	Joiner Player `json:"joiner"`
}

type SetWordsRequest struct {
	SetPlayerID string   `json:"set_player_id"`
	WordList    []string `json:"word_list"`
}

type SetWordsResponse struct {
	WordList []string `json:"word_list"`
}

type StartGameRequest struct {
	StartPlayerID string `json:"start_player_id"`
}

type StartGameResponse struct {
	AssignedRole string `json:"assigned_role"`
	AssignedWord string `json:"assigned_word"`
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
