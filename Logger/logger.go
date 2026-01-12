package logger

import (
	"log/slog"
	"os"
)
var Logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
    AddSource: true,
}))
func init(){
	slog.SetDefault(Logger)
}

