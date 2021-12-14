package controller

import (
	"testing"
	"time"

	"github.wdf.sap.corp/ICN-ML/aicore/operators/node-harvester/pkg/drainer"
	"github.wdf.sap.corp/ICN-ML/aicore/operators/node-harvester/pkg/types"

	"github.com/google/go-cmp/cmp"
	v1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
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

func TestListImages(t *testing.T) {
	var tests = []struct {
		description string
		namespace   string
		expected    []string
		objs        []runtime.Object
	}{
		{"no pods", "", nil, nil},
		{"all namespaces", "", []string{"a", "b"}, []runtime.Object{pod("correct-namespace", "a"), pod("wrong-namespace", "b")}},
		{"filter namespace", "correct-namespace", []string{"a"}, []runtime.Object{pod("correct-namespace", "a"), pod("wrong-namespace", "b")}},
		{"wrong namespace", "correct-namespace", nil, []runtime.Object{pod("wrong-namespace", "b")}},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			client := fake.NewSimpleClientset(test.objs...)
			actual, err := listImages(client, test.namespace)
			if err != nil {
				t.Errorf("Unexpected error: %s", err)
				return
			}
			if diff := cmp.Diff(actual, test.expected); diff != "" {
				t.Errorf("%T differ (-got, +want): %s", test.expected, diff)
				return
			}
		})
	}
}

func TestControllerPodList(t *testing.T) {

	client := fake.NewSimpleClientset(pod("namespace", "test"))
	controller := testController(client)

	time.Sleep(1 * time.Second)

	if len(controller.podsMap) != 1 {
		t.Errorf("Expected 1 pod, got %d", len(controller.podsMap))
	}

}
