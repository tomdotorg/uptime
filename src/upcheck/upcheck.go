package upcheck

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

type Target struct {
	Mu           sync.RWMutex
	Name         string
	Host         string
	Port         int
	IP           net.IP
	IsAlive      bool
	Since        time.Time
	CurrentError string
	LastLatency  time.Duration
	TotalLatency time.Duration
	Attempts     int
	Failures     int
	Errors       map[string]int
	CmdChan      chan ControlSignal
}

var defaultTargets = []*Target{
	{
		Name:     "Google DNS",
		Host:     "8.8.8.8",
		IP:       net.IP{8, 8, 8, 8},
		Port:     53,
		IsAlive:  true,
		Since:    time.Now(),
		Attempts: 0,
		Failures: 0,
		Errors:   make(map[string]int),
		CmdChan:  make(chan ControlSignal, 10),
	},
	{
		Name:     "Cloudflare DNS",
		Host:     "1.1.1.1",
		IP:       net.IP{1, 1, 1, 1},
		Port:     53,
		IsAlive:  true,
		Since:    time.Now(),
		Attempts: 0,
		Failures: 0,
		Errors:   make(map[string]int),
		CmdChan:  make(chan ControlSignal, 10),
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

// isHostListening checks if a host is listening on a given port.
func isHostListening(ctx context.Context, host string, port int) (checkInfo CheckInfo, err error) {
	hostPort := net.JoinHostPort(host, strconv.Itoa(port))
	checkInfo.Host = host
	checkInfo.Port = port
	start := time.Now()
	var dialer = &net.Dialer{}
	conn, err := dialer.DialContext(ctx, "tcp", hostPort)
	checkInfo.Latency = time.Now().Sub(start)
	if conn != nil {
		defer func(conn net.Conn) {
			connErr := conn.Close()
			if connErr != nil {
				log.Error().Err(err).Msgf("error closing connection: %s", err)
			}
		}(conn)
	}
	// TODO: after a host has a problem, hitting s locks up on the host with the error

	if err != nil {
		// if isMemoryError(err) {
		// 	log.Info().Msgf("memory error connecting to %s : %s", host, err)
		// 	printMemUsage()
		// 	// for now, ignore memory errors TODO: handle this better
		// 	return CheckInfo{true, host, port, checkInfo.Latency, nil}, nil
		// }
		return CheckInfo{false, host, port, checkInfo.Latency, err}, err
	}
	// if we get here, the connection was successful
	return CheckInfo{true, host, port, checkInfo.Latency, nil}, nil
}

// FindDefaultGateway returns the Target that matches the default gateway from the NetInfo struct
func FindDefaultGateway(targets []*Target, defaultGW *NetworkInfo) *Target {
	for _, target := range targets {
		target.Mu.RLock()
		if target.IP.Equal(defaultGW.GW) {
			target.Mu.RUnlock()
			return target
		} else {
			target.Mu.RUnlock()
		}
	}
	return nil
}

// todo make this thread safe

// AddDefaultGatewayTarget adds the default gateway to the list of targets
func AddDefaultGatewayTarget(ctx context.Context, targets []*Target, netInfo *NetworkInfo) ([]*Target, *Target) {
	if FindDefaultGateway(targets, netInfo) != nil { // already in the list
		log.Info().Msgf("default gateway %s is already in the list - not adding", netInfo.GW)
		return targets, nil
	}
	// see if the gw is listening on 53, else try 80, else quit trying
	var foundListenPort = false
	var listenPort = -1
	const (
		PortDNS      = 53
		PortHTTP     = 80
		PortHTTPS    = 443
		PortSSH      = 22
		PortHTTPAlt  = 8080
		PortHTTPSAlt = 8443
		PortNTP      = 123
	)
	ports := []int{PortDNS, PortHTTP, PortHTTPS, PortSSH, PortHTTPAlt, PortHTTPSAlt, PortNTP}
	var upCheckInfo CheckInfo
	addr := netInfo.GW.String()
	for _, targetPort := range ports {
		upCheckInfo, _ = isHostListening(ctx, addr, targetPort)
		if upCheckInfo.IsUp {
			foundListenPort = true
			listenPort = targetPort
			log.Info().Msgf("default gateway %s is listening on port %d", netInfo.GW, targetPort)
			break
		}
	}
	if !foundListenPort {
		log.Warn().Msgf("default gateway %s is not listening on known ports", netInfo.GW)
	}
	netInfo.Mu.RLock()
	name := netInfo.GW.String() + " (auto)"
	rec := &Target{
		Name:        name,
		Host:        netInfo.GW.String(),
		IP:          netInfo.GW,
		Port:        listenPort,
		Attempts:    0,
		Failures:    0,
		IsAlive:     true,
		Since:       time.Time{},
		LastLatency: upCheckInfo.Latency,
		Errors:      make(map[string]int),
		CmdChan:     make(chan ControlSignal, 10),
	}
	log.Debug().Msgf("adding %v", rec)
	netInfo.Mu.RUnlock()
	rec.Since = time.Now()
	rec.TotalLatency += upCheckInfo.Latency
	log.Info().Msgf("adding %v - targets is %d big", rec, len(targets))
	targets = append(targets, rec)
	log.Info().Msgf("added %v - targets is %d big", rec, len(targets))
	return targets, rec
}

func LoadTargets(ctx context.Context, filename string) []*Target {
	results := make([]*Target, 0)

	// Open the file
	file, err := os.Open(filename)
	if err != nil {
		log.Warn().Err(err).Msgf("error opening %s", filename)
		log.Info().Msg("using defaults")
		results = defaultTargets
	} else {
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
						Attempts: 0,
						Failures: 0,
						IsAlive:  true,
						Since:    time.Time{},
						Errors:   make(map[string]int),
						CmdChan:  make(chan ControlSignal, 10),
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
	}

	netInfo, err := GetNetworkInfo()
	if err != nil {
		log.Warn().Msg("error getting network info so no default gw check")
	} else {
		defaultGWTarget := FindDefaultGateway(results, netInfo)
		if defaultGWTarget == nil {
			results, _ = AddDefaultGatewayTarget(ctx, results, netInfo)
		}
	}
	return results
}

func FindTarget(targets []*Target, host string, port int) *Target {
	for _, target := range targets {
		target.Mu.RLock()
		if target.Host == host && target.Port == port {
			target.Mu.RUnlock()
			return target
		}
		target.Mu.RUnlock()
	}
	return nil
}

func updateTargetStats(target *Target, upCheckInfo CheckInfo) {
	target.Mu.Lock()
	defer target.Mu.Unlock()
	target.Attempts++
	if upCheckInfo.Err != nil {
		target.CurrentError = upCheckInfo.Err.Error()
		target.Errors[upCheckInfo.Err.Error()]++
	}
	if upCheckInfo.IsUp {
		target.LastLatency = upCheckInfo.Latency
		target.TotalLatency += upCheckInfo.Latency
		if !target.IsAlive {
			target.CurrentError = ""
			log.Info().Msgf("target %s is back up - was down for %s", target.Name, time.Now().Sub(target.Since).Round(time.Second).String())
			target.Since = time.Now()
		}
		target.IsAlive = true
	} else {
		target.Failures++
		if target.IsAlive {
			log.Info().Msgf("target %s is down - was up for %s (%s)", target.Name, time.Now().Sub(target.Since).Round(time.Second).String(), target.CurrentError)
			target.Since = time.Now()
		}
		target.IsAlive = false
	}
}

func (t *Target) Copy() *Target {
	t.Mu.RLock()
	defer t.Mu.RUnlock()

	newTarget := Target{
		Mu:           sync.RWMutex{},
		Name:         t.Name,
		Host:         t.Host,
		Port:         t.Port,
		IP:           make(net.IP, len(t.IP)),
		IsAlive:      t.IsAlive,
		Since:        t.Since,
		CurrentError: t.CurrentError,
		LastLatency:  t.LastLatency,
		TotalLatency: t.TotalLatency,
		Attempts:     t.Attempts,
		Failures:     t.Failures,
		Errors:       make(map[string]int),
		CmdChan:      make(chan ControlSignal, 10),
	}

	copy(newTarget.IP, t.IP)
	for k, v := range t.Errors {
		newTarget.Errors[k] = v
	}

	return &newTarget
}

func (t *Target) String() string {
	t.Mu.RLock()
	defer t.Mu.RUnlock()
	// dt := t.Since.Format("15:04:05")
	var alive string
	if !t.IsAlive {
		alive = "DOWN"
	} else {
		alive = "UP"
	}

	uptime := time.Now().Sub(t.Since).Round(time.Second)

	errorStr := t.CurrentError
	if errorStr == "" { // no current error, so count the errors
		log.Debug().Msgf("no current error, so count the errors")
		errorCount := len(t.Errors)
		errorStr = strconv.Itoa(errorCount)
	}
	var avgLatency string
	if t.Attempts-t.Failures == 0 {
		avgLatency = "NaN"
	} else {
		avgLatency = strconv.FormatInt(t.TotalLatency.Milliseconds()/int64(t.Attempts-t.Failures), 10)
	}

	uptimeAvg := "NaN"
	if t.Attempts > 0 {
		uptimeAvg = fmt.Sprintf("%6.02f%%", float32(t.Attempts-t.Failures)/float32(t.Attempts)*100.0)
	}

	return fmt.Sprintf("%-20s - %-4s %dms (avg %sms) %v %s %d/%d (%s)", t.Name, alive, t.LastLatency.Milliseconds(), avgLatency, uptime, uptimeAvg, t.Attempts-t.Failures, t.Attempts, errorStr)
}

func (t *Target) Update(info CheckInfo) {
	updateTargetStats(t, info)
}

func ResetAllStats(targets []*Target) {
	for _, target := range targets {
		resetStats(target)
	}
}

func resetStats(target *Target) {
	target.Mu.Lock()
	defer target.Mu.Unlock()
	target.Since = time.Now()
	target.Attempts = 0
	target.Failures = 0
	target.Errors = make(map[string]int)
	target.CurrentError = ""
	target.LastLatency = 0
	target.TotalLatency = 0
}

// ClassifyTargets classify the targets as on this subnet, gateway, or external to this subnet
func ClassifyTargets(targets []*Target, netInfo *NetworkInfo) (subnetTargets, gatewayTargets, externalTargets []*Target, ok bool) {
	if netInfo == nil {
		log.Warn().Msg("can't classify targets with no network")
		ok = false
		return
	}

	subnetTargets = make([]*Target, 0)
	gatewayTargets = make([]*Target, 0)
	externalTargets = make([]*Target, 0)

	log.Debug().Msgf("Classifying targets for\n%s", netInfo)
	for _, target := range targets {
		log.Debug().Msgf("locking %v", target)
		target.Mu.RLock()
		ip := target.IP
		target.Mu.RUnlock()
		log.Debug().Msgf("unlocked %v", target)
		mask := netInfo.Mask
		addr := netInfo.Address
		if ip.Equal(netInfo.GW) {
			gatewayTargets = append(gatewayTargets, target)
		} else if IsInSameSubnet(addr, mask, ip) {
			subnetTargets = append(subnetTargets, target)
		} else {
			externalTargets = append(externalTargets, target)
		}
	}
	ok = true
	return
}
