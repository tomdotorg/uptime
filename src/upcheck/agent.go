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
			log.Info().Msg("Context cancelled - returning")
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
					log.Warn().Msgf("Error checking host %s:%d: %v", host, port, err)
				}
				log.Debug().Msgf("Sending %v", checkInfo)
				checks <- checkInfo
				cancel()
			}
		}
	}
}

/*
func PeriodicallyCheckHost(target *Target, intervalSecs int, ctx context.Context, cmd <-chan ControlSignal, checks chan<- CheckInfo) {
	paused := false
	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("Context cancelled - returning")
			return
		case command := <-cmd:
			if command == PAUSE {
				log.Info().Msgf("%v Received PAUSE command", target.IP)
				paused = true
			}
			if command == RESUME {
				log.Info().Msgf("%v Received RESUME command", target.IP)
				paused = false
			}
		default:
			if !paused {
				log.Debug().Msgf("Checking host %s:%d", target.IP, target.Port)
				checkInfo, err := isHostListening(target.Host, target.Port)
				if err != nil {
					log.Info().Msgf("Error checking host %s:%d: %v", target.Host, target.Port, err)
				}
				log.Debug().Msgf("Sending %v", checkInfo)
				checks <- checkInfo
				// intentionally not allowing time to go by while we might be timing out so we don't
				// get a thundering herd problem or anything.
			} else {
				log.Debug().Msgf("paused, so skipping check: %v:%v", target.Host, target.Port)
			}
			time.Sleep(time.Duration(intervalSecs) * time.Second)
		}
	}
}
*/
