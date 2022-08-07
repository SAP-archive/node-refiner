# Testing Node Refiner
 
 Node Refiner Controller interacts with many parts of the Kubernetes infrastructure, which makes it a particular case for testing. As a result, we currently use a mixture of manual and automated tests to ensure that the operator's behavior is as intended.

## Automated Testing
### KUTTL
### What's KUTTL?

The KUbernetes Test TooL (KUTTL) is a toolkit that makes it easy to test [Kubernetes Operators](https://kudo.dev/#what-are-operators), just using YAML.

It provides a way to inject an operator (subject under test) during the TestSuite setup and allows tests to be standard YAML files. Test assertions are often partial YAML documents which assert the state defined is true.

It is also possible to have KUTTL automate the setup of a cluster.

### How to KUTTL?

```shell
kubectl kuttl test
```
It's as simple as this, by changing the value `${IMAGE_TAGGED}` (used for ci purposes) in the yaml test files to the image that you would like to test, running this command in the operator's directory, all the tests will be run against the cluster in your environment. This will include spinning a test namespace to test all your configurations and deleting it at the end of the test process.

```shell
kubectl kuttl test --start-control-plane
```
This command can also be used if you want to start a control plane for the testing process instead of using the current cluster, one can also change `--start-control-plane` to `--start-kind` to spin a kind cluster for testing.

### Current KUTTL Tests
**1. Installation**: Tests if the image provided in the manifests is present and that the necessary configurations are present for a successful application deployment.

**2. Service Account Attachement:** Asserts for failure if the operator doesn't have a service account attached to it, then a patch is done to add a service account and an assert for success is made to check whether the operator works as intended. 


## Manual Testing

2. Create a namespace to contain all the testing objects within it
```yaml
apiVersion: v1
kind: Namespace
metadata:
  labels:
    app: node-refiner
  name: node-refiner
```
```shell
kubectl apply -f namespace.yaml

kubectl config set-context --current --namespace=node-refiner
```
3. Create a K8s Service Account, to give this namespace the necessary rights.
```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  labels:
    app: node-refiner
  name: node-refiner-sa
  namespace: node-refiner

```
4. Create a K8s ClusterRoleBinding, to give this service account cluster-admin rights.
```yaml

apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  labels:
    app: node-refiner
  name: controller-admin
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: cluster-admin
subjects:
- kind: ServiceAccount
  name: node-refiner-sa
  namespace: node-refiner

```

### Deployment
Now we have all the necessary requirements for our application to be up and running, we now just need an image of this application to be functioning within our namespace. Therefore, we will create a deployment with only 1 replica.

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    app: node-refiner
  name: node-refiner
  namespace: node-refiner
spec:
  replicas: 1
  selector:
    matchLabels:
      app: node-refiner
  template:
    metadata:
      labels:
        app: node-refiner
    spec:
      containers:
      - env:
        - name: LISTENING_PORT
          value: "8080"
        image: eu.gcr.io/sap-ml-mlf-dev/com.sap.aicore/node-refiner:0.1.2
        imagePullPolicy: Always
        name: node-refiner
        resources:
          limits:
            cpu: 200m
            memory: 512Mi
          requests:
            cpu: 50m
            memory: 256Mi
      imagePullSecrets:
      - name: docker-registry
      serviceAccountName: node-refiner-sa
```

### Stress Test
The idea here is to test how the `CA` and this operator coordinate their tasks together. We will do this by simulating a heavy workload load to be scheduled in the cluster, thus requiring the `CA` to act upon the excess needed resources.

When the `CA` spins up new resources, the `NR` will be notified and will ensure that there is no action on its part to be performed for a pre-set Grace Period; this allows the cluster to stabilize its resources consumption before evaluating the cluster utilization and taking any action.

We can then delete this workload, leaving the cluster with an unneeded number of resources dangling. Ideally, the operator should act upon this and drain some of the nodes depending on the utilization metrics calculated. The`CA` would find that these nodes are highly under-utilized and will delete them in the following step.
1. First, ensure that you have the testing namespace active. If you haven't done yet, you'll find the steps written above.
2. We will mock a heavy workload by initiating a `mongodb` deployment.
```shell
kubectl apply -f https://k8s.io/examples/application/guestbook/mongo-deployment.yaml

```
3. The current number of replicas in the previous YAML file is `1`, lets scale it up to `200`
```shell
kubectl scale deployment mongo --replicas=200
```
4. You can either check the pods in the deployment and how many of them are ready.
5. It might take a couple of minutes for the `CA` to spin new nodes and for these nodes to be ready.
6. Once the deployment is finished, its time to delete it.
```shell

kubectl delete deployment mongo

```




