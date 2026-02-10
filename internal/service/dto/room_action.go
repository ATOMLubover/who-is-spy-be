package dto

import "encoding/json"

type UpdateWordsRequest struct {
	Words []string `json:"words"`
}

type UpdateWordsResponse struct {
	Words []string `json:"words"`
}

type PreparingStartResponse struct {
	// 被分配到的词语，可能是空字符串（非 normal 和 spy 玩家）
	AssignedWord string `json:"assigned_word"`
}

// 需要注意的是，这个请求是只有当前玩家允许说话时才会发送的
type PlayerSpeakRequest struct {
	Message string `json:"message"`
}

type PlayerSpeakResponse struct {
	PlayerID string `json:"player_id"`
	Message  string `json:"message"`
}

type PlayerOnTurnResponse struct {
	PlayerID string `json:"player_id"`
}

type VoteRequest struct {
	VotedPlayerID string `json:"voted_player_id"`
}

type UpdateVotingResponse struct {
	// key: player_id, value: vote_count
	// 注意，为 0 的不会出现在这个 map 里
	Votes map[string]int `json:"votes"`
}

const (
	RESULT_NORMAL_WIN = "NormalWin"
	RESULT_SPY_WIN    = "SpyWin"
)

type JudgingStartResponse struct {
	EliminatedPlayerID string         `json:"eliminated_player_id"`
	VoteCounts         map[string]int `json:"vote_counts"`
	Word               string         `json:"word"`
	Result             string         `json:"result"`
}

// 客户端发送的房间操作
const (
	REQ_ACTION_UPDATE_WORDS = "UpdateWords"
	REQ_ACTION_VOTE         = "Vote"
	REQ_ACTION_PLAYER_SPEAK = "PlayerSpeak"
)

type RoomActionRequest[T any] struct {
	RequestType string `json:"request_type"`
	Data        T      `json:"data"`
}

// 服务器发送的房间操作
const (
	RES_ACTION_PREPARING_START = "PreparingStart"
	RES_ACTION_PLAYER_ON_TURN  = "PlayerOnTurn"
	RES_ACTION_PLAYER_SPEAK    = "PlayerSpeak"
	RES_ACTION_VOTING_START    = "VotingStart"
	RES_ACTION_JUDGING_START   = "JudgingStart"
)

type RawRoomActionResponse struct {
	ResponseType string          `json:"response_type"`
	Data         json.RawMessage `json:"data"`
}
