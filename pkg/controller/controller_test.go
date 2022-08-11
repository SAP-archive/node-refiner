package controller

import (
	"testing"
	"time"

	"github.com/SAP/node-refiner/pkg/drainer"
	"github.com/SAP/node-refiner/pkg/types"

	v1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func pod(namespace, image string) *v1.Pod {
	return &v1.Pod{ObjectMeta: meta_v1.ObjectMeta{Namespace: namespace}, Spec: v1.PodSpec{Containers: []v1.Container{{Image: image}}}}
}

func testController(client *fake.Clientset) *WorkloadsController {
	d := drainer.NewAPICordonDrainer(client, nil)
	controller := WorkloadsController{
		client:   client,
		d:        d,
		s:        nil,
		podsMap:  make(map[string]types.PodManifest),
		nodesMap: make(map[string]types.NodeManifest),
	}

	go controller.CreateRunInformers()

	return &controller
}

func TestControllerPodList(t *testing.T) {

	client := fake.NewSimpleClientset(pod("namespace", "test"))
	controller := testController(client)

	time.Sleep(1 * time.Second)

	if len(controller.podsMap) != 1 {
		t.Errorf("Expected 1 pod, got %d", len(controller.podsMap))
	}

}
