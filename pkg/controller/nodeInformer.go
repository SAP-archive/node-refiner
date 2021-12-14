package controller

import (
	"errors"
	"github.wdf.sap.corp/ICN-ML/aicore/operators/node-harvester/pkg/common"
	"github.wdf.sap.corp/ICN-ML/aicore/operators/node-harvester/pkg/types"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"
)

// AddNodeEventHandler subscribes and routes the different events of interest to the nodes informer
func (c *WorkloadsController) AddNodeEventHandler() {
	c.nodesInformer.AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc:    c.addNode,
			UpdateFunc: c.updateNode,
			DeleteFunc: c.deleteNode,
		})
}

func (c *WorkloadsController) addNode(obj interface{}) {
	node := obj.(*corev1.Node)
	c.nodesMap[node.Name] = types.NodeManifest{
		Node:         node,
		Metrics:      types.CreateNodeMetricsFromNodeObj(node),
		Pods:         make([]*types.PodManifest, 10),
		NumberOfPods: 0,
	}
	nodeTime := node.CreationTimestamp.Time
	if c.d.LastNodeAddition.Before(nodeTime) {
		zap.S().Infow("Updated the newest node addition time", "node", node.Name, "creation timestamp", nodeTime)
		c.d.LastNodeAddition = nodeTime
	}
}
func (c *WorkloadsController) updateNode(old, new interface{}) {
	// Cast the obj as Node
	oldNode := old.(*corev1.Node)
	newNode := new.(*corev1.Node)

	if compareNodes(oldNode, newNode) {
		delete(c.nodesMap, oldNode.Name)
		c.nodesMap[newNode.Name] = types.NodeManifest{
			Node:         newNode,
			Metrics:      types.CreateNodeMetricsFromNodeObj(newNode),
			Pods:         make([]*types.PodManifest, 10),
			NumberOfPods: 0,
		}
		zap.S().Infof("Update took place for node %v, now node %v", oldNode.Name, newNode.Name)
	}
}
func (c *WorkloadsController) deleteNode(obj interface{}) {
	// Cast the obj as Node
	node := obj.(*corev1.Node)
	zap.S().Infow("node was deleted", "node", node.Name)
	delete(c.nodesMap, node.Name)
}

// compareNodes checks if there are any relevant information that got changed to perform an update
func compareNodes(oldNode *corev1.Node, newNode *corev1.Node) bool {
	// Change node schedulable
	if oldNode.Spec.Unschedulable != newNode.Spec.Unschedulable {
		return true
	}

	// Change in node allocatable resources
	if *oldNode.Status.Allocatable.Cpu() != *newNode.Status.Allocatable.Cpu() || *oldNode.Status.Allocatable.Memory() != *newNode.Status.Allocatable.Memory() {
		return true
	}

	if len(oldNode.Spec.Taints) != len(newNode.Spec.Taints) {
		return true
	}

	return false
}

// getRandomNode picks one of the non-tainted nodes randomly
func (c *WorkloadsController) getRandomNode() (*types.NodeManifest, error) {
	var res types.NodeManifest
	found := false

	if len(c.nodesMap) == 0 {
		return &res, errors.New("couldn't find any node manifests in this map")
	}

	for _, nm := range c.nodesMap {
		if !common.CheckForTaints(nm.Node) {
			res = nm
			found = true
		}
		if found {
			break
		}
	}
	if !found {
		return &res, errors.New("all nodes are tainted, unable to find any node to drain")
	}

	return &res, nil
}

// getNodeToDrain get the least utilized node, potentially to drain it
func (c *WorkloadsController) getNodeToDrain() (*types.NodeManifest, error) {
	nmMin, err := c.getRandomNode()
	if err != nil {
		zap.S().Warnw("unable to proceed with picking a node", "error", err)
		return nmMin, err
	}

	for i := range c.nodesMap {
		nm := c.nodesMap[i]
		if !common.CheckForTaints(nm.Node) {
			if nm.Utilization.Score < nmMin.Utilization.Score {
				nmMin = &nm
			}
		}
	}

	return nmMin, nil
}

// addPodsToNodes assigns Pods to the corresponding node in the central nodesMap
func (c *WorkloadsController) addPodsToNodes() {
	for i := range c.podsMap {
		pm := c.podsMap[i]
		c.addIfNodeExists(&pm)
	}
}

// clearPodsList clears all the Pods in the NodeManifest object
func (c *WorkloadsController) clearPodsList() {
	for i, val := range c.nodesMap {
		val.Pods = make([]*types.PodManifest, 0)
		c.nodesMap[i] = val
	}
}

// addIfNodeExists add the pod if and only if It was already added in the central nodesMap (avoid data race)
func (c *WorkloadsController) addIfNodeExists(podManifest *types.PodManifest) {
	if val, ok := c.nodesMap[podManifest.Pod.Spec.NodeName]; ok {
		val.Pods = append(val.Pods, podManifest)
		c.nodesMap[podManifest.Pod.Spec.NodeName] = val
	}
}
