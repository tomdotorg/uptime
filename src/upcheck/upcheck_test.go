// src/upcheck/upcheck_test.go
package upcheck

import (
	"testing"
	"time"
)

func TestTargetString(t *testing.T) {
	target := Target{
		Name:     "Test Target",
		Host:     "localhost",
		Port:     8080,
		Type:     1,
		IsAlive:  true,
		Since:    time.Now(),
		Attempts: 10,
		Failures: 2,
		Errors:   map[string]int{"timeout": 1, "connection refused": 1},
	}

	expected := "localhost:8080 - Alive: true since "
	if got := target.String(); got[:len(expected)] != expected {
		t.Errorf("Target.String() = %v, want prefix %v", got, expected)
	}
}

func TestShowStatus(t *testing.T) {
	target := Target{
		Name:     "Test Target",
		Host:     "localhost",
		Port:     8080,
		Type:     1,
		IsAlive:  true,
		Since:    time.Now(),
		Attempts: 5,
		Failures: 1,
		Errors:   map[string]int{"timeout": 1},
	}

	// We can't capture log output easily, so just ensure no panics
	ShowStatus(target)
}

func TestShowStatuses(t *testing.T) {
	target1 := &Target{
		Name:     "Target 1",
		Host:     "localhost",
		Port:     8080,
		Type:     1,
		IsAlive:  true,
		Since:    time.Now(),
		Attempts: 10,
		Failures: 2,
		Errors:   map[string]int{"timeout": 1, "connection refused": 1},
	}

	target2 := &Target{
		Name:     "Target 2",
		Host:     "localhost",
		Port:     9090,
		Type:     1,
		IsAlive:  false,
		Since:    time.Now(),
		Attempts: 5,
		Failures: 5,
		Errors:   map[string]int{"timeout": 3, "connection refused": 2},
	}

	targets := []*Target{target1, target2}

	// We can't capture log output easily, so just ensure no panics
	ShowStatuses(targets)
}
