package upcheck

import (
	"bytes"
	"fmt"
	"github.com/rs/zerolog/log"
	"net"
	"os/exec"
	"runtime"
	"strings"
)

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
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("error getting network interfaces: %w", err)
	}
	for _, iface := range ifaces {
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
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
	return gw, nil
}

func getLinuxGateway() (net.IP, error) {
	gw := net.IP{}
	//tom@hanalei:~$ route -n
	//Kernel IP routing table
	//Destination     Gateway         Genmask         Flags Metric Ref    Use Iface
	//0.0.0.0         192.168.0.254   0.0.0.0         UG    0      0        0 enp6s0
	//172.17.0.0      0.0.0.0         255.255.0.0     U     0      0        0 docker0
	//172.18.0.0      0.0.0.0         255.255.0.0     U     0      0        0 br-0f2b158226c8
	//192.168.0.0     0.0.0.0         255.255.255.0   U     0      0        0 enp6s0
	//tom@hanalei:~$
	cmd := exec.Command("route", "-n")
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

func GetDefaultGateway() (net.IP, error) {
	myOs := runtime.GOOS
	log.Info().Msgf("OS: %s", myOs)
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
