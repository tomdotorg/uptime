// src/upcheck/network_test.go
package upcheck

import (
	"net"
	"runtime"
	"testing"
)

func TestGetLocalIP(t *testing.T) {
	ip, err := GetLocalIP()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if ip == nil {
		t.Fatalf("Expected valid IP, got nil")
	}
}

func TestGetNetmask(t *testing.T) {
	ip, err := GetLocalIP()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	mask, err := GetNetmask(ip)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if mask == nil {
		t.Fatalf("Expected valid netmask, got nil")
	}
}

func TestGetDefaultGateway(t *testing.T) {
	myOs := runtime.GOOS
	if myOs != "linux" && myOs != "darwin" {
		t.Skip("Skipping test on unsupported OS")
	}

	gw, err := GetDefaultGateway()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if gw == nil {
		t.Fatalf("Expected valid gateway IP, got nil")
	}
}

func TestIPMaskToString(t *testing.T) {
	mask := net.CIDRMask(24, 32)
	maskStr := IPMaskToString(mask)
	expected := "255.255.255.0"
	if maskStr != expected {
		t.Fatalf("Expected %v, got %v", expected, maskStr)
	}
}

func TestIsInSameSubnet(t *testing.T) {
	ip1 := net.ParseIP("192.168.1.1")
	ip2 := net.ParseIP("192.168.1.2")
	ip3 := net.ParseIP("10.0.0.1")
	mask := net.CIDRMask(24, 32)

	if !IsInSameSubnet(ip1, mask, ip2) {
		t.Fatalf("Expected %v and %v to be in the same subnet", ip1, ip2)
	}
	if IsInSameSubnet(ip1, mask, ip3) {
		t.Fatalf("Expected %v and %v to not be in the same subnet", ip1, ip3)
	}
}
