package main

import (
	"bufio"
	"errors"
	"fmt"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/rs/zerolog/pkgerrors"
	"net"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
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

// host, port, err := upcheck.ParseHostPort(line)
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

func getTargetsFromFile(filename string) []*upcheck.Target {
	var results []*upcheck.Target

	// Open the file
	file, err := os.Open(filename)
	if err != nil {
		log.Fatal().Err(err).Msgf("error opening %s", filename)
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
				log.Info().Msgf("added %v", rec)
			}
		}
	}
	// Check for any scanner errors
	if err := scanner.Err(); err != nil {
		log.Fatal().Err(err).Msgf("error reading %s", filename)
	}
	return results
}

func registerSignals(targets []*upcheck.Target) {
	log.Info().Msg("registering signals")
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, os.Kill)

	go func() {
		sig := <-c
		log.Info().Msgf("Received signal: %s", sig)
		if sig == syscall.SIGINT {
			log.Info().Msg("SIGINT caught")
			upcheck.ShowStatuses(targets)
		} else if sig == syscall.SIGKILL {
			log.Info().Msg("SIGKILL caught")
			upcheck.ShowStatuses(targets)
			os.Exit(1)
		} else {
			log.Info().Msgf("Caught signal: %s", sig)
		}
	}()
}

func showStatuses(targets []*upcheck.Target) {
	for {
		upcheck.ShowStatuses(targets)
		time.Sleep(10 * time.Second)
	}
}

func main() {
	initLogs()
	baseIP := net.ParseIP("192.168.1.1")
	baseMask := net.IPv4Mask(255, 255, 255, 0)
	checkIP := net.ParseIP("1.1.1.1")

	if upcheck.IsInSameSubnet(baseIP, baseMask, checkIP) {
		fmt.Println("The IP is in the same subnet.")
	} else {
		fmt.Println("The IP is not in the same subnet.")
	}

	thisHost, thisNetmask, thisGateway, err := getNetworkInfo()

	if err != nil {
		log.Fatal().Err(err).Msg("Error getting network info")
	} else {
		log.Info().Msgf("Local IP: %s", thisHost)
		log.Info().Msgf("Netmask: %s", upcheck.IPMaskToString(thisNetmask))
		log.Info().Msgf("Default Gateway: %s", thisGateway)
	}

	checkTargets := getTargetsFromFile(CONFIGFILE)
	//registerSignals(checkAllTargets)
	go showStatuses(checkTargets)
	//registerSignals(checkAllTargets)
	for {
		checkAllTargets(checkTargets)
		time.Sleep(1 * time.Second)
	}
}

func checkAllTargets(targets []*upcheck.Target) {
	// loop through all the targets
	//  state changes matter. if a target goes from up to down, or down to up, log it. do the fields in the target struct
	//  make sense? if not, change them.
	// determine status:
	//  if the local targets and gateway are available but the internet is not, then the isp is down.
	//  if local targets are online but the gateway and internet are not, then the gateway is down
	for _, target := range targets {
		target.Attempts++
		alive, err := isHostListening(target.Host, target.Port)
		if err != nil {
			target.Errors[err.Error()]++
			//			log.Warn().Msgf("error connecting to %s:%d", target.Host, target.Port)
		}
		if alive {
			if !target.IsAlive {
				target.IsAlive = true
				log.Info().Msgf("target %v is back up - was down for %s", target, time.Now().Sub(target.Since).Round(time.Second).String())
				target.Since = time.Now()
			}
			target.IsAlive = true
			log.Debug().Msgf("target %v is up", target)
		} else {
			target.Failures++
			if target.IsAlive {
				target.IsAlive = false
				log.Info().Msgf("target %v is down - was up for %s", target, time.Now().Sub(target.Since).Round(time.Second).String())
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

	//if os.Getenv("CONSOLE") != "" || 1 == 1 {
	//	log.Info().Msg("logging to console")
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})

	//} else {
	//	log.Output(os.Stdout)
	//}

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
