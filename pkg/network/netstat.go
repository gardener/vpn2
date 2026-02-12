package network

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// inspired by: https://github.com/prometheus/node_exporter/blob/master/collector/netstat_linux.go

// GetNetStats returns a map of protocol to a map of stat name to value, by parsing the files in /proc/net.
func GetNetStats(procPath string) (map[string]map[string]string, error) {
	netStats, err := getNetStats(filepath.Join(procPath, "net/netstat"))
	if err != nil {
		return nil, fmt.Errorf("couldn't get netstats: %w", err)
	}
	snmpStats, err := getNetStats(filepath.Join(procPath, "net/snmp"))
	if err != nil {
		return nil, fmt.Errorf("couldn't get SNMP stats: %w", err)
	}
	snmp6Stats, err := getSNMP6Stats(filepath.Join(procPath, "net/snmp6"))
	if err != nil {
		return nil, fmt.Errorf("couldn't get SNMP6 stats: %w", err)
	}
	// Merge the results of snmpStats into netStats (collisions are possible, but
	// we know that the keys are always unique for the given use case).
	maps.Copy(netStats, snmpStats)
	maps.Copy(netStats, snmp6Stats)
	return netStats, nil
}

func getNetStats(filePath string) (map[string]map[string]string, error) {
	dirName, fileName := path.Split(filePath)
	file, err := os.OpenInRoot(dirName, fileName)

	if err != nil {
		return nil, err
	}
	defer file.Close()

	return parseNetStats(file, fileName)
}

func parseNetStats(r io.Reader, fileName string) (map[string]map[string]string, error) {
	var (
		netStats = map[string]map[string]string{}
		scanner  = bufio.NewScanner(r)
	)

	for scanner.Scan() {
		nameParts := strings.Split(scanner.Text(), " ")
		if ok := scanner.Scan(); !ok {
			return nil, fmt.Errorf("missing value line in %s for %s", fileName, nameParts[0])
		}
		valueParts := strings.Split(scanner.Text(), " ")
		if nameParts[0] != valueParts[0] {
			return nil, fmt.Errorf("header to value mismatch in %s: %s vs %s",
				fileName, nameParts[0], valueParts[0])
		}
		// Remove trailing :.
		protocol := nameParts[0][:len(nameParts[0])-1]
		netStats[protocol] = map[string]string{}
		if len(nameParts) != len(valueParts) {
			return nil, fmt.Errorf("mismatch field count mismatch in %s: %s",
				fileName, protocol)
		}
		for i := 1; i < len(nameParts); i++ {
			netStats[protocol][nameParts[i]] = valueParts[i]
		}
	}

	return netStats, scanner.Err()
}

func getSNMP6Stats(filePath string) (map[string]map[string]string, error) {
	dirName, fileName := path.Split(filePath)
	file, err := os.OpenInRoot(dirName, fileName)
	if err != nil {
		// On systems with IPv6 disabled, this file won't exist.
		// Do nothing.
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}

		return nil, err
	}
	defer file.Close()

	return parseSNMP6Stats(file)
}

func parseSNMP6Stats(r io.Reader) (map[string]map[string]string, error) {
	var (
		netStats = map[string]map[string]string{}
		scanner  = bufio.NewScanner(r)
	)

	for scanner.Scan() {
		stat := strings.Fields(scanner.Text())
		if len(stat) != 2 {
			continue
		}
		// Expect to have "6" in metric name, skip line otherwise
		if sixIndex := strings.Index(stat[0], "6"); sixIndex != -1 {
			protocol := stat[0][:sixIndex+1]
			name := stat[0][sixIndex+1:]
			if _, present := netStats[protocol]; !present {
				netStats[protocol] = map[string]string{}
			}
			netStats[protocol][name] = stat[1]
		}
	}

	return netStats, scanner.Err()
}
