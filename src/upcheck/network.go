package upcheck

import (
	"bytes"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/jackpal/gateway"
	"github.com/rs/zerolog/log"
)

type NetworkInfo struct {
	Address net.IP
	Mask    net.IPMask
	GW      net.IP
}

func (n NetworkInfo) Equals(o *NetworkInfo) bool {
	return n.Address.Equal(o.Address) && bytes.Equal(n.Mask, o.Mask) && n.GW.Equal(o.GW)
}

func (n NetworkInfo) String() string {
	return fmt.Sprintf("Address: %s\nMask: %s\nGW: %s", n.Address, net.IP(n.Mask), n.GW)
}

func GetNetworkInfo() (netInfo *NetworkInfo, err error) {
	iface, err := gateway.DiscoverInterface()
	if err != nil {
		return nil, err
	}
	log.Debug().Msgf("GetNetworkInfo(): interface: %v %s\n", iface, net.IP(iface.DefaultMask()).String())
	gw, err := gateway.DiscoverGateway()
	if err != nil {
		return nil, err
	}
	log.Debug().Msgf("GetNetworkInfo(): default gateway: %v\n", gw)
	return &NetworkInfo{Address: iface, Mask: iface.DefaultMask(), GW: gw}, nil
}

func IsInSameSubnet(baseIP net.IP, mask net.IPMask, checkIP net.IP) bool {
	baseNetwork := baseIP.Mask(mask)
	checkNetwork := checkIP.Mask(mask)
	return baseNetwork.Equal(checkNetwork)
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
			log.Debug().Msgf("Node %s is reachable on port %s\n", address, port)
			return strconv.Atoi(port)
		}
	}
	return -1, nil
}
