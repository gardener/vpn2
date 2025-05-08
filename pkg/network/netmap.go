package network

import (
	"fmt"
	"net"
)

// Netmap maps the given IP address to a new IP address in the provided subnet.
func Netmap(ip string, subnet string) (string, error) {
	srcIp := net.ParseIP(ip)
	_, subnetMask, err := net.ParseCIDR(subnet)
	if err != nil {
		return "", fmt.Errorf("failed to parse subnet: %v", err)
	}
	mappedIP, err := netmap(srcIp, subnetMask)
	if err != nil {
		return "", err
	}
	return mappedIP.String(), nil
}

func netmap(ip net.IP, subnet *net.IPNet) (net.IP, error) {
	// Ensure the IP is in the correct format (IPv4 or IPv6)
	ip = ip.To4()
	subnetBase := subnet.IP.To4()
	if ip == nil || subnetBase == nil {
		return nil, fmt.Errorf("only IPv4 is supported for mapping")
	}

	// Apply the subnet mask to get the base IP address
	mappedIP := make(net.IP, len(ip))
	for i := range ip {
		mappedIP[i] = (ip[i] & ^subnet.Mask[i]) | subnetBase[i]
	}

	return mappedIP, nil
}
