package main

import (
	"fmt"
	"log"
	"net"
	"time"

	"github.com/go-ping/ping"
	"github.com/jackpal/gateway"
)

func main() {
	// Get default gateway
	ifs, err := gateway.DiscoverInterface()
	if err != nil {
		log.Fatalf("Error getting interfaces: %v", err)
	}
	fmt.Printf("interface: %v %s\n", ifs, net.IP(ifs.DefaultMask()).String())
	gw, err := gateway.DiscoverGateway()
	if err != nil {
		log.Fatalf("Error getting gateway: %v", err)
	}
	fmt.Printf("Default Gateway: %v\n", gw)

	PingHost(gw, 2)
}
