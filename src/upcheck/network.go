package upcheck

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/go-ping/ping"
	"github.com/jackpal/gateway"
	"github.com/rs/zerolog/log"
)

type NetworkInfo struct {
	Address net.IP
	Mask    net.IPMask
	GW      net.IP
}

func (n NetworkInfo) Equals(o NetworkInfo) bool {
	return n.Address.Equal(o.Address) && bytes.Equal(n.Mask, o.Mask) && n.GW.Equal(o.GW)
}

func (n NetworkInfo) String() string {
	return fmt.Sprintf("Address: %s\nMask: %s\nGW: %s", n.Address, IPMaskToString(n.Mask), n.GW)
}

func GetNetworkInfo() (netInfo NetworkInfo, err error) {
	ifs, err := gateway.DiscoverInterface()
	if err != nil {
		log.Fatal().Msgf("Error getting interfaces: %v", err)
	}
	log.Debug().Msgf("interface: %v %s\n", ifs, net.IP(ifs.DefaultMask()).String())
	gw, err := gateway.DiscoverGateway()
	if err != nil {
		log.Fatal().Msgf("Error getting gateway: %v", err)
	}
	log.Debug().Msgf("Default Gateway: %v\n", gw)
	return NetworkInfo{Address: ifs, Mask: ifs.DefaultMask(), GW: gw}, nil
}

func GetLocalIP() (net.IP, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil, fmt.Errorf("error getting network interfaces: %w", err)
	}
	for _, addr := range addrs {
		ipnet, ok := addr.(*net.IPNet)
		if ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP, nil
			}
		}
	}
	return nil, fmt.Errorf("no IP found")
}

func GetNetmask(ip net.IP) (net.IPMask, error) {
	netInterfaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("error getting network interfaces: %w", err)
	}
	for _, iface := range netInterfaces {
		addresses, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addresses {
			ipnet, ok := addr.(*net.IPNet)
			if ok && ipnet.IP.Equal(ip) {
				return ipnet.Mask, nil
			}
		}
	}
	return nil, fmt.Errorf("no netmask found")
}

func isInSubnet(ip net.IP, subnet net.IPNet, mask net.IPMask) bool {
	// compare the network portion of the IP address
	// with the network portion of the subnet
	if ip.Mask(mask).Equal(subnet.IP.Mask(mask)) {
		return true
	} else {
		return false
	}
}

func getDarwinGateway() (net.IP, error) {
	// Use "route -n get default" command for macOS
	gw := net.IP{}
	gatewayFound := false

	cmd := exec.Command("route", "-n", "get", "default")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return nil, err
	}

	output := out.String()
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "gateway:") {
			gatewayFound = true
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				// parse a string like "1.2.3.4" into a net.IP
				gw = net.ParseIP(parts[1])
				if gw == nil {
					return nil, fmt.Errorf("invalid gateway IP address")
				}
			}
		}
	}
	if !gatewayFound {
		return nil, fmt.Errorf("gateway not found - network likely down")
	}
	return gw, nil
}

func getLinuxGateway() (net.IP, error) {
	gw := net.IP{}
	// tom@hanalei:~$ route -n
	// Kernel IP routing table
	// Destination     Gateway         Genmask         Flags Metric Ref    Use Iface
	// 0.0.0.0         192.168.0.254   0.0.0.0         UG    0      0        0 enp6s0
	// 172.17.0.0      0.0.0.0         255.255.0.0     U     0      0        0 docker0
	// 172.18.0.0      0.0.0.0         255.255.0.0     U     0      0        0 br-0f2b158226c8
	// 192.168.0.0     0.0.0.0         255.255.255.0   U     0      0        0 enp6s0
	// tom@hanalei:~$
	cmd := exec.Command("/sbin/route", "-n")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return nil, err
	}

	output := out.String()
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "0.0.0.0") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				gw = net.IP(parts[1])
				break
			}
		}
	}
	return gw, nil
}

func PingHost(host string, timeoutSecs int) (bool, error) {
	pinger, err := ping.NewPinger(host)
	if err != nil {
		log.Warn().Msgf("Ping failed:", err)
		return false, err
	}
	timer1 := time.NewTimer(time.Duration(timeoutSecs) * time.Second)
	go func() {
		<-timer1.C
		pinger.Stop()
	}()
	pinger.Count = 1
	pinger.SetPrivileged(true)
	err = pinger.Run()
	time.Sleep(2 * time.Second)
	stats := pinger.Statistics()
	if err == nil {
		log.Debug().Msgf("Gateway is up: %v", stats)
		return true, nil
	} else {
		log.Warn().Msgf("Gateway is down or unreachable: %v", err)
		return false, err
	}
}

func GetDefaultGateway() (net.IP, error) {
	myOs := runtime.GOOS
	log.Debug().Msgf("OS: %s", myOs)
	if myOs == "linux" {
		return getLinuxGateway()
	} else if myOs == "darwin" {
		return getDarwinGateway()
	}
	return nil, fmt.Errorf("unsupported OS")
}

// IPMaskToString converts a net.IPMask to a string in a.b.c.d format
func IPMaskToString(mask net.IPMask) string {
	parts := make([]string, len(mask))
	for i, b := range mask {
		parts[i] = fmt.Sprintf("%d", b)
	}
	return strings.Join(parts, ".")
}

func IsInSameSubnet(baseIP net.IP, mask net.IPMask, checkIP net.IP) bool {
	baseNetwork := baseIP.Mask(mask)
	checkNetwork := checkIP.Mask(mask)
	return baseNetwork.Equal(checkNetwork)
}

func HasNetworkConnection() bool {
	hasNetwork := false
	interfaces, err := net.Interfaces()
	if err != nil {
		log.Warn().Msgf("Error fetching interfaces: %v\n", err)
		return false
	}

	for _, iface := range interfaces {
		// Ignore interfaces that are down or loopback
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		// Check if the interface name indicates a tunnel (e.g., utun*)
		if strings.HasPrefix(iface.Name, "utun") {
			continue
		}

		// Retrieve addresses associated with the interface
		addrs, err := iface.Addrs()
		if err != nil {
			log.Warn().Msgf("Error fetching addresses for interface %s: %v\n", iface.Name, err)
			continue
		}

		// Look for valid IPv4 addresses
		hasValidIPv4 := false
		for _, addr := range addrs {
			ip, _, err := net.ParseCIDR(addr.String())
			if err != nil {
				continue
			}
			if ip.To4() != nil {
				hasValidIPv4 = true
				hasNetwork = true
				log.Debug().Msgf("Interface: %s, IPv4 Address: %s\n", iface.Name, ip.String())
			}
		}

		// If no valid IPv4 is found, report the interface as inactive
		if !hasValidIPv4 {
			log.Debug().Msgf("Interface: %s has no valid IPv4 address.\n", iface.Name)
		}
	}
	return hasNetwork
}

func CheckViaDNS(nameServer string, hostname string) {
	resolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			return net.Dial(network, nameServer)
		},
	}
	start := time.Now()
	ips, err := resolver.LookupHost(context.Background(), hostname)
	log.Debug().Msgf("LookupHost took %v\n", time.Since(start))
	if err != nil {
		log.Debug().Msgf("Error:", err)
	} else {
		log.Debug().Msgf("IPs:", ips)
	}
}
