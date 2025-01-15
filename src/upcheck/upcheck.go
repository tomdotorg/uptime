package upcheck

import (
	"fmt"
	"net"
	"time"
)

type Target struct {
	Name         string
	Host         string
	Port         int
	IP           net.IP
	Type         int
	IsAlive      bool
	Since        time.Time
	CurrentError string
	Attempts     int
	Failures     int
	Errors       map[string]int
}

func (t Target) String() string {
	dt := t.Since.Format("15:04:05")
	alive := "OFFLINE"
	if !t.IsAlive {
		alive = "OFFLINE"
	} else {
		alive = "ONLINE"
	}
	return fmt.Sprintf("%v:%v - %s since %s (%3.02f%%) %d/%d (%v)", t.Host, t.Port, alive, dt, float32(t.Attempts-t.Failures)/float32(t.Attempts)*100.0, t.Attempts-t.Failures, t.Attempts, t.CurrentError)
}

func ResetAllStats(targets []*Target) {
	for _, target := range targets {
		resetStats(target)
	}
}

func resetStats(target *Target) {
	target.IsAlive = true
	target.Since = time.Now()
	target.Attempts = 0
	target.Failures = 0
	target.Errors = make(map[string]int)
	target.CurrentError = ""
}

func ShowStatus(target Target) {
	if target.IsAlive {
		fmt.Printf("target: %+v\n", target.String())
	} else {
		fmt.Printf("target: %+v\n", target.String())
	}
}

func ShowStatuses(targets []*Target) {
	for _, target := range targets {
		ShowStatus(*target)
	}
	fmt.Println()
}
