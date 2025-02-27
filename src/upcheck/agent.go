package upcheck

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"
)

type ControlSignal int

const (
	PAUSE ControlSignal = iota
	RESUME
)

type CheckInfo struct {
	IsUp    bool
	Host    string
	Port    int
	Latency time.Duration
	Err     error
}

func PeriodicallyCheckHost(host string, port int, intervalSecs int, ctx context.Context, cmd <-chan ControlSignal, checks chan<- CheckInfo) {
	paused := false
	ticker := time.NewTicker(time.Duration(intervalSecs) * time.Second)
	defer ticker.Stop()

	lastErr := ""

	for {
		select {
		case <-ctx.Done():
			log.Info().Msgf("%s:%d - context cancelled - returning", host, port)
			return
		case command := <-cmd:
			if command == PAUSE {
				log.Info().Msgf("%v Received PAUSE command", host)
				paused = true
			}
			if command == RESUME {
				log.Info().Msgf("%v Received RESUME command", host)
				paused = false
			}
		case <-ticker.C:
			if !paused {
				checkCtx, cancel := context.WithTimeout(context.Background(), time.Duration(intervalSecs)*time.Second)
				log.Debug().Msgf("Checking host %s:%d", host, port)
				checkInfo, err := isHostListening(checkCtx, host, port)
				if err != nil && err.Error() != lastErr {
					lastErr = err.Error()
					log.Debug().Msgf("Error checking host %s:%d: %v", host, port, err)
				}
				log.Debug().Msgf("Sending %v", checkInfo)
				checks <- checkInfo
				cancel()
			}
		}
	}
}
