package supervisor

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.wdf.sap.corp/ICN-ML/aicore/operators/node-harvester/pkg/types"
)

// ClusterMetrics is struct of prometheus metrics to be exported
type ClusterMetrics struct {
	ExcessNodes             prometheus.Gauge
	NumberOfNonTaintedNodes prometheus.Gauge
	NumberOfNodes           prometheus.Gauge
	NumberOfPods            prometheus.Gauge
	UnschedulableNodes      prometheus.Gauge
	CPUUtilization          prometheus.Gauge
	RAMUtilization          prometheus.Gauge
}

// InitClusterMetrics initializes these metrics
func InitClusterMetrics(prefix string) *ClusterMetrics {
	cm := ClusterMetrics{
		ExcessNodes: promauto.NewGauge(prometheus.GaugeOpts{
			Name: prefix + "_cluster_excess_nodes",
			Help: "Number of excess nodes in the cluster",
		}),
		NumberOfNonTaintedNodes: promauto.NewGauge(prometheus.GaugeOpts{
			Name: prefix + "_cluster_non_tainted_nodes",
			Help: "Total number of non tainted nodes in the cluster",
		}),
		NumberOfNodes: promauto.NewGauge(prometheus.GaugeOpts{
			Name: prefix + "_cluster_nodes",
			Help: "Total number of nodes in the cluster",
		}),
		NumberOfPods: promauto.NewGauge(prometheus.GaugeOpts{
			Name: prefix + "_cluster_pods",
			Help: "Total number of pods in the cluster",
		}),
		CPUUtilization: promauto.NewGauge(prometheus.GaugeOpts{
			Name: prefix + "_cluster_cpu_utilization",
			Help: "Overall utilization of CPU resources in the cluster",
		}),
		RAMUtilization: promauto.NewGauge(prometheus.GaugeOpts{
			Name: prefix + "_cluster_memory_utilization",
			Help: "Overall utilization of memory resources in the cluster",
		}),
		UnschedulableNodes: promauto.NewGauge(prometheus.GaugeOpts{
			Name: prefix + "_cluster_unschedulable_nodes",
			Help: "Number of nodes that are unschedulable",
		}),
	}
	return &cm
}

// PublishClusterMetrics updates the exported metrics
func (cm *ClusterMetrics) PublishClusterMetrics(clusterState *types.ClusterManifest) {
	cm.ExcessNodes.Set(clusterState.ExcessNodes)
	cm.NumberOfNonTaintedNodes.Set(float64(clusterState.NumberOfNonTaintedNodes))
	cm.NumberOfNodes.Set(float64(clusterState.NumberOfNodes))
	cm.NumberOfPods.Set(float64(clusterState.NumberOfPods))
	cm.CPUUtilization.Set(clusterState.Utilization.PercentageCPU)
	cm.RAMUtilization.Set(clusterState.Utilization.PercentageRAM)
}

// PublishNodeUnschedulable updates the number of unschedulable nodes
func (cm *ClusterMetrics) PublishNodeUnschedulable(nodesMap map[string]types.NodeManifest) {
	unschedulableNodes := float64(0)
	for _, nodeManifest := range nodesMap {
		if nodeManifest.Node.Spec.Unschedulable {
			unschedulableNodes++
		}
	}
	cm.UnschedulableNodes.Set(unschedulableNodes)
}
