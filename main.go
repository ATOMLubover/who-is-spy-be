package main

import (
	"who-is-spy-be/internal/api/http"
	"who-is-spy-be/internal/config"
	"who-is-spy-be/internal/logger"
	"who-is-spy-be/internal/service"
	"who-is-spy-be/internal/state"
)

func main() {
	// 加载配置
	cfg := config.InitConfig()

	// 初始化日志器
	logger.InitLogger(cfg.LogLevel)
    
    // 组装应用状态
    appState := state.NewAppState(
        cfg,
        service.NewRoomService(),
    )
    
    // 启动服务器
    http.RunServer(appState)
}
