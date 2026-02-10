package game

type JoinGameRequest struct {
	JoinerName string               `json:"joiner_name"`
	ReqCh      chan RequestWrapper  `json:"-"`
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
