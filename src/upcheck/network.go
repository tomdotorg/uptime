package upcheck

import (
	"bytes"
	"fmt"
	"net"

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
	iface, err := gateway.DiscoverInterface()
	if err != nil {
		return NetworkInfo{}, err
	}
	log.Debug().Msgf("interface: %v %s\n", iface, net.IP(iface.DefaultMask()).String())
	gw, err := gateway.DiscoverGateway()
	if err != nil {
		return NetworkInfo{}, err
	}
	log.Debug().Msgf("Default Gateway: %v\n", gw)
	return NetworkInfo{Address: iface, Mask: iface.DefaultMask(), GW: gw}, nil
}

func IsInSameSubnet(baseIP net.IP, mask net.IPMask, checkIP net.IP) bool {
	baseNetwork := baseIP.Mask(mask)
	checkNetwork := checkIP.Mask(mask)
	return baseNetwork.Equal(checkNetwork)
}
