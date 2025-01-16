package upcheck

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
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

var defaultTargets = []*Target{
	{
		Name:     "Google DNS",
		Host:     "8.8.8.8",
		IP:       net.IP{8, 8, 8, 8},
		Port:     53,
		Type:     0,
		IsAlive:  false,
		Since:    time.Now(),
		Attempts: 0,
		Failures: 0,
		Errors:   make(map[string]int),
	},
	{
		Name:     "Cloudflare DNS",
		Host:     "1.1.1.1",
		IP:       net.IP{1, 1, 1, 1},
		Port:     53,
		Type:     0,
		IsAlive:  false,
		Since:    time.Now(),
		Attempts: 0,
		Failures: 0,
		Errors:   make(map[string]int),
	},
}

func parseHostPortType(line string) (string, net.IP, int, error) {
	defaultPort := 80
	var port int
	var err error
	parts := strings.Split(line, ":")
	host := net.ParseIP(parts[0])
	// If a port is provided, parse it
	if len(parts) > 1 {
		port, err = strconv.Atoi(parts[1])
		if err != nil || port <= 0 || port > 65535 {
			log.Warn().Msgf("invalid port: %s", parts[1])
			port = -1
		}
	} else {
		port = defaultPort
	}
	// see if host is an IP address or a hostname
	if host == nil {
		// not an IP address, so it must be a hostname so resolve the hostname to an IP address
		ips, err := net.LookupIP(parts[0])
		if err == nil && len(ips) > 0 {
			host = ips[0]
		} else {
			log.Warn().Msgf("invalid hostname: %s", parts[0])
		}
	}
	return parts[0], host, port, nil
}

func isMemoryError(err error) bool {
	var opErr *net.OpError
	if errors.As(err, &opErr) && opErr.Op == "dial" && strings.Contains(opErr.Err.Error(), "cannot allocate memory") {
		return true
	} else {
		return false
	}
}

// isHostListening checks if a host is listening on a given port.
func isHostListening(host string, port int) (bool, error) {
	address := net.JoinHostPort(host, strconv.Itoa(port))
	conn, err := net.DialTimeout("tcp", address, 2*time.Second)
	if conn != nil {
		defer func(conn net.Conn) {
			connErr := conn.Close()
			if connErr != nil {
				log.Fatal().Err(err).Msgf("error closing connection: %s", err)
			}
		}(conn)
	}
	if err != nil {
		if isMemoryError(err) {
			log.Debug().Msgf("memory error connecting to %s : %s", host, err)
			printMemUsage()
			// for now, ignore memory errors TODO: handle this better
			return true, err
		}
		return false, err
	}
	// if we get here, the connection was successful
	return true, nil
}

func LoadTargets(filename string) []*Target {
	var results []*Target

	// Open the file
	file, err := os.Open(filename)
	if err != nil {
		log.Warn().Err(err).Msgf("error opening %s", filename)
		log.Info().Msg("using defaults")
		return defaultTargets
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			log.Fatal().Err(err).Msgf("error closing %s", filename)
		}
	}(file)

	// Read each line from the file
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "#") {
			host, ip, port, err := parseHostPortType(line)
			if err != nil {
				log.Warn().Msgf("invalid line: %s - skipping", line)
			} else {
				// Add the validated host:port to the results array
				rec := &Target{
					Name:     line,
					Host:     host,
					IP:       ip,
					Port:     port,
					Attempts: 1,
					Failures: 0,
					IsAlive:  true,
					Since:    time.Time{},
					Errors:   make(map[string]int),
				}
				rec.Since = time.Now()
				results = append(results, rec)
				log.Debug().Msgf("added %v", rec)
			}
		}
	}
	// Check for any scanner errors
	if err := scanner.Err(); err != nil {
		log.Fatal().Err(err).Msgf("error reading %s", filename)
	}
	return results
}

func CheckAllTargets(targets []*Target) {
	for _, target := range targets {
		alive, err := isHostListening(target.IP.String(), target.Port)
		target.Attempts++
		if err != nil {
			target.CurrentError = err.Error()
			target.Errors[err.Error()]++
		}
		if alive {
			if !target.IsAlive {
				target.IsAlive = true
				target.CurrentError = ""
				log.Info().Msgf("target %v is back up - was down for %s", target, time.Now().Sub(target.Since).Round(time.Second).String())
				target.Since = time.Now()
			}
			target.IsAlive = true
			log.Debug().Msgf("target %v is up", target)
		} else {
			target.Failures++
			if target.IsAlive {
				target.IsAlive = false
				log.Info().Msgf("target %v is down - was up for %s (%s)", target, time.Now().Sub(target.Since).Round(time.Second).String(), target.CurrentError)
				target.Since = time.Now()
			}
			target.IsAlive = false
		}
	} // for
}

func bToMb(b uint64) uint64 {
	return b / 1024 / 1024
}

func printMemUsage() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	// For info on each, see: https://golang.org/pkg/runtime/#MemStats
	log.Debug().Msgf("Alloc = %v MiB", bToMb(m.Alloc))
	log.Debug().Msgf("\tTotalAlloc = %v MiB", bToMb(m.TotalAlloc))
	log.Debug().Msgf("\tSys = %v MiB", bToMb(m.Sys))
	log.Debug().Msgf("\tNumGC = %v\n", m.NumGC)
}

func (t Target) String() string {
	dt := t.Since.Format("15:04:05")
	alive := "OFFLINE"
	if !t.IsAlive {
		alive = "OFFLINE"
	} else {
		alive = "ONLINE"
	}
	return fmt.Sprintf("%-20s - %s since %s (%3.02f%%) %d/%d (%v)", t.Name, alive, dt, float32(t.Attempts-t.Failures)/float32(t.Attempts)*100.0, t.Attempts-t.Failures, t.Attempts, t.CurrentError)
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
	fmt.Printf("%+v\n", target.String())
}

func ShowStatuses(targets []*Target) {
	for _, target := range targets {
		ShowStatus(*target)
	}
	fmt.Println()
}

func isNodeAliveOnAnyPort(address string, ports []string) (port int, err error) {
	for _, port := range ports {
		target := net.JoinHostPort(address, port)
		conn, err := net.DialTimeout("tcp", target, 2*time.Second)
		if err == nil {
			// the linter below is worried about the defer statement in the loop.
			// this is fine because the loop will exit after the first successful connection
			//goland:noinspection ALL
			defer conn.Close()
			fmt.Printf("Node %s is reachable on port %s\n", address, port)
			return strconv.Atoi(port)
		}
	}
	return -1, nil
}
