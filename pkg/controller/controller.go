package controller

import (
	"context"
	"time"

	"github.com/SAP/node-refiner/pkg/common"
	"github.com/SAP/node-refiner/pkg/drainer"
	"github.com/SAP/node-refiner/pkg/supervisor"
	"github.com/SAP/node-refiner/pkg/types"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

// WorkloadsController central controller that manages the communication between the different modules
type WorkloadsController struct {
	client kubernetes.Interface

	// Drainer Module
	d *drainer.APICordonDrainer

	// Prometheus Supervision
	s *supervisor.Supervisor

	// Informers
	nodesInformer cache.SharedIndexInformer
	podsInformer  cache.SharedIndexInformer
	cmInformer    cache.SharedIndexInformer

	// Cluster State
	podsMap  map[string]types.PodManifest
	nodesMap map[string]types.NodeManifest
}

// NewController creates a new controller and returns a pointer to the created object
func NewController() (*WorkloadsController, error) {

	var kubeClient kubernetes.Interface

	if _, err := rest.InClusterConfig(); err != nil {
		kubeClient, err = common.GetClientOutOfCluster()
		if err != nil {
			zap.S().Warn("Unable to instantiate a client")
		}
	} else {
		kubeClient, err = common.GetClient()
		if err != nil {
			zap.S().Warn("Unable to instantiate a client")
		}
	}

	s := supervisor.InitSupervisor("node_refiner")
	d := drainer.NewAPICordonDrainer(kubeClient, s)

	go s.StartSupervising()

	controller := WorkloadsController{
		client:   kubeClient,
		d:        d,
		s:        s,
		podsMap:  make(map[string]types.PodManifest),
		nodesMap: make(map[string]types.NodeManifest),
	}

	return &controller, nil
}

// CreateRunInformers create and run the informers in a parallel thread
func (c *WorkloadsController) CreateRunInformers() {

	factory := informers.NewSharedInformerFactory(c.client, 10*time.Minute)

	podsInformer := cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				options.FieldSelector = "status.phase!=Succeeded,status.phase!=Failed,status.phase!=Unknown"
				return c.client.CoreV1().Pods(corev1.NamespaceAll).List(context.TODO(), options)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				options.FieldSelector = "status.phase!=Succeeded,status.phase!=Failed,status.phase!=Unknown"
				return c.client.CoreV1().Pods(corev1.NamespaceAll).Watch(context.TODO(), options)
			},
		},
		&corev1.Pod{},
		0, //Skip resync
		cache.Indexers{},
	)

	c.podsInformer = podsInformer
	// c.podsInformer = factory.Core().V1().Pods().Informer()
	c.cmInformer = factory.Core().V1().ConfigMaps().Informer()
	c.nodesInformer = factory.Core().V1().Nodes().Informer()

	// Starting the factory will start all informers created
	// by this factory
	stopCh := common.CreateSignalHandler()
	factory.Start(stopCh)
	go c.podsInformer.Run(stopCh)
	zap.S().Info("Informers running")

	//
	// Wait for informer to sync. We use the helper function
	// WaitForCacheSync which will also take care of signal
	// handling, i.e. it returns when stopCh is closed
	if ok := cache.WaitForCacheSync(stopCh, c.podsInformer.HasSynced); !ok {
		panic("Error while waiting for pods informer to sync")
	}

	if ok := cache.WaitForCacheSync(stopCh, c.cmInformer.HasSynced); !ok {
		panic("Error while waiting for config maps informer to sync")
	}

	if ok := cache.WaitForCacheSync(stopCh, c.nodesInformer.HasSynced); !ok {
		panic("Error while waiting for nodes informer to sync")
	}

	c.AddNodeEventHandler()
	c.AddPodEventHandler()
	c.AddConfigMapEventHandler()

	<-stopCh
	zap.S().Info("Stopping Node Refiner")
}

func (c *WorkloadsController) calculateTotalPodsMetrics() {
	for key := range c.nodesMap {
		totalMetrics := types.PodMetrics{}
		node := c.nodesMap[key]
		for _, pod := range node.Pods {
			totalMetrics.AddPodMetrics(pod.Metrics)
		}
		node.TotalPodsRequests = totalMetrics
		c.nodesMap[key] = node
	}
}

func (c *WorkloadsController) calculateClusterUtilization() {
	for key := range c.nodesMap {
		node := c.nodesMap[key]
		node.Utilization = types.CalculateUtilizationPercentage(&node.TotalPodsRequests, node.Metrics)
		c.nodesMap[key] = node
	}
}

// RunCalculationLoop Run the cluster calculation loop every minute
func (c *WorkloadsController) RunCalculationLoop() {
	for {
		supervisor.UpdateHeartbeat()
		c.clearPodsList()
		c.addPodsToNodes()
		c.calculateTotalPodsMetrics()
		c.calculateClusterUtilization()
		cluster := types.NewClusterManifest(c.nodesMap)
		potentialNodeDrain, err := c.getNodeToDrain()
		if err != nil {
			zap.S().Warn("Not ready to get nodes to drain")
		} else {
			zap.S().Infow("Potential node to drain",
				"node", potentialNodeDrain.Node.Name, "number of pods", len(potentialNodeDrain.Pods),
				"CPU Utilization", common.FormatPercentage(potentialNodeDrain.Utilization.PercentageCPU),
				"RAM Utilization", common.FormatPercentage(potentialNodeDrain.Utilization.PercentageRAM))
			cluster.CalculateExcessNode(potentialNodeDrain)
			c.d.AttemptDrain(potentialNodeDrain.Node.Name, &cluster)
		}

		logCluster(&cluster)
		c.s.ClusterMetrics.PublishClusterMetrics(&cluster)
		c.s.ClusterMetrics.PublishNodeUnschedulable(c.nodesMap)
		//types.TabulateNodeMap(c.nodesMap)
		//types.TabulatePodsMap(c.podsMap)
		//types.TabulateCluster(&cluster)

		time.Sleep(1 * time.Minute)
	}
}

func logCluster(clusterManifest *types.ClusterManifest) {
	zap.S().Infow("Cluster State",
		"Number of nodes", clusterManifest.NumberOfNodes,
		"Number of non tainted nodes", clusterManifest.NumberOfNonTaintedNodes,
		"Number of pods", clusterManifest.NumberOfPods,
		"Number of excess nodes", clusterManifest.ExcessNodes,
		"CPU Utilization", common.FormatPercentage(clusterManifest.Utilization.PercentageCPU),
		"RAM Utilization", common.FormatPercentage(clusterManifest.Utilization.PercentageRAM),
	)
}
