package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var Agent, Server, Echo *zap.Logger

func InitAgentLogger(file, level string) {
	var err error
	if Agent, err = newLogger(file, level); err != nil {
		panic(err)
	}
	//sync
	defer Agent.Sync()
}

func InitServerLogger(file, level string) {
	var err error
	if Server, err = newLogger(file, level); err != nil {
		panic(err)
	}
	//sync
	defer Server.Sync()
}

func InitEchoLogger(file, level string) {
	var err error
	if Echo, err = newLogger(file, level); err != nil {
		panic(err)
	}
	//sync
	defer Echo.Sync()
}

func newLogger(file, level string) (*zap.Logger, error) {
	enc := zap.NewProductionEncoderConfig()
	enc.TimeKey = "time"
	enc.EncodeTime = zapcore.TimeEncoderOfLayout("2006-01-02 15:04:05.000")
	enc.EncodeCaller = zapcore.ShortCallerEncoder

	cfg := zap.NewProductionConfig()
	cfg.OutputPaths = []string{}
	//cfg.OutputPaths = append(cfg.OutputPaths, "stdout")
	cfg.OutputPaths = append(cfg.OutputPaths, file)
	cfg.Encoding = "console" // "json" or "console"
	cfg.EncoderConfig = enc
	lvl := getZapLevel(level)
	cfg.Level.SetLevel(lvl)

	return cfg.Build()
}

func getZapLevel(level string) zapcore.Level {
	switch level {
	case "info":
		return zapcore.InfoLevel
	case "warn":
		return zapcore.WarnLevel
	case "debug":
		return zapcore.DebugLevel
	case "error":
		return zapcore.ErrorLevel
	case "fatal":
		return zapcore.FatalLevel
	default:
		return zapcore.InfoLevel
	}
}
