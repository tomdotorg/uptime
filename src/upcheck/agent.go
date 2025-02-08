package upcheck

import "time"

type CheckInfo struct {
	isUp    bool
	host    string
	port    int
	latency time.Duration
	err     error
}

func checkHost(ip string, port int, cmd <-chan string, checks chan<- CheckInfo) {
	for {
		select {
		case command := <-cmd:
			switch command {
			case "poll":
				checkInfo, _ := isHostListening(ip, port)
				checks <- checkInfo
			case "stop":
				return
			}
		}
	}
}
