package logger

import (
	"context"
	"log/slog"
	"os"
)

type CrawlFiles struct {
	ItemName   string
	CrawlType  string
	HTMLString string
	ScreenShot []byte
}

var LogFileChannel chan *CrawlFiles

func InitLogFileChannel(ctx context.Context) {
	LogFileChannel = make(chan *CrawlFiles, 10)
	go processLogFiles(ctx)
}

func processLogFiles(ctx context.Context) {
	for {
		select {
		case crawlFiles := <-LogFileChannel:
			fileName := "/logs/" + crawlFiles.ItemName + crawlFiles.CrawlType

			if crawlFiles.HTMLString != "" {
				err := os.WriteFile(fileName+"HTML.html", []byte(crawlFiles.HTMLString), 0o644)
				if err != nil {
					slog.Error("could not write log files",
						slog.Any("err", err),
					)
				}
			}
			if len(crawlFiles.ScreenShot) != 0 {
				err := os.WriteFile(fileName+"SS.png", crawlFiles.ScreenShot, 0o644)
				if err != nil {
					slog.Error("could not write log files",
						slog.Any("err", err),
					)
				}
			}
			return
		case <-ctx.Done():
			return
		}
	}
}
