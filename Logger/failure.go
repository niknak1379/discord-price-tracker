package logger

import (
	"context"
	"log/slog"

	proxy "priceTracker/Proxy"
	types "priceTracker/Types"
)

var (
	IncidentChannel chan types.Incident
	IncidentCounter int
)

func StartIncidentListener(dbFunc func(*types.Incident), ctx context.Context) {
	IncidentChannel = make(chan types.Incident, 100)
	go func() {
		for {
			select {
			case Incident := <-IncidentChannel:
				slog.Warn("logging Incident", slog.Any("incident", Incident))
				dbFunc(&Incident)
				proxy.ProxyCounterChannel <- Incident.Attempts
			case <-ctx.Done():
				return
			}
		}
	}()
}
