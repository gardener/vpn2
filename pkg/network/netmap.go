// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package network

import (
	"fmt"
	"net"
	"sort"

	"github.com/gardener/vpn2/pkg/constants"
)

// NetmapIP maps the given IP address to a new IP address in the provided subnet.
func NetmapIP(ip string, subnet string) (string, error) {
	srcIp := net.ParseIP(ip)
	if srcIp == nil {
		return "", fmt.Errorf("failed to parse ip: %v", ip)
	}
	_, subnetMask, err := net.ParseCIDR(subnet)
	if err != nil {
		return "", fmt.Errorf("failed to parse subnet: %v", err)
	}
	mappedIP, err := netmapIP(srcIp, *subnetMask)
	if err != nil {
		return "", err
	}
	return mappedIP.String(), nil
}

func netmapIP(ip net.IP, subnet net.IPNet) (net.IP, error) {
	// Ensure the IP is in the correct format (IPv4 or IPv6)
	ipv4 := ip.To4()
	subnetBase := subnet.IP.To4()
	if ipv4 == nil || subnetBase == nil {
		return nil, fmt.Errorf("failed to map ip %s to subnet %s. only IPv4 is supported for mapping", ip.String(), subnet.String())
	}

	// Apply the subnet mask to get the base IP address
	mappedIP := make(net.IP, len(ipv4))
	for i := range ipv4 {
		mappedIP[i] = (ipv4[i] & ^subnet.Mask[i]) | subnetBase[i]
	}

	return mappedIP, nil
}

// subnetSplit splits a parent *net.IPNet into all subnets of the given newPrefixLen length.
func subnetSplit(parent *net.IPNet, newPrefixLen int) ([]*net.IPNet, error) {
	parentPrefixLen, bits := parent.Mask.Size()
	if newPrefixLen < parentPrefixLen || newPrefixLen > bits {
		return nil, fmt.Errorf("invalid new prefix length %d for parent subnet %s", newPrefixLen, parent.String())
	}
	var (
		subnets      []*net.IPNet
		countSubnets uint32
		numSubnets   uint32
		baseInt      uint32
		base         net.IP
		incr         uint32
	)

	numSubnets = 1 << (newPrefixLen - parentPrefixLen)
	base = parent.IP.Mask(parent.Mask).To4()
	if base == nil {
		return nil, fmt.Errorf("only IPv4 is supported")
	}
	baseInt = uint32(base[0])<<24 | uint32(base[1])<<16 | uint32(base[2])<<8 | uint32(base[3])
	subnets = make([]*net.IPNet, numSubnets)
	incr = 1 << (32 - newPrefixLen)

	for countSubnets = 0; countSubnets < numSubnets; countSubnets++ {
		ipInt := baseInt + countSubnets*incr
		ip := net.IPv4(
			byte(ipInt>>24),
			byte(ipInt>>16),
			byte(ipInt>>8),
			byte(ipInt),
		)
		mask := net.CIDRMask(newPrefixLen, bits)
		subnets[countSubnets] = &net.IPNet{IP: ip.Mask(mask), Mask: mask}
	}
	return subnets, nil
}

func netmapSubnet(srcNet net.IPNet, dstNet net.IPNet) (net.IPNet, error) {
	// Ensure the source and destination networks are both IPv4
	if srcNet.IP.To4() == nil || dstNet.IP.To4() == nil {
		return net.IPNet{}, fmt.Errorf("only IPv4 is supported for subnet mapping")
	}

	// Map the source network IP to the destination network
	mappedIP, err := netmapIP(srcNet.IP, dstNet)
	if err != nil {
		return net.IPNet{}, err
	}

	// Create a new IPNet with the mapped IP and the destination mask
	return net.IPNet{
		IP:   mappedIP,
		Mask: dstNet.Mask,
	}, nil
}

// NetmapSubnets maps multiple source subnets to a single destination subnet.
// It returns a map where keys are source CIDRs and values are the mapped destination CIDRs.
func NetmapSubnets(srcCIDRs []string, dstCIDR string) (map[string]string, error) {
	dstIPNet, err := ParseIPNet(dstCIDR)
	if err != nil {
		return nil, fmt.Errorf("failed to parse destination CIDR: %w", err)
	}

	type srcInfo struct {
		raw    string
		ipnet  *net.IPNet
		prefix int
	}
	var srcInfos []srcInfo
	for _, src := range srcCIDRs {
		ipnet, err := ParseIPNet(src)
		if err != nil {
			return nil, fmt.Errorf("failed to parse source subnet %s: %w", src, err)
		}
		ones, bits := ipnet.Mask.Size()
		if bits != 32 {
			return nil, fmt.Errorf("only IPv4 is supported: %s", src)
		}
		srcInfos = append(srcInfos, srcInfo{raw: src, ipnet: ipnet.ToIPNet(), prefix: ones})
	}

	// Sort by decreasing prefix length (largest subnets first)
	sort.Slice(srcInfos, func(i, j int) bool {
		return srcInfos[i].prefix < srcInfos[j].prefix
	})

	// Recursive mapping function
	var mapSubnetsRec func(srcs []srcInfo, dstSubnets []*net.IPNet, result map[string]string) error
	mapSubnetsRec = func(srcs []srcInfo, dstSubnets []*net.IPNet, result map[string]string) error {
		if len(srcs) == 0 {
			return nil // All mapped
		}
		// Take the largest src subnet
		src := srcs[0]
		// Split available dst subnets into chunks of src.prefix
		var candidateSubnets []*net.IPNet
		for _, dst := range dstSubnets {
			subnets, err := subnetSplit(dst, src.prefix)
			if err != nil {
				return fmt.Errorf("failed to split destination subnet: %w", err)
			}
			candidateSubnets = append(candidateSubnets, subnets...)
		}
		if len(candidateSubnets) == 0 {
			return fmt.Errorf("not enough space in %s to fit all source subnets: %s", dstCIDR, srcCIDRs)
		}
		// Map src to the first candidate
		mapped, err := netmapSubnet(*src.ipnet, *candidateSubnets[0])
		if err != nil {
			return fmt.Errorf("failed to map subnet %s: %w", src.raw, err)
		}
		result[src.raw] = mapped.String()
		// Remove the mapped subnets and recurse
		return mapSubnetsRec(srcs[1:], candidateSubnets[1:], result)
	}

	// Start recursion with the whole dstCIDR
	result := make(map[string]string)
	err = mapSubnetsRec(srcInfos, []*net.IPNet{dstIPNet.ToIPNet()}, result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// NetmapCIDRs maps a list of source CIDRs to a destination CIDR.
// It returns a map where keys are pointers to source CIDRs and values are the mapped destination CIDRs.
func NetmapCIDRs(srcCIDRs []CIDR, dstCIDR CIDR) (map[*CIDR]CIDR, error) {
	// We convert CIDRs to strings for the NetmapSubnets function internally
	srcCIDRsStr := make([]string, len(srcCIDRs))
	for i, cidr := range srcCIDRs {
		srcCIDRsStr[i] = cidr.String()
	}
	dstCIDRStr := dstCIDR.String()

	mappedStr, err := NetmapSubnets(srcCIDRsStr, dstCIDRStr)
	if err != nil {
		return nil, fmt.Errorf("failed to map CIDRs: %w", err)
	}
	mappedCIDRs := make(map[*CIDR]CIDR, len(mappedStr))
	for src, dst := range mappedStr {
		srcCIDR, err := ParseIPNet(src)
		if err != nil {
			return nil, fmt.Errorf("failed to parse source CIDR %s: %w", src, err)
		}
		dstCIDR, err := ParseIPNet(dst)
		if err != nil {
			return nil, fmt.Errorf("failed to parse destination CIDR %s: %w", dst, err)
		}
		mappedCIDRs[&srcCIDR] = dstCIDR
	}
	return mappedCIDRs, nil
}

// ShootNetworksForNetmap returns the mappings of shoot pod, service, and node networks to subnets of their reserved mapping ranges.
// This will work for an arbitrary number of networks as along as they fit into the reserved ranges.
// Example: node networks of 10.100.80.0/16, 10.99.14.0/24, 10.99.15.0/24 will be mapped to
// 242.0.0.0/16, 242.1.0.0/24, 242.1.1.0/24 all fitting into 242.0.0.0/8 and non-overlapping with each other.
func ShootNetworksForNetmap(
	ShootPodNetworks, ShootServiceNetworks, ShootNodeNetworks []CIDR,
) (ipv4PodNetworkMappings, ipv4ServiceNetworkMappings, ipv4NodeNetworkMappings map[*CIDR]CIDR, err error) {
	ipv4PodNetworks := GetByIPFamily(ShootPodNetworks, IPv4Family)
	ipv4ServiceNetworks := GetByIPFamily(ShootServiceNetworks, IPv4Family)
	ipv4NodeNetworks := GetByIPFamily(ShootNodeNetworks, IPv4Family)

	ipv4PodNetworkMappings, err = NetmapCIDRs(ipv4PodNetworks, ParseIPNetIgnoreError(constants.ShootPodNetworkMapped))
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to map shoot pod networks: %w", err)
	}

	ipv4ServiceNetworkMappings, err = NetmapCIDRs(ipv4ServiceNetworks, ParseIPNetIgnoreError(constants.ShootServiceNetworkMapped))
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to map shoot service networks: %w", err)
	}

	ipv4NodeNetworkMappings, err = NetmapCIDRs(ipv4NodeNetworks, ParseIPNetIgnoreError(constants.ShootNodeNetworkMapped))
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to map shoot node networks: %w", err)
	}

	return ipv4PodNetworkMappings, ipv4ServiceNetworkMappings, ipv4NodeNetworkMappings, nil
}
