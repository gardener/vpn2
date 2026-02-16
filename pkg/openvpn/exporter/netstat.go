package exporter

import (
	"fmt"
	"regexp"
	"strconv"

	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/gardener/vpn2/pkg/network"
)

// netStatFields is a regexp pattern to filter the network stats we want to expose as metrics.
var netStatFields = "^Tcp_(OutSegs|RetransSegs)$"

type netStatCollector struct {
	logger       logr.Logger
	fieldPattern *regexp.Regexp
}

// NewNetStatCollector takes and returns
// a new Collector exposing network stats.
func NewNetStatCollector(log logr.Logger) (prometheus.Collector, error) {
	pattern := regexp.MustCompile(netStatFields)
	return &netStatCollector{
		logger:       log,
		fieldPattern: pattern,
	}, nil
}

func (c *netStatCollector) Describe(ch chan<- *prometheus.Desc) {
	// No need to send any descriptors since we are using MustNewConstMetric in Collect.
}

func (c *netStatCollector) Collect(ch chan<- prometheus.Metric) {
	netStats, err := network.GetNetStats("/proc")

	if err != nil {
		c.logger.Error(err, "failed to get net stats")
		return
	}

	for protocol, protocolStats := range netStats {
		for name, value := range protocolStats {
			key := protocol + "_" + name
			v, err := strconv.ParseFloat(value, 64)
			if err != nil {
				c.logger.Error(err, "failed to parse net stat value", "key", key, "value", value)
				continue
			}
			if !c.fieldPattern.MatchString(key) {
				continue
			}
			ch <- prometheus.MustNewConstMetric(
				prometheus.NewDesc(
					prometheus.BuildFQName("openvpn", "netstat", key),
					fmt.Sprintf("Statistic %s.", protocol+name),
					nil, nil,
				),
				prometheus.UntypedValue, v,
			)
		}
	}
}
