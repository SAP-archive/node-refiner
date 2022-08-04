package controller

import (
	"github.com/SAP/node-refiner/pkg/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"
)

// AddPodEventHandler subscribes and routes the different events of interest to the pods informer
func (c *WorkloadsController) AddPodEventHandler() {
	c.podsInformer.AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc:    c.addPod,
			UpdateFunc: c.updatePod,
			DeleteFunc: c.deletePod,
		})
}

// addPod notifies informer that a pod is added
func (c *WorkloadsController) addPod(obj interface{}) {
	// Cast the obj as Pods
	pod := obj.(*corev1.Pod)
	pm := types.NewPodManifest(pod)
	c.podsMap[pod.Name] = pm
}

// updatePod notifies informer that a pod is updated
func (c *WorkloadsController) updatePod(old, new interface{}) {
	// Cast the obj as Pods
	oldPod := old.(*corev1.Pod)
	newPod := new.(*corev1.Pod)
	if comparePods(oldPod, newPod) {
		delete(c.podsMap, oldPod.Name)
		podManifest := types.NewPodManifest(newPod)
		c.podsMap[newPod.Name] = podManifest
	}
}

// deletePod notifies informer that a pod is deleted
func (c *WorkloadsController) deletePod(obj interface{}) {
	// Cast the obj as Pods
	pod := obj.(*corev1.Pod)
	delete(c.podsMap, pod.Name)
}

// comparePods compares the application relevant changes and send a bool value to act upon them if found
func comparePods(oldPod *corev1.Pod, newPod *corev1.Pod) bool {
	// Different container sizes
	if len(oldPod.Spec.Containers) != len(newPod.Spec.Containers) {
		return true
	}
	// Change in the allocated Pod
	if oldPod.Spec.NodeName != newPod.Spec.NodeName {
		return true
	}
	return false
}
