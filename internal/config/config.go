package config

import (
	"log"
	"time"
	"github.com/spf13/viper"
)

// Config 结构体，对应 yaml 里的字段
type Config struct {
    Server ServerConfig `mapstructure:"server"`
}

type ServerConfig struct {
    Eth EthConfig `mapstructure:"eth"`
}

type EthConfig struct {
    RpcUrl       string        `mapstructure:"rpc_url"`
    PollInterval time.Duration `mapstructure:"poll_interval"` 
	DefaultStartBlock uint64    `mapstructure:"default_start_block"`
}
func LoadConfig() *Config {
	// 1. 设置配置文件的名字和类型
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	// 2. 告诉 viper 在当前路径寻找
	viper.AddConfigPath("./internal/config/")
	// 3. 读取文件
	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("读取配置文件失败: %v", err)
	}

	// 4. 将读取到的内容解析到结构体里
	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		log.Fatalf("解析配置文件失败: %v", err)
	}

	return &cfg
}
