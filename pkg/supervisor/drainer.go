package supervisor

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// DrainerMetrics is struct of prometheus metrics to be exported
type DrainerMetrics struct {
	// Drainer Metrics
	NodesCordoned   prometheus.Counter
	NodesDrained    prometheus.Counter
	NodesUncordoned prometheus.Counter
}

// InitDrainerMetrics initializes these metrics
func InitDrainerMetrics(prefix string) *DrainerMetrics {
	dm := DrainerMetrics{
		NodesCordoned: promauto.NewCounter(prometheus.CounterOpts{
			Name: prefix + "_nodes_cordoned",
			Help: "Number of nodes that were cordoned by node refiner",
		}),
		NodesDrained: promauto.NewCounter(prometheus.CounterOpts{
			Name: prefix + "_nodes_drained",
			Help: "Number of nodes that were drained by node refiner",
		}),
		NodesUncordoned: promauto.NewCounter(prometheus.CounterOpts{
			Name: prefix + "_nodes_uncordoned",
			Help: "Number of nodes that were uncordoned by node refiner",
		}),
	}
	return &dm
}
