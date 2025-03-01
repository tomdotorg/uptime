package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"upcheck"
)

func main() {
	// Get default gateway
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(1*time.Second))
	duration, err := upcheck.PingHost(ctx, "google.com")
	if err != nil {
		log.Fatalf("Error calling ping: %v", err)
	}
	fmt.Printf("latency: %v", duration)
	cancel()
}
