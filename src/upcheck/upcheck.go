package upcheck

import (
	"fmt"
	"github.com/rs/zerolog/log"
	"time"
)

type Target struct {
	Name     string
	Host     string
	Port     int
	Type     int
	IsAlive  bool
	Since    time.Time
	Attempts int
	Failures int
	Errors   map[string]int
}

func (t Target) String() string {
	return fmt.Sprintf("%v:%v - Alive: %v since %v (%3.02f success rate) %d/%d (%v)", t.Host, t.Port, t.IsAlive, t.Since, float32(t.Attempts-t.Failures)/float32(t.Attempts)*100.0, t.Attempts-t.Failures, t.Attempts, t.Errors)
}

func ShowStatus(target Target) {
	if target.IsAlive {
		log.Info().Msgf("target: %+v", target.String())
	} else {
		log.Warn().Msgf("target: %+v", target.String())
	}
	//for err, count := range target.Errors {
	//	log.Info().Msgf("error: %v count: %v", err, count)
	//}
}

func ShowStatuses(targets []*Target) {
	for _, target := range targets {
		ShowStatus(*target)
	}
	log.Info().Msgf("----")
}
