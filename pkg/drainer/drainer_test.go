package drainer

import (
	"context"
	"testing"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
)

const (
	testNodeName = "Test Node"
)

func node(unschedulable bool) *v1.Node {
	return &v1.Node{ObjectMeta: meta_v1.ObjectMeta{Name: testNodeName, Labels: map[string]string{"node-type": testNodeName}}, Spec: v1.NodeSpec{Unschedulable: unschedulable}}
}

func TestAddNode(t *testing.T) {
	client := fake.NewSimpleClientset(node(false))
	_, err := client.CoreV1().Nodes().Get(context.TODO(), testNodeName, meta_v1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			t.Error(err)
		} else {
			t.Errorf("failed to get node: %s", err)
		}
	}
}

// TestCordon tests if controller can Cordon a Node
func TestCordon(t *testing.T) {
	// Creating test environment
	objs := []runtime.Object{node(false)}
	client := fake.NewSimpleClientset(objs...)
	d := NewAPICordonDrainer(client, nil)

	// Attempt to cordon node
	err := d.Cordon(testNodeName)

	if err != nil {
		t.Errorf("Unexpected error while cordoning node: %s", err)
		return
	}

	// Get updated node object
	node, err := client.CoreV1().Nodes().Get(context.TODO(), testNodeName, meta_v1.GetOptions{})

	if err != nil {
		t.Errorf("failed to get node: %s", err)
	}

	// Assert that node is is not still accessible by the API
	if node.Spec.Unschedulable == false {
		t.Errorf("Node wasn't cordoned")
		return
	}
}

// TestUncordon tests if controller can Uncordon a Node
func TestUncordon(t *testing.T) {
	objs := []runtime.Object{node(true)}
	client := fake.NewSimpleClientset(objs...)
	d := NewAPICordonDrainer(client, nil)
	err := d.Uncordon(testNodeName)
	if err != nil {
		t.Errorf("Unexpected error while uncordoning node: %s", err)
		return
	}

	node, err := client.CoreV1().Nodes().Get(context.TODO(), testNodeName, meta_v1.GetOptions{})
	if err != nil {
		t.Errorf("failed to get node: %s", err)
	}

	if node.Spec.Unschedulable == true {
		t.Errorf("Node wasn't Uncordoned")
		return
	}
}

// TestCordonUncordon tests if controller can Cordon and Uncordon a Node consequtively
func TestCordonUncordon(t *testing.T) {
	objs := []runtime.Object{node(false)}
	client := fake.NewSimpleClientset(objs...)
	d := NewAPICordonDrainer(client, nil)

	err := d.Cordon(testNodeName)

	if err != nil {
		t.Errorf("Unexpected error while cordoning node: %s", err)
		return
	}

	err = d.Uncordon(testNodeName)

	if err != nil {
		t.Errorf("Unexpected error while uncordoning node: %s", err)
		return
	}

	node, err := client.CoreV1().Nodes().Get(context.TODO(), testNodeName, meta_v1.GetOptions{})
	if err != nil {
		t.Errorf("failed to get node: %s", err)
	}

	if node.Spec.Unschedulable == true {
		t.Errorf("Node wasn't Uncordoned")
		return
	}
}
