package drainer

import (
	"context"
	"fmt"
	"strconv"

	"github.wdf.sap.corp/ICN-ML/aicore/operators/node-harvester/pkg/supervisor"
	internaltypes "github.wdf.sap.corp/ICN-ML/aicore/operators/node-harvester/pkg/types"

	"github.com/pkg/errors"

	"time"

	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	policy "k8s.io/api/policy/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

// Default pod eviction settings.
const (
	DefaultMaxGracePeriod   = 8 * time.Minute
	DefaultEvictionOverhead = 30 * time.Second

	DefaultTimeGap                      = 10 * time.Minute
	DefaultMinimumTimeSinceLastAddition = 60 * time.Minute

	DefaultMinimumNodes           = 3
	DefaultMinimumNonTaintedNodes = 3
	DefaultExcessNodesThreshold   = 2
	DefaultEnabled                = true
)

// Cordoner cordons nodes.
type Cordoner interface {
	// Cordon the supplied node. Marks it unschedulable for new pods.
	Cordon(nodeName string) error

	// Uncordon the supplied node. Marks it schedulable for new pods.
	Uncordon(nodeName string) error
}

// Drainer drains nodes.
type Drainer interface {
	// Drain the supplied node. Evicts the node of all but mirror and DaemonSet pods.
	Drain(nodeName string) error
}

// CordonDrainer both cordons and drains nodes!
type CordonDrainer interface {
	Cordoner
	Drainer
}

// APICordonDrainer drains Kubernetes nodes via the Kubernetes API.
type APICordonDrainer struct {
	c kubernetes.Interface
	s *supervisor.Supervisor

	// Current State
	LastNodeAddition time.Time
	LastScaleDown    time.Time

	// Settings
	enabled                      bool
	maxGracePeriod               time.Duration
	evictionHeadroom             time.Duration
	timeGap                      time.Duration
	minimumTimeSinceLastAddition time.Duration
	minimumNodes                 int
	minimumNonTaintedNodes       int
	excessNodesThreshold         float64
}

// NodeDesiredState to set a future state for the unschedulable node flag
type NodeDesiredState struct {
	nodeName      string
	unschedulable bool
}

// NewAPICordonDrainer returns a CordonDrainer that cordons and drains nodes via
// the Kubernetes API.
func NewAPICordonDrainer(c kubernetes.Interface, supervisor *supervisor.Supervisor) *APICordonDrainer {
	d := &APICordonDrainer{
		c: c,
		s: supervisor,

		// Setup Initial Settings
		enabled:                      DefaultEnabled,
		maxGracePeriod:               DefaultMaxGracePeriod,
		evictionHeadroom:             DefaultEvictionOverhead,
		timeGap:                      DefaultTimeGap,
		minimumTimeSinceLastAddition: DefaultMinimumTimeSinceLastAddition,
		minimumNodes:                 DefaultMinimumNodes,
		minimumNonTaintedNodes:       DefaultMinimumNonTaintedNodes,
		excessNodesThreshold:         DefaultExcessNodesThreshold,
	}
	return d
}

// AttemptDrain runs multiple checks to ensure that the drain procedure satisfies all the requirements
func (d *APICordonDrainer) AttemptDrain(nodeToDrain string, clusterManifest *internaltypes.ClusterManifest) {
	if !d.enabled {
		zap.S().Infow("Drainer", "state", "drainer is disabled based on the provided configuration")
		return
	}
	if clusterManifest.ExcessNodes < d.excessNodesThreshold {
		zap.S().Infow("Drainer", "state", "nothing to scale down, cluster has no excess resources")
		return
	}

	if time.Since(d.LastNodeAddition) < d.minimumTimeSinceLastAddition {
		remaining := int((d.minimumTimeSinceLastAddition - time.Since(d.LastNodeAddition)).Minutes())
		zap.S().Infof("waiting for default time for scale down operations to start after adding a new node, time remaining %v minutes", remaining)
		return
	}

	if time.Since(d.LastScaleDown) < d.timeGap {
		remaining := d.timeGap - time.Since(d.LastScaleDown)
		zap.S().Infof("Waiting for Default Grace Period for another Node Drain %v seconds remaining", int(remaining.Seconds()))
		return
	}

	if clusterManifest.NumberOfNodes < d.minimumNodes {
		logMessage := fmt.Sprintf("unable to scale down because the cluster has less than %v nodes", d.minimumNodes)
		zap.S().Infow("Drainer", "issue", logMessage)
		return
	}

	if clusterManifest.NumberOfNonTaintedNodes < d.minimumNonTaintedNodes {
		logMessage := fmt.Sprintf("unable to scale down because the cluster has less than %v non tainted nodes", d.minimumNonTaintedNodes)
		zap.S().Infow("Drainer", "issue", logMessage)
		return
	}

	// All conditions passed
	go d.ScaleDown(nodeToDrain)
}

// ScaleDown records timestamp to the last scale down and initiates a node drain
func (d *APICordonDrainer) ScaleDown(node string) {
	d.LastScaleDown = time.Now()
	zap.S().Infow("Cordoning Node", "node", node)
	err := d.Cordon(node)
	if err != nil {
		zap.S().Warnw("Couldn't Cordon Node", "node", node)
		return
	}

	zap.S().Infow("Initiating a node drain", "node", node)
	err = d.Drain(node)
	if err != nil {
		zap.S().Warnw("Couldn't drain node, will uncordon the node", "node", node)
		err = d.Uncordon(node)
		if err != nil {
			zap.S().Warnw("Couldn't Uncordon node", "node", node)
			return
		}
		return
	}
}

// Cordon the supplied node. Marks it unschedulable for new pods.
func (d *APICordonDrainer) Cordon(nodeName string) error {
	zap.S().Infow("Cordoning Node", "node", nodeName)

	// Increment Prometheus Metrics
	if d.s != nil {
		d.s.DrainerMetrics.NodesCordoned.Inc()
	}

	nodeDesiredState := NodeDesiredState{
		nodeName:      nodeName,
		unschedulable: true,
	}

	return d.AlterNodeState(nodeDesiredState)
}

// Uncordon the supplied node. Marks it schedulable for new pods.
func (d *APICordonDrainer) Uncordon(nodeName string) error {
	zap.S().Infow("Uncordoning Node", "node", nodeName)

	// Increment Prometheus Metrics
	if d.s != nil {
		d.s.DrainerMetrics.NodesUncordoned.Inc()
	}

	nodeDesiredState := NodeDesiredState{
		nodeName:      nodeName,
		unschedulable: false,
	}

	return d.AlterNodeState(nodeDesiredState)
}

// AlterNodeState from unschedulable to schedulable and vice-versa
func (d *APICordonDrainer) AlterNodeState(nodeDesiredState NodeDesiredState) error {
	ctx := d.getContext()
	node, err := d.c.CoreV1().Nodes().Get(ctx, nodeDesiredState.nodeName, metav1.GetOptions{})

	if err != nil {
		panic(err.Error())
	}

	if node.Spec.Unschedulable == nodeDesiredState.unschedulable {
		return nil
	}

	oldData, err := json.Marshal(node)
	if err != nil {
		return err
	}

	node.Spec.Unschedulable = nodeDesiredState.unschedulable

	newData, err := json.Marshal(node)
	if err != nil {
		return err
	}

	patchBytes, patchErr := strategicpatch.CreateTwoWayMergePatch(oldData, newData, node)
	if patchErr == nil {
		_, err = d.c.CoreV1().Nodes().Patch(ctx, node.Name, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
		if err != nil {
			return err
		}
	} else {
		_, err = d.c.CoreV1().Nodes().Update(ctx, node, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}

	return nil
}

// Drain searches and evicts all pods contained in a node.
func (d *APICordonDrainer) Drain(nodeName string) error {
	// Increment Prometheus Metrics
	if d.s != nil {
		d.s.DrainerMetrics.NodesDrained.Inc()
	}

	pods, err := d.getPods(nodeName)
	if err != nil {
		return errors.Wrapf(err, "cannot get pods for node %s", nodeName)
	}

	abort := make(chan struct{})
	errs := make(chan error, 1)
	for i := range pods {
		go d.evict(&pods[i], abort, errs)
	}
	// This will _eventually_ abort evictions. Evictions may spend up to
	// d.deleteTimeout() in d.awaitDeletion(), or 5 seconds in backoff before
	// noticing they've been aborted.
	defer close(abort)

	deadline := time.After(d.deleteTimeout())
	for range pods {
		select {
		case err := <-errs:
			if err != nil {
				return errors.Wrap(err, "cannot evict all pods")
			}
		case <-deadline:
			return errors.Wrap(errTimeout{}, "timed out waiting for evictions to complete")
		}
	}
	return nil
}

func (d *APICordonDrainer) evict(p *v1.Pod, abort <-chan struct{}, e chan<- error) {
	gracePeriod := int64(d.maxGracePeriod.Seconds())
	if p.Spec.TerminationGracePeriodSeconds != nil && *p.Spec.TerminationGracePeriodSeconds < gracePeriod {
		gracePeriod = *p.Spec.TerminationGracePeriodSeconds
	}
	for {
		select {
		case <-abort:
			e <- errors.New("pod eviction aborted")
			return
		default:
			err := d.c.CoreV1().Pods(p.Namespace).Evict(context.TODO(),
				&policy.Eviction{
					ObjectMeta:    metav1.ObjectMeta{Namespace: p.Namespace, Name: p.Name},
					DeleteOptions: &metav1.DeleteOptions{GracePeriodSeconds: &gracePeriod},
				})
			//err := d.c.CoreV1().Pods(p.GetNamespace()).Delete(d.getContext(), p.Name, metav1.DeleteOptions{})
			switch {
			// The eviction API returns 429 Too Many Requests if a pod
			// cannot currently be evicted, for example due to a pod
			// disruption budget.
			case apierrors.IsTooManyRequests(err):
				time.Sleep(5 * time.Second)
			case apierrors.IsNotFound(err):
				e <- nil
				return
			case err != nil:
				e <- errors.Wrapf(err, "cannot evict pod %s/%s", p.GetNamespace(), p.GetName())
				return
			default:
				e <- errors.Wrapf(d.awaitDeletion(p, d.deleteTimeout()), "cannot confirm pod %s/%s was deleted", p.GetNamespace(), p.GetName())
				return
			}
		}
	}
}

func (d *APICordonDrainer) awaitDeletion(p *v1.Pod, timeout time.Duration) error {
	return wait.PollImmediate(1*time.Second, timeout, func() (bool, error) {
		got, err := d.c.CoreV1().Pods(p.GetNamespace()).Get(d.getContext(), p.GetName(), metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return true, nil
		}
		if err != nil {
			return false, errors.Wrapf(err, "cannot get pod %s/%s", p.GetNamespace(), p.GetName())
		}
		if got.GetUID() != p.GetUID() {
			return true, nil
		}
		return false, nil
	})
}

// SetLastNodeAddition sets the time the last node was added to the cluster
func (d *APICordonDrainer) SetLastNodeAddition(time time.Time) {
	d.LastNodeAddition = time
}

func (d *APICordonDrainer) getPods(nodeName string) ([]v1.Pod, error) {
	pods, err := d.c.CoreV1().Pods(metav1.NamespaceAll).List(d.getContext(), metav1.ListOptions{
		FieldSelector: "spec.nodeName=" + nodeName,
	})
	if err != nil {
		return nil, err
	}

	return pods.Items, nil
}

func (d *APICordonDrainer) getContext() context.Context {
	return context.Background()
}

func (d *APICordonDrainer) deleteTimeout() time.Duration {
	return d.maxGracePeriod + d.evictionHeadroom
}

// UpdateSettings checks every setting in the drainer and sees if it needs to be updated or not and does the necessary action
func (d *APICordonDrainer) UpdateSettings(cm *v1.ConfigMap) error {
	data := cm.Data

	// Enabling/Disabling Drainer
	if value, ok := data["drainer_enabled"]; ok {
		sEnabled, err := strconv.ParseBool(value)
		if err != nil {
			return err
		}
		if d.enabled != sEnabled {
			d.enabled = sEnabled
			if d.enabled {
				zap.S().Info("Enabling drainer")
			} else {
				zap.S().Info("Disabling drainer")
			}
		}
	}

	// Set time gap
	if value, ok := data["time_gap"]; ok {
		sTimeGap, err := strconv.Atoi(value)
		if err != nil {
			return err
		}
		currentDuration := float64(d.timeGap) / float64(time.Minute)
		if int(currentDuration) != sTimeGap {
			zap.S().Infow("Changing default time gap between each node drain", "from", currentDuration, "to", sTimeGap)
			d.timeGap = time.Duration(sTimeGap) * time.Minute
		}
	}

	// Set time since last addition
	if value, ok := data["time_since_last_addition"]; ok {
		sTimeSinceLastAddition, err := strconv.Atoi(value)
		if err != nil {
			return err
		}
		currentDuration := float64(d.minimumTimeSinceLastAddition) / float64(time.Minute)
		if int(currentDuration) != sTimeSinceLastAddition {
			zap.S().Infow("Changing default time gap to start a scale down procedure", "from", currentDuration, "to", sTimeSinceLastAddition)
			d.minimumTimeSinceLastAddition = time.Duration(sTimeSinceLastAddition) * time.Minute
		}
	}

	// Set Excess Nodes Threshold
	if value, ok := data["excess_nodes_threshold"]; ok {
		sExcessNodesThreshold, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return err
		}
		if d.excessNodesThreshold != sExcessNodesThreshold {
			zap.S().Infow("Changing excess nodes threshold", "from", d.excessNodesThreshold, "to", sExcessNodesThreshold)
			d.excessNodesThreshold = sExcessNodesThreshold
		}
	}

	// Set Minimum Nodes
	if value, ok := data["minimum_nodes"]; ok {
		sMinimumNodes, err := strconv.Atoi(value)
		if err != nil {
			return err
		}
		if d.minimumNodes != sMinimumNodes {
			zap.S().Infow("Changing the value of minimum nodes", "from", d.minimumNodes, "to", sMinimumNodes)
			d.minimumNodes = sMinimumNodes
		}
	}

	// Set Minimum NonTainted Nodes
	if value, ok := data["minimum_non_tainted_nodes"]; ok {
		sMinimumNonTaintedNodes, err := strconv.Atoi(value)
		if err != nil {
			return err
		}
		if d.minimumNonTaintedNodes != sMinimumNonTaintedNodes {
			zap.S().Infow("Changing the value of minimum non tainted nodes", "from", d.minimumNonTaintedNodes, "to", sMinimumNonTaintedNodes)
			d.minimumNonTaintedNodes = sMinimumNonTaintedNodes
		}
	}

	zap.S().Info("Drainer settings update successful")
	return nil
}

type errTimeout struct{}

func (e errTimeout) Error() string {
	return "timed out"
}
