package upcheck

import (
	"bytes"
	"fmt"
	"net"
	"strings"

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
	return fmt.Sprintf("Address: %s\nMask: %s\nGW: %s", n.Address, net.IP(n.Mask), n.GW)
}

func GetNetworkInfo() (netInfo NetworkInfo, err error) {
	ifs, err := gateway.DiscoverInterface()
	if err != nil {
		log.Error().Msgf("Error getting interfaces: %v", err)
		return NetworkInfo{}, err
	}
	log.Debug().Msgf("interface: %v %s\n", ifs, net.IP(ifs.DefaultMask()).String())
	gw, err := gateway.DiscoverGateway()
	if err != nil {
		log.Error().Msgf("Error getting gateway: %v", err)
		return NetworkInfo{}, err
	}
	log.Debug().Msgf("Default Gateway: %v\n", gw)
	return NetworkInfo{Address: ifs, Mask: ifs.DefaultMask(), GW: gw}, nil
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
