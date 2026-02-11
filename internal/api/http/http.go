package http

import (
	"fmt"

	"who-is-spy-be/internal/api/http/websocket"
	"who-is-spy-be/internal/state"

	"github.com/kataras/iris/v12"
)

func RunServer(appState *state.AppState) {
	app := iris.Default()

	app.HandleDir(
		"/",
		iris.Dir("./who-is-spy-fe"),
		iris.DirOptions{
			IndexName: "index.html",
			SPA:       true,
			Compress:  true,
		},
	)

	api := app.Party("/api/v1")

	api.Post("/rooms/create", CreateRoom(appState))

	api.Get("/ws/join", websocket.JoinGame(appState))

	addr := fmt.Sprintf(
		"%s:%d",
		appState.Cfg.Host,
		appState.Cfg.Port,
	)

	app.Listen(addr)
}
