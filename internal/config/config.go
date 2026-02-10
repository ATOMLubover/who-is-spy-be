package config

import (
	"fmt"

	"github.com/spf13/viper"
)

type AppConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	LogLevel string `mapstructure:"log_level"`
}

var cfg *AppConfig

func GetConfig() *AppConfig {
	if cfg == nil {
		cfg = InitConfig()
	}

	return cfg
}

func InitConfig() *AppConfig {
	v := viper.New()

	v.SetConfigFile("app_config")
	v.SetConfigType("json")
	v.AddConfigPath(".")

	if err := v.ReadInConfig(); err != nil {
		panic(fmt.Errorf("加载配置失败: %w", err))
	}

	var config AppConfig

	if err := v.Unmarshal(&config); err != nil {
		panic(fmt.Errorf("解析配置失败: %w", err))
	}

	return &config
}
