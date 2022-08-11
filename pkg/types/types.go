package types

import (
	"github.com/SAP/node-refiner/pkg/common"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// Weight for calculating utilization scores
// design decision is to make CPU usage weigh more in score calculation
const (
	cpuWeight = 0.8
	ramWeight = 0.2
)

// ClusterManifest Overall cluster metrics
type ClusterManifest struct {
	ExcessNodes             float64
	NumberOfNonTaintedNodes int
	NumberOfNodes           int
	NumberOfPods            int
	TotalNodeMetrics        NodeMetrics
	TotalPodsMetrics        PodMetrics
	Utilization             Utilization
}

// NodeManifest meta-data of the node + the metrics of our concern
type NodeManifest struct {
	Node              *v1.Node
	Metrics           *NodeMetrics
	NumberOfPods      int
	TotalPodsRequests PodMetrics
	Pods              []*PodManifest
	Utilization       Utilization
}

// NodeMetrics allocatable cpu and ram of a node
type NodeMetrics struct {
	AllocCPU resource.Quantity
	AllocRAM resource.Quantity
}

// PodManifest meta-data of the pod + the metrics of our concern
type PodManifest struct {
	Pod     *v1.Pod
	Metrics *PodMetrics
}

// PodMetrics requests cpu and ram of a pod
type PodMetrics struct {
	ReqCPU resource.Quantity
	ReqRAM resource.Quantity
}

// Utilization percentage of total requests on a node over its allocatable
type Utilization struct {
	PercentageCPU float64
	PercentageRAM float64
	Score         float64
}

// NewClusterManifest creates a new cluster manifest object from a map of NodeManifest
func NewClusterManifest(nodesMap map[string]NodeManifest) ClusterManifest {
	clusterManifest := ClusterManifest{}
	for _, nodeManifest := range nodesMap {
		if !common.CheckForTaints(nodeManifest.Node) {
			clusterManifest.NumberOfNonTaintedNodes++
			clusterManifest.TotalPodsMetrics.AddPodMetrics(&nodeManifest.TotalPodsRequests)
			clusterManifest.TotalNodeMetrics.AddNodeMetrics(nodeManifest.Metrics)
		}
		clusterManifest.NumberOfPods += len(nodeManifest.Pods)

	}
	clusterManifest.Utilization = CalculateUtilizationPercentage(&clusterManifest.TotalPodsMetrics, &clusterManifest.TotalNodeMetrics)
	clusterManifest.NumberOfNodes = len(nodesMap)

	return clusterManifest
}

// NewPodManifest create a PodManifest by extracting the relevant information from a Pod object
func NewPodManifest(pod *v1.Pod) PodManifest {
	pm := PodManifest{
		Pod:     pod,
		Metrics: CreatePodMetricsFromPodObj(pod),
	}
	return pm
}

// CreateNodeMetricsFromNodeObj create a NodeMetrics object by extracting the relevant information from a Node object
func CreateNodeMetricsFromNodeObj(node *v1.Node) *NodeMetrics {
	status := node.Status
	nm := NodeMetrics{
		AllocCPU: *status.Allocatable.Cpu(),
		AllocRAM: *status.Allocatable.Memory(),
	}
	return &nm
}

// CreatePodMetricsFromPodObj create a PodMetrics object by extracting the relevant information from a Pod object
func CreatePodMetricsFromPodObj(pod *v1.Pod) *PodMetrics {
	pm := PodMetrics{}
	for _, container := range pod.Spec.Containers {
		// Aggregating for the pod resources
		pm.AddPodContainerResources(&container)
	}
	return &pm
}

// CalculateUtilizationPercentage do the arithmetics to create a utilization metrics for a node
func CalculateUtilizationPercentage(podMetrics *PodMetrics, nodeMetrics *NodeMetrics) Utilization {
	utilization := Utilization{}
	if nodeMetrics.AllocCPU.MilliValue() != 0 && nodeMetrics.AllocRAM.MilliValue() != 0 {
		utilization.PercentageCPU = float64(podMetrics.ReqCPU.MilliValue()) / float64(nodeMetrics.AllocCPU.MilliValue()) * 100
		utilization.PercentageRAM = float64(podMetrics.ReqRAM.MilliValue()) / float64(nodeMetrics.AllocRAM.MilliValue()) * 100
	} else {
		return utilization
	}
	utilization.Score = (utilization.PercentageCPU * cpuWeight) + (utilization.PercentageRAM * ramWeight)
	return utilization
}

// CalculateExcessNode divide the remaining unused resources by a sample node to know the excess nodes
func (cm *ClusterManifest) CalculateExcessNode(sampleNode *NodeManifest) {
	fractionExcessCPU := float64(0)
	fractionExcessRAM := float64(0)

	fractionExcessCPU = float64(cm.TotalNodeMetrics.AllocCPU.MilliValue()-cm.TotalPodsMetrics.ReqCPU.MilliValue()) / float64(sampleNode.Metrics.AllocCPU.MilliValue())
	fractionExcessRAM = float64(cm.TotalNodeMetrics.AllocRAM.MilliValue()-cm.TotalPodsMetrics.ReqRAM.MilliValue()) / float64(sampleNode.Metrics.AllocRAM.MilliValue())

	if fractionExcessCPU < fractionExcessRAM {
		cm.ExcessNodes = fractionExcessCPU
	} else {
		cm.ExcessNodes = fractionExcessRAM
	}
}

// IncPods increment number of pods in a NodeManifest by 1
func (nm *NodeManifest) IncPods() {
	nm.NumberOfPods++
}

// AddPodMetrics add PodMetrics to an existing PodMetrics
func (pm *PodMetrics) AddPodMetrics(pmNew *PodMetrics) {
	pm.ReqCPU.Add(pmNew.ReqCPU)
	pm.ReqRAM.Add(pmNew.ReqRAM)
}

// AddNodeMetrics add AddNodeMetrics to an existing AddNodeMetrics
func (nm *NodeMetrics) AddNodeMetrics(nmNew *NodeMetrics) {
	nm.AllocCPU.Add(nmNew.AllocCPU)
	nm.AllocRAM.Add(nmNew.AllocRAM)
}

// AddPodContainerResources from *v1.Pod.Spec.Containers to Metrics Object - Requests and Limits Resources
func (pm *PodMetrics) AddPodContainerResources(container *v1.Container) {
	pm.ReqCPU.Add(*container.Resources.Requests.Cpu())
	pm.ReqRAM.Add(*container.Resources.Requests.Memory())
}
