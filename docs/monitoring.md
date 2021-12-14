# Monitoring Node Harvester

Here we write more about the steps involved to monitor the behavior of the node harvester operator. Similar to the
testing document, this will provide steps to create an environment where you can see what kind of actions the controller
is taking and the generic state of the cluster.

We do this by utilizing the stack that includes Loki, Grafana, and Prometheus.

## Installing Stack

We need to initially deploy this stack, the easiest way is to use the corresponding helm chart.

```shell
helm upgrade --install loki grafana/loki-stack --namespace=<YOUR-NAMESPACE>  --set grafana.enabled=true,prometheus.enabled=true,prometheus.alertmanager.persistentVolume.enabled=false,prometheus.server.persistentVolume.enabled=false
```

By the time this is done you'll have the three tools installed, we then need to log in to our Grafana instance. We first
identify the password that we will use for logging in.

```shell
kubectl get secret --namespace <YOUR-NAMESPACE> loki-grafana -o jsonpath="{.data.admin-password}" | base64 --decode ; echo
```

Then we initiate a port-forwarding operation to be able to access it from our local browser

```shell
kubectl port-forward --namespace <YOUR-NAMESPACE> service/loki-grafana 3000:80
```

## Configuring Prometheus

Now, we need to configure prometheus in order to scrape the custom metrics we are exposing from the controller.

Get the config map that sets up the data sources

```shell
kubectl -n node-harvester-ns get cm  loki-stack-prometheus-server -o jsonpath="{ .data.prometheus\\.yml }" > prom.yaml
```

Append the following jobs

```shell
- job_name: node-harvester
  scrape_interval: 10s
  kubernetes_sd_configs:
    - role: pod
  relabel_configs:
    - source_labels: [__meta_kubernetes_namespace]
      action: replace
      target_label: k8s_namespace
    - source_labels: [__meta_kubernetes_pod_name]
      action: replace
      target_label: k8s_pod_name
    - source_labels: [__address__]
      action: replace
      regex: ([^:]+)(?::\d+)?
      replacement: ${1}:8080
      target_label: __address__
    - source_labels: [__meta_kubernetes_pod_label_app]
      action: keep
      regex: node-harvester
```

Apply changes by deleting the old configmap and adding the locally edited one

```shell
kubectl -n node-harvester-ns delete cm loki-stack-prometheus-server
kubectl -n node-harvester-ns create cm loki-stack-prometheus-server --from-file=prometheus.yml=prom.yaml
rm prometheus.yaml
```

## Additional Commands\

Check what's the usage of a Persistent Volume Claim attached to a certain Pod

```shell
kubectl -n <namespace> -c <container-name> exec <pod-name> -- df -h
```

Check the pod metrics, CPU and Memory

```shell
kubectl top pod <pod-name> -n <namespace> --containers
```

### Sources

1. [Install Loki with Helm](https://grafana.com/docs/loki/latest/installation/helm/)
2. [Custom Prometheus Metrics for Apps Running in Kubernetes](https://zhimin-wen.medium.com/custom-prometheus-metrics-for-apps-running-in-kubernetes-498d69ada7aa)