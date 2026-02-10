package http

import (
	"who-is-spy-be/internal/service/dto"
	"who-is-spy-be/internal/state"

	"github.com/kataras/iris/v12"
)

func CreateRoom(appState *state.AppState) iris.Handler {
	return func(ctx iris.Context) {
		var req dto.CreateRoomRequest

		if err := ctx.ReadJSON(&req); err != nil {
			ctx.StatusCode(iris.StatusBadRequest)
			ctx.JSON(iris.Map{
				"error": "请求参数无效",
			})
			return
		}

		resp, err := appState.RoomSvc.CreateRoom(req)
		if err != nil {
			ctx.StatusCode(iris.StatusBadRequest)
			ctx.JSON(iris.Map{
				"error": err.Error(),
			})
			return
		}

		ctx.JSON(resp)
	}
}
