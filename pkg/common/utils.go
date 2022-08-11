// package common provides all the common functions used by the controller
// by handling cluster configuration, and formatting the output
package common

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// GetClient returns a k8s clientset to the request from inside of cluster
func GetClient() (kubernetes.Interface, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		zap.S().Fatalf("Can not get kubernetes config: %v", err)
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		zap.S().Fatalf("Can not create kubernetes client: %v", err)
		return nil, err
	}

	return clientset, nil
}

func buildOutOfClusterConfig() (*rest.Config, error) {
	//kubeconfigPath := "/Users/ali/Dev/Kubernetes/kubeconfig/ber-config.yaml"
	kubeconfigPath := os.Getenv("KUBECONFIG")
	if kubeconfigPath == "" {
		kubeconfigPath = os.Getenv("HOME") + "/.kube/config"
	}
	return clientcmd.BuildConfigFromFlags("", kubeconfigPath)
}

// GetClientOutOfCluster returns a k8s clientset to the request from outside of cluster
func GetClientOutOfCluster() (kubernetes.Interface, error) {
	config, err := buildOutOfClusterConfig()
	if err != nil {
		zap.S().Fatalf("Can not get kubernetes config: %v", err)
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		zap.S().Fatalf("Can not create kubernetes client: %v", err)
		return nil, err
	}

	return clientset, nil
}

// FormatValue to prepare the quantities for logging
func FormatValue(resourceType string, quantity resource.Quantity) string {
	switch resourceType {
	case "CPU":
		return fmt.Sprintf("%vmi", quantity.ScaledValue(resource.Milli))
	case "RAM":
		return fmt.Sprintf("%vMB", quantity.ScaledValue(resource.Mega))
	default:
		return "Unknown Quantity"
	}
}

// FormatPercentage reusable styling for printing percentages
func FormatPercentage(value float64) string {
	return fmt.Sprintf("%.2f%%", value)
}

// CheckForTaints returns whether the node is tainted or not
func CheckForTaints(node *corev1.Node) bool {
	if len(node.Spec.Taints) > 0 || node.Spec.Unschedulable {
		return true
	}
	return false
}

// CreateSignalHandler picks any signal for closing the controller
func CreateSignalHandler() (stopCh <-chan struct{}) {
	stop := make(chan struct{})
	c := make(chan os.Signal, 2)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-c
		fmt.Printf("Signal handler: received signal %s\n", sig)
		close(stop)
	}()
	return stop
}
