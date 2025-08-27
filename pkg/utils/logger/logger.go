package logger

import (
	"fmt"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

func NewLogger(file, level string) (*zap.Logger, error) {
	var err error
	var l *zap.Logger
	if l, err = newLogger(file, level); err != nil {
		return nil, fmt.Errorf("failed to open log file: %s", err)
	}
	return l, nil
}

func newLogger(logfile, loglevel string) (*zap.Logger, error) {
	var level zapcore.Level
	if err := level.UnmarshalText([]byte(loglevel)); err != nil {
		return nil, fmt.Errorf("failed to unmarshal level %s, error: %s", loglevel, err)
	}

	// check if logfile is valid
	f, err := os.OpenFile(logfile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	f.Close()

	cfg := zapcore.EncoderConfig{
		TimeKey:  "time",
		LevelKey: "level",
		//NameKey:    "logger",
		CallerKey:  "caller",
		MessageKey: "msg",
		//StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalLevelEncoder,
		EncodeTime:     zapcore.TimeEncoderOfLayout("2006-01-02 15:04:05.000"),
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	// use lumberjack to rotate logfile
	writer := &lumberjack.Logger{
		Filename:   logfile,
		MaxSize:    100, // megabytes
		MaxBackups: 7,
		//MaxAge:     28,    //days
		LocalTime: true,
		Compress:  false,
	}

	core := zapcore.NewCore(
		zapcore.NewConsoleEncoder(cfg),
		//zapcore.NewMultiWriteSyncer(zapcore.AddSync(writer), zapcore.AddSync(os.Stdout)),
		zapcore.NewMultiWriteSyncer(zapcore.AddSync(writer)),
		level,
	)

	logger := zap.New(core, zap.AddCaller())

	return logger, nil
}
