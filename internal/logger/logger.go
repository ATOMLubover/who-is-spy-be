package logger

import (
	"fmt"

	"go.uber.org/zap"
)

func InitLogger(logLevel string) {
	cfg := zap.NewDevelopmentConfig()

	switch logLevel {
	case "debug":
		cfg.Level.SetLevel(zap.DebugLevel)
	case "info":
		cfg.Level.SetLevel(zap.InfoLevel)
	case "warn":
		cfg.Level.SetLevel(zap.WarnLevel)
	case "error":
		cfg.Level.SetLevel(zap.ErrorLevel)
	default:
		cfg.Level.SetLevel(zap.InfoLevel)
	}

	lgr, err := cfg.Build()
	if err != nil {
		panic(fmt.Errorf("构建日志器失败: %w", err))
	}

	zap.ReplaceGlobals(lgr)
}
