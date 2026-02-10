package state

import (
	"who-is-spy-be/internal/config"
	"who-is-spy-be/internal/service"
)

type AppState struct {
	Cfg     *config.AppConfig
	RoomSvc *service.RoomService
}

func NewAppState(
	cfg *config.AppConfig,
	roomSvc *service.RoomService,
) *AppState {
	return &AppState{
		Cfg:     cfg,
		RoomSvc: roomSvc,
	}
}
