package game

import (
	"encoding/json"

	"go.uber.org/zap"
)

// 请求类型
const (
	REQ_JOIN_GAME  = "JoinGame"
	REQ_SET_WORDS  = "SetWords"
	REQ_START_GAME = "StartGame"
	REQ_DESCRIBE   = "Describe"
	REQ_VOTE       = "Vote"
	REQ_TIMEOUT    = "Timeout"
)

type RequestWrapper struct {
	ReqType string          `json:"request_type"`
	Data    json.RawMessage `json:"data"`
}

func TryUnwrapJoinGameRequest(wrapper RequestWrapper) *JoinGameRequest {
	if wrapper.ReqType != REQ_JOIN_GAME {
		return nil
	}

	var joinGameRequest JoinGameRequest

	err := json.Unmarshal(wrapper.Data, &joinGameRequest)
	if err != nil {
		zap.L().Error(
			"Failed to unwrap JoinGameRequest",
			zap.Error(err),
			zap.Any("wrapper", wrapper),
		)
		return nil
	}

	return &joinGameRequest
}

func TryUnwrapSetWordsRequest(wrapper RequestWrapper) *SetWordsRequest {
	if wrapper.ReqType != REQ_SET_WORDS {
		return nil
	}

	var setWordsRequest SetWordsRequest

	err := json.Unmarshal(wrapper.Data, &setWordsRequest)
	if err != nil {
		zap.L().Error(
			"Failed to unwrap SetWordsRequest",
			zap.Error(err),
			zap.Any("wrapper", wrapper),
		)
		return nil
	}

	return &setWordsRequest
}

func TryUnwrapStartGameRequest(wrapper RequestWrapper) *StartGameRequest {
	if wrapper.ReqType != REQ_START_GAME {
		return nil
	}

	var startGameRequest StartGameRequest

	err := json.Unmarshal(wrapper.Data, &startGameRequest)
	if err != nil {
		zap.L().Error(
			"Failed to unwrap StartGameRequest",
			zap.Error(err),
			zap.Any("wrapper", wrapper),
		)
		return nil
	}

	return &startGameRequest
}

func TryUnwrapDescribeRequest(wrapper RequestWrapper) *DescribeRequest {
	if wrapper.ReqType != REQ_DESCRIBE {
		return nil
	}

	var describeRequest DescribeRequest

	err := json.Unmarshal(wrapper.Data, &describeRequest)
	if err != nil {
		zap.L().Error(
			"Failed to unwrap DescribeRequest",
			zap.Error(err),
			zap.Any("wrapper", wrapper),
		)
		return nil
	}

	return &describeRequest
}

func TryUnwrapVoteRequest(wrapper RequestWrapper) *VoteRequest {
	if wrapper.ReqType != REQ_VOTE {
		return nil
	}

	var voteRequest VoteRequest

	err := json.Unmarshal(wrapper.Data, &voteRequest)
	if err != nil {
		zap.L().Error(
			"Failed to unwrap VoteRequest",
			zap.Error(err),
			zap.Any("wrapper", wrapper),
		)
		return nil
	}

	return &voteRequest
}

func TryUnwrapTimeoutRequest(wrapper RequestWrapper) *TimeoutRequest {
	if wrapper.ReqType != REQ_TIMEOUT {
		return nil
	}

	var timeoutRequest TimeoutRequest

	err := json.Unmarshal(wrapper.Data, &timeoutRequest)
	if err != nil {
		zap.L().Error(
			"Failed to unwrap TimeoutRequest",
			zap.Error(err),
			zap.Any("wrapper", wrapper),
		)
		return nil
	}

	return &timeoutRequest
}

// 响应类型
const (
	RESP_ERROR = "Error"

	RESP_JOIN_GAME   = "JoinGame"
	RESP_SET_WORDS   = "SetWords"
	RESP_START_GAME  = "StartGame"
	RESP_DESCRIBE    = "Describe"
	RESP_VOTE        = "Vote"
	RESP_GAME_STATE  = "GameState"
	RESP_ELIMINATE   = "Eliminate"
	RESP_GAME_RESULT = "GameResult"
)

type ResponseWrapper struct {
	RespType string `json:"response_type"`
	Data     any    `json:"data"`
	ErrMsg   string `json:"error_message,omitempty"`
}

func WrapResponse(respType string, data any) ResponseWrapper {
	return ResponseWrapper{
		RespType: respType,
		Data:     data,
	}
}

func WrapErrResponse(errMsg string) ResponseWrapper {
	return ResponseWrapper{
		RespType: RESP_ERROR,
		ErrMsg:   errMsg,
	}
}
