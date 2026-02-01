package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var Log *zap.Logger

// InitLogger 初始化全局日志变量
func InitLogger() {
	// 1. 创建一个默认的生产环境配置（输出为 JSON 格式，性能最高）
	config := zap.NewProductionConfig()
	
	// 2. 自定义时间格式（可选，默认是时间戳，改成人类易读的）
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	
	var err error
	Log, err = config.Build()
	if err != nil {
		panic("无法初始化日志库: " + err.Error())
	}
}