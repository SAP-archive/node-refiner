package supervisor

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

// Supervisor is an object to manage the exported prometheus metrics
type Supervisor struct {
	Prefix string

	DrainerMetrics *DrainerMetrics
	ClusterMetrics *ClusterMetrics
}

// InitSupervisor initializes the supervisor using the two contexts, drainer metrics and cluster metrics
func InitSupervisor(prefix string) *Supervisor {
	s := Supervisor{
		Prefix:         prefix,
		DrainerMetrics: InitDrainerMetrics(prefix),
		ClusterMetrics: InitClusterMetrics(prefix),
	}
	return &s
}

// StartSupervising opens a web port that can be used by prometheus to track the metrics we are exposing
func (s *Supervisor) StartSupervising() {
	go s.startLiveness()
	go s.startPrometheus()
}

func (s *Supervisor) startLiveness() {
	livenessPort := "9102"
	health := Handler{MaxLoopTime: 60 * time.Second}
	zap.S().Infof("starting liveness monitor at %s", livenessPort)
	http.Handle("/alive", &health)
	Check(http.ListenAndServe(fmt.Sprintf(":%s", livenessPort), nil))
}

func (s *Supervisor) startPrometheus() {
	// Setup Prometheus Metrics

	port := os.Getenv("LISTENING_PORT")

	if port == "" {
		port = "8080"
	}

	http.Handle("/metrics", promhttp.Handler())
	zap.S().Infow("Started serving metrics on /metrics")
	zap.S().Infow("listening on", "port", port)
	zap.S().Fatal(http.ListenAndServe(":"+port, nil))

}
