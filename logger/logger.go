package logger

import (
	"go.uber.org/zap"
)

var Logger *zap.Logger

func Init(debug bool) *zap.Logger {
	if debug {
		return zap.Must(zap.NewDevelopment())
	}
	return zap.Must(zap.NewProduction())
}

func Sync() {
	if Logger != nil {
		Logger.Sync()
	}
}
