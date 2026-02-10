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

// 响应类型
const (
	RESP_JOIN_GAME  = "JoinGame"
	RESP_SET_WORDS  = "SetWords"
	RESP_START_GAME = "StartGame"
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
		ErrMsg: errMsg,
	}
}
