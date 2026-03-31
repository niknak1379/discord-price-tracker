package logger

import (
	"log/slog"
	"os"
)

var logLevel = new(slog.LevelVar)

var Logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
	AddSource: true,
	Level:     logLevel,
}))

func ToggleLogLevel() slog.Level {
	current := logLevel.Level()
	if current == slog.LevelInfo {
		logLevel.Set(slog.LevelDebug)
	} else {
		logLevel.Set(slog.LevelInfo)
	}
	return logLevel.Level()
}

func GetLogLevel() slog.Level {
	return logLevel.Level()
}
