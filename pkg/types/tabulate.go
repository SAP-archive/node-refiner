package types

import (
	"fmt"
	"os"

	"github.com/jedib0t/go-pretty/table"
	"github.wdf.sap.corp/ICN-ML/aicore/operators/node-harvester/pkg/common"
)

// TabulateNodeMap Print the Nodes Metrics in a Table
func TabulateNodeMap(nodesMap map[string]NodeManifest) {
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{"Node", "Tainted", "Pods", "CPU Pods Requests", "Memory Pods Requests", "CPU Allocatable", "Memory Allocatable", "% CPU", "% Memory", "Score"})

	for nodeName, nodeManifest := range nodesMap {
		t.AppendRow(table.Row{
			nodeName,
			func() string {
				if common.CheckForTaints(nodeManifest.Node) {
					return "yes"
				}
				return "no"
			}(),
			len(nodeManifest.Pods),
			common.FormatValue("CPU", nodeManifest.TotalPodsRequests.ReqCPU), common.FormatValue("RAM", nodeManifest.TotalPodsRequests.ReqRAM),
			common.FormatValue("CPU", nodeManifest.Metrics.AllocCPU), common.FormatValue("RAM", nodeManifest.Metrics.AllocRAM),
			common.FormatPercentage(nodeManifest.Utilization.PercentageCPU), common.FormatPercentage(nodeManifest.Utilization.PercentageRAM),
			fmt.Sprintf("%.2f", nodeManifest.Utilization.Score)})
	}
	t.Render()
}

// TabulatePodsMap Print pod analytics in a tabular form
func TabulatePodsMap(podsMap map[string]PodManifest) {
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{"Pod", "Namespace", "Status", "CPU Requests", "Memory Requests"})

	for podName, podMetric := range podsMap {
		t.AppendRow(table.Row{
			podName,
			podMetric.Pod.Namespace,
			podMetric.Pod.Status.Phase,
			common.FormatValue("CPU", podMetric.Metrics.ReqCPU), common.FormatValue("RAM", podMetric.Metrics.ReqRAM)})
	}
	t.Render()
}

// TabulateCluster Print cluster analytics in a tabular form
func TabulateCluster(clusterManifest *ClusterManifest) {
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.SetTitle(fmt.Sprintf("Cluster State\n"+
		"Number of Nodes: %v\n"+
		"Number of Pods: %v \n"+
		"Number of non-tainted Nodes: %v\n"+
		"Excess Nodes: %.2f",
		clusterManifest.NumberOfNodes, clusterManifest.NumberOfPods, clusterManifest.NumberOfNonTaintedNodes, clusterManifest.ExcessNodes))
	t.AppendHeader(table.Row{"Resource", "Pods Consumption", "Nodes Allocatable", "Percentage"})

	t.AppendRow(table.Row{"CPU",
		common.FormatValue("CPU", clusterManifest.TotalPodsMetrics.ReqCPU),
		common.FormatValue("CPU", clusterManifest.TotalNodeMetrics.AllocCPU),
		common.FormatPercentage(clusterManifest.Utilization.PercentageCPU)})
	t.AppendRow(table.Row{"RAM",
		common.FormatValue("RAM", clusterManifest.TotalPodsMetrics.ReqRAM),
		common.FormatValue("RAM", clusterManifest.TotalNodeMetrics.AllocRAM),
		common.FormatPercentage(clusterManifest.Utilization.PercentageRAM)})

	t.Render()
}
