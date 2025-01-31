package main

import (
	"fmt"
	"net"
	"strconv"
	"time"
)

func main() {
	host := "8.8.8.8"
	port := 53
	address := net.JoinHostPort(host, strconv.Itoa(port))
	for {
		_, err := net.DialTimeout("tcp", address, 5*time.Second)
		if err != nil {
			fmt.Printf("error connecting to %s : %s", host, err)
		} else {
			fmt.Printf("connected to %s\n", address)
		}
		time.Sleep(1000 * time.Millisecond)
	}

}
