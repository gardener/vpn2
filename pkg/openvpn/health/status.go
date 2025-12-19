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
	"strconv"
	"strings"
	"time"

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
					return nil, fmt.Errorf("invalid IPv6 address: %s", parts[4])
				}
				realAddress, err := netip.ParseAddrPort(parts[2])
				if err != nil {
					return nil, err
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
				realAddress, err := netip.ParseAddrPort(parts[3])
				if err != nil {
					return nil, err
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
func isUp(status *OpenVPNStatus, updateInterval int) bool {
	if status == nil {
		return false
	}
	// We assume OpenVPN is dead if it hasn't been updated in updateInterval + 2 seconds
	return time.Since(status.UpdatedAt) <= time.Duration(updateInterval+2)*time.Second
}

// isReady checks if the OpenVPN server is considered "ready" based on the number of connected clients.
func isReady(status *OpenVPNStatus, isHA bool) bool {
	if status == nil {
		return false
	}

	// In HA-mode we need at least 2 clients (one from seed, one from shoot)
	if isHA {
		// We need at least 2 connected clients to be considered ready
		if len(status.Clients) < 2 {
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

		return foundSeedClient && foundShootClient
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
	return false
}
