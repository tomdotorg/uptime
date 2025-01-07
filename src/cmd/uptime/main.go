package main

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

	"github.com/eiannone/keyboard"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/rs/zerolog/pkgerrors"
	"upcheck"
)

const CONFIGFILE = "hosts.txt"

var defaultTargets = []*upcheck.Target{
	{
		Name:     "Google DNS",
		Host:     "8.8.8.8",
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
		Port:     53,
		Type:     0,
		IsAlive:  false,
		Since:    time.Now(),
		Attempts: 0,
		Failures: 0,
		Errors:   make(map[string]int),
	},
}

func parseHostPortType(line string) (string, int, error) {
	defaultPort := 80
	// Split the connection string into host and port
	// TODO grab the type after a comma at the end. default to external

	commaIndex := strings.Index(line, ",")
	if commaIndex != -1 {
		line = line[:commaIndex]
	}
	parts := strings.Split(line, ":")
	host := parts[0]
	// see if host is an IP address or a hostname
	if net.ParseIP(host) == nil {
		// not an IP address, so it must be a hostname
		// resolve the hostname to an IP address
		_, err := net.LookupIP(host)
		if err != nil {
			return host, -1, err
		}
	}
	port := defaultPort

	// If a port is provided, parse it
	if len(parts) > 1 {
		var err error
		port, err = strconv.Atoi(parts[1])
		if err != nil || port <= 0 || port > 65535 {
			return host, -1, err
		}
	}
	return host, port, nil
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
	conn, err := net.DialTimeout("tcp", address, 5*time.Second)
	if conn != nil {
		defer func(conn net.Conn) {
			connErr := conn.Close()
			if connErr != nil {
				log.Fatal().Err(err).Msgf("error closing connection: %s", err)
			}
		}(conn)
	}
	if err != nil {
		// todo: treat dns issues specially: lookup www.tom.org: no such host:14])
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

func loadTargetsFromFile(filename string) []*upcheck.Target {
	var results []*upcheck.Target

	// Open the file
	file, err := os.Open(filename)
	if err != nil {
		log.Warn().Err(err).Msgf("error opening %s", filename)
		log.Info().Msg("uing defaults")
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
			host, port, err := parseHostPortType(line)
			if err != nil {
				log.Warn().Msgf("invalid line: %s - skipping", line)
			} else {
				// Add the validated host:port to the results array
				rec := &upcheck.Target{
					Name:     line,
					Host:     host,
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

func showStatuses(targets []*upcheck.Target) {
	for {
		upcheck.ShowStatuses(targets)
		time.Sleep(10 * time.Second)
	}
}

func main() {
	initLogs()
	thisHost, thisNetmask, thisGateway, err := getNetworkInfo()

	if err != nil {
		log.Fatal().Err(err).Msg("Error getting network info")
	} else {
		fmt.Printf("Local IP: %s\n", thisHost)
		fmt.Printf("Netmask: %s\n", upcheck.IPMaskToString(thisNetmask))
		fmt.Printf("Default Gateway: %s\n", thisGateway)
	}

	checkTargets := loadTargetsFromFile(CONFIGFILE)
	// go showStatuses(checkTargets)

	// Initialize keyboard listener
	if err := keyboard.Open(); err != nil {
		log.Fatal().Err(err).Msg("Failed to open keyboard")
	}

	defer func() {
		if err := keyboard.Close(); err != nil {
			log.Fatal().Err(err).Msg("Failed to close keyboard")
		}
	}()
	stopChan := make(chan struct{})
	go func(stopChan chan struct{}) {
		for {
			select {
			case <-stopChan:
				log.Info().Msg("Stopping...")
				return
			default:
				checkAllTargets(checkTargets)
				time.Sleep(1 * time.Second)
			}

		}
	}(stopChan)
	for {
		handleKeys(checkTargets, stopChan)
	}
}

func handleKeys(checkTargets []*upcheck.Target, stopChan chan struct{}) []*upcheck.Target {
	// Check for key presses
	if char, key, err := keyboard.GetKey(); err == nil {
		if key == keyboard.KeyEsc || key == keyboard.KeyCtrlC {
			fmt.Println("Exiting...")
			os.Exit(0)
		}
		switch char {
		case 's':
			upcheck.ShowStatuses(checkTargets)
			break
		case 'r':
			fmt.Println("Resetting all stats...")
			upcheck.ResetAllStats(checkTargets)
		default:
			fmt.Printf("You pressed: %q\n", char)
		}
	}
	return checkTargets
}

func checkAllTargets(targets []*upcheck.Target) {
	for _, target := range targets {
		target.Attempts++
		alive, err := isHostListening(target.Host, target.Port)
		if err != nil {
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
			target.CurrentError = err.Error()
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

func initLogs() {
	// initialize the logger
	zerolog.ErrorStackMarshaler = pkgerrors.MarshalStack
	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	// if os.Getenv("CONSOLE") != "" || 1 == 1 {
	//	log.Info().Msg("logging to console")
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})

	// } else {
	//	log.Output(os.Stdout)
	// }

	if os.Getenv("DEBUG") != "" {
		zerolog.SetGlobalLevel(zerolog.TraceLevel)
		log.Info().Msg("enabling Trace level logging")
	} else {
		log.Info().Msg("enabling Info level logging")
	}
}

func getNetworkInfo() (localnet net.IP, mask net.IPMask, gw net.IP, err error) {
	localIP, err := upcheck.GetLocalIP()
	if err != nil {
		log.Warn().Msgf("Error getting local IP: %v", err)
		return nil, nil, nil, err
	}
	netmask, err := upcheck.GetNetmask(localIP)
	if err != nil {
		log.Warn().Msgf("Error getting netmask: %v", err)
		return localIP, nil, nil, err
	}
	defaultGateway, err := upcheck.GetDefaultGateway()
	if err != nil {
		log.Warn().Msgf("Error getting default gateway: %v", err)
		return localIP, netmask, nil, err
	}
	return localIP, netmask, defaultGateway, nil
}
