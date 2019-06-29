package prom

import (
	"strconv"

	"github.com/bio-routing/bio-rd/routingtable/vrf"
	"github.com/bio-routing/bio-rd/routingtable/vrf/metrics"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	prefix = "bio_vrf_"
)

var (
	routeCountDesc *prometheus.Desc
)

func init() {
	labels := []string{"vrf", "rib", "afi", "safi"}
	routeCountDesc = prometheus.NewDesc(prefix+"route_count", "Number of routes in the RIB", labels, nil)
}

// NewCollector creates a new collector instance for the given BGP server
func NewCollector(r *vrf.VRFRegistry) prometheus.Collector {
	return &vrfCollector{
		registry: r,
	}
}

// vrfCollector provides a collector for VRF metrics of BIO to use with Prometheus
type vrfCollector struct {
	registry *vrf.VRFRegistry
}

// Describe conforms to the prometheus collector interface
func (c *vrfCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- routeCountDesc
}

func Describe(ch chan<- *prometheus.Desc) {
	ch <- routeCountDesc
}

// Collect conforms to the prometheus collector interface
func (c *vrfCollector) Collect(ch chan<- prometheus.Metric) {
	for _, v := range vrf.Metrics(c.registry) {
		c.collectForVRF(ch, v)
	}
}

func (c *vrfCollector) collectForVRF(ch chan<- prometheus.Metric, v *metrics.VRFMetrics) {
	CollectForVRF(ch, v)
}

func CollectForVRF(ch chan<- prometheus.Metric, v *metrics.VRFMetrics) {
	for _, rib := range v.RIBs {
		ch <- prometheus.MustNewConstMetric(routeCountDesc, prometheus.GaugeValue, float64(rib.RouteCount),
			v.Name, rib.Name, strconv.Itoa(int(rib.AFI)), strconv.Itoa(int(rib.SAFI)))
	}
}
