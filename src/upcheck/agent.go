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

func PeriodicallyCheckHost(ip string, port int, intervalSecs int, ctx context.Context, cmd <-chan ControlSignal, checks chan<- CheckInfo) {
	for paused := false; ; {
		select {
		case <-ctx.Done():
			log.Info().Msg("Context cancelled - returning")
			return
		case command := <-cmd:
			if command == PAUSE {
				log.Info().Msg("Received PAUSE command - pausing")
				paused = true
			}
			if command == RESUME {
				log.Info().Msg("Received RESUME command - resuming")
				paused = false
			}
		default:
			if !paused {
				log.Debug().Msgf("Checking host %s:%d", ip, port)
				checkInfo, err := isHostListening(ip, port)
				if err != nil {
					log.Warn().Msgf("Error checking host %s:%d: %v", ip, port, err)
				}
				log.Debug().Msgf("Sending %v", checkInfo)
				checks <- checkInfo
				// intentionally not allowing time to go by while we might be timing out so we dont
				// get a thundering herd problem or anything.
			}
			time.Sleep(time.Duration(intervalSecs) * time.Second)
		}
	}
}
