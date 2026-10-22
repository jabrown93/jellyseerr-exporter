package collector

import "github.com/prometheus/client_golang/prometheus"

const (
	namespace = "jellyseerr"
	upHelp    = "Whether the last Jellyseerr API query for this collector succeeded (1) or failed (0)."
)

func newUpDesc(collector string) *prometheus.Desc {
	return prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "up"),
		upHelp,
		nil,
		prometheus.Labels{"collector": collector},
	)
}
