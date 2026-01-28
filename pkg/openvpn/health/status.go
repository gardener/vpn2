// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package health

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/netip"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-logr/logr"

	"github.com/gardener/vpn2/pkg/openvpn"
)

type OpenVPNStatus struct {
	Version      string
	UpdatedAt    time.Time
	Clients      []ClientInfo
	RoutingTable []RoutingEntry
	GlobalStats  map[string]string
}

type ClientInfo struct {
	CommonName         string
	RealAddress        netip.AddrPort
	VirtualAddress     string
	VirtualIPv6Address net.IP
	BytesReceived      uint64
	BytesSent          uint64
	ConnectedSince     time.Time
	Username           string
	ClientID           string
	PeerID             string
	DataChannelCipher  string
}

type RoutingEntry struct {
	VirtualAddress string
	CommonName     string
	RealAddress    netip.AddrPort
	LastRef        time.Time
}

func ParseFile(filePath string) (*OpenVPNStatus, error) {
	dirName, fileName := path.Split(filePath)
	file, err := os.OpenInRoot(dirName, fileName)

	if err != nil {
		return nil, err
	}
	defer file.Close()

	return ParseOpenVPNStatus(file)
}

func ParseOpenVPNStatus(reader io.Reader) (*OpenVPNStatus, error) {
	status := &OpenVPNStatus{
		Clients:      []ClientInfo{},
		RoutingTable: []RoutingEntry{},
		GlobalStats:  make(map[string]string),
	}

	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || line == "END" {
			continue
		}

		parts := strings.Split(line, ",")
		if len(parts) == 0 {
			continue
		}

		switch parts[0] {
		case "TITLE":
			status.Version = strings.Join(parts[1:], ",")
		case "TIME":
			if len(parts) >= 2 {
				updatedAt, err := time.Parse(time.DateTime, parts[1])
				if err != nil {
					return nil, err
				}
				status.UpdatedAt = updatedAt
			} else {
				return nil, fmt.Errorf("invalid TIME line: %s", line)
			}
		case "CLIENT_LIST":
			if len(parts) >= 13 {
				connectedSince, err := time.Parse(time.DateTime, parts[7])
				if err != nil {
					return nil, err
				}
				ipv6 := net.ParseIP(parts[4])
				if ipv6 == nil {
					return nil, fmt.Errorf("CLIENT_LIST: can't parse virtual client address: %s", parts[4])
				}
				realAddress, err := parseRealClientAddress(parts[2])
				if err != nil {
					return nil, fmt.Errorf("CLIENT_LIST: can't parse real client address: %s (%w)", parts[2], err)
				}
				bytesReceived, err := strconv.ParseUint(parts[5], 10, 64)
				if err != nil {
					return nil, err
				}
				bytesSent, err := strconv.ParseUint(parts[6], 10, 64)
				if err != nil {
					return nil, err
				}

				status.Clients = append(status.Clients, ClientInfo{
					CommonName:         parts[1],
					RealAddress:        realAddress,
					VirtualAddress:     parts[3],
					VirtualIPv6Address: ipv6,
					BytesReceived:      bytesReceived,
					BytesSent:          bytesSent,
					ConnectedSince:     connectedSince,
					Username:           parts[9],
					ClientID:           parts[10],
					PeerID:             parts[11],
					DataChannelCipher:  parts[12],
				})
			} else {
				return nil, fmt.Errorf("invalid CLIENT_LIST line: %s", line)
			}
		case "ROUTING_TABLE":
			if len(parts) >= 5 {
				lastRef, err := time.Parse(time.DateTime, parts[4])
				if err != nil {
					return nil, err
				}
				realAddress, err := parseRealClientAddress(parts[3])
				if err != nil {
					return nil, fmt.Errorf("ROUTING_TABLE: can't parse real client address: %s (%w)", parts[3], err)
				}

				status.RoutingTable = append(status.RoutingTable, RoutingEntry{
					VirtualAddress: parts[1],
					CommonName:     parts[2],
					RealAddress:    realAddress,
					LastRef:        lastRef,
				})
			} else {
				return nil, fmt.Errorf("invalid ROUTING_TABLE line: %s", line)
			}
		case "GLOBAL_STATS":
			if len(parts) >= 2 {
				status.GlobalStats[parts[1]] = strings.Join(parts[2:], ",")
			} else {
				return nil, fmt.Errorf("invalid GLOBAL_STATS line: %s", line)
			}
		case "HEADER":
			// Ignore header lines
		default:
			return nil, fmt.Errorf("unknown line type: %s", line)
		}
	}

	return status, scanner.Err()
}

// isUp checks if the OpenVPN server is considered "up" based on the last update time.
func isUp(log logr.Logger, status *OpenVPNStatus, updateInterval int) bool {
	if status == nil {
		return false
	}
	lastUpdate := time.Since(status.UpdatedAt)
	expectedUpdate := time.Duration(updateInterval+2) * time.Second
	alive := lastUpdate <= expectedUpdate
	if !alive {
		log.Info("OpenVPN status is stale", "lastUpdate", lastUpdate.String(), "expectedUpdate", expectedUpdate.String())
	}
	// We assume OpenVPN is dead if it hasn't been updated in updateInterval + 2 seconds
	return alive
}

// isReady checks if the OpenVPN server is considered "ready" based on the number of connected clients.
func isReady(log logr.Logger, status *OpenVPNStatus, isHA bool) bool {
	if status == nil {
		log.Info("OpenVPN status is nil", "isHA", isHA)
		return false
	}

	// In HA-mode we need at least 2 clients (one from seed, one from shoot)
	if isHA {
		// We need at least 2 connected clients to be considered ready
		if len(status.Clients) < 2 {
			log.Info("Not enough connected clients for HA mode", "connectedClients", len(status.Clients), "isHA", isHA)
			return false
		}
		// We need at least 1 client from the seed and one from the shoot side
		foundSeedClient := false
		foundShootClient := false
		for _, client := range status.Clients {
			if strings.HasPrefix(client.CommonName, openvpn.SeedClientPrefix) {
				foundSeedClient = true
				continue
			}
			if strings.HasPrefix(client.CommonName, openvpn.ShootClientPrefix) {
				foundShootClient = true
				continue
			}
		}

		ready := foundSeedClient && foundShootClient
		if !ready {
			log.Info("Missing required clients for HA mode", "foundSeedClient", foundSeedClient, "foundShootClient", foundShootClient, "isHA", isHA)
		}

		return ready
	}

	// In non-HA mode there are two cases:
	// - No shoot clients connected yet after deployment rollout. This is considered ready as the shoot will connect later.
	// - At least one shoot client connected. This is considered ready.
	if len(status.Clients) == 0 {
		return true
	}
	for _, client := range status.Clients {
		if strings.HasPrefix(client.CommonName, openvpn.ShootClientPrefix) {
			return true
		}
	}
	log.Info("no shoot client connected yet", "connectedClients", len(status.Clients), "isHA", isHA)
	return false
}

// parseRealClientAddress parses a real client address string in the format "IP:Port".
func parseRealClientAddress(addrStr string) (netip.AddrPort, error) {
	// In OpenVPN 2.6 the address can be in two different formats:
	// - IPv4:Port (e.g., 192.168.0.1:1234
	// - IPv6 (e.g., 2001:db8::1) (no port)
	// In OpenVPN 2.7 the format is always IP:Port, even for IPv6 (e.g., [2001:db8::1]:1234) but it may be prefixed by
	// the protocol (e.g., udp6:[2001:db8::1]:1234)
	// See https://github.com/OpenVPN/openvpn/issues/963

	// Parse using regexp to handle both formats
	regex := `^(?:(?:udp|tcp)(?:4|6)?\:)?(\[?[a-fA-F0-9:.]+\]?)?(?::(\d+))?$`
	re := regexp.MustCompile(regex)
	matches := re.FindStringSubmatch(addrStr)
	if len(matches) == 0 {
		return netip.AddrPort{}, fmt.Errorf("invalid address format: %s", addrStr)
	}

	ipStr := matches[1]
	portStr := matches[2]

	// [2001:db8::1]:1234 case
	if portStr != "" {
		return netip.ParseAddrPort(fmt.Sprintf("%s:%s", ipStr, portStr))
	}

	// 192.168.0.1:1234 case
	addrPort, err := netip.ParseAddrPort(ipStr)
	if err != nil {
		// No port case (OpenVPN 2.6 IPv6 without port)
		ip, err := netip.ParseAddr(ipStr)
		if err != nil {
			return netip.AddrPort{}, err
		}
		return netip.AddrPortFrom(ip, 0), nil
	}
	return addrPort, nil
}
