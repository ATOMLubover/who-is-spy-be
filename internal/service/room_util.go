package service

import "undercover-be/internal/service/dto"

type RoomRequestAction struct {
	JoinRoomReq    *dto.JoinRoomRequest
	UpdateWordsReq *dto.UpdateWordsRequest
	VoteReq        *dto.VoteRequest
	PlayerSpeakReq *dto.PlayerSpeakRequest
	Done           *struct{}
}

type joinRoomResponseWrapper struct {
	JoinRoomResp dto.JoinRoomResponse
	Err          error
}

func isRoomValid(room *dto.Room) bool {
	if room == nil {
		return false
	}

	if len(room.JoinedPlayers) <= 0 {
		return false
	}

	return true
}
