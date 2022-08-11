# Quickstart

Installing `node-refiner` is a straightforward process; you can use pre-prepared kustomize files in this repository by invoking this command.

```shell
kubectl apply -k manifests/base
```

You can specify the image version in the [kustomization](../manifests/base/kustomization.yaml) file.

```yaml
images:
  - name: default
    newName: ghcr.io/sap/node-refiner
    newTag: latest
```

## Access

Given that `node-refiner` handles eviction/deletion of pods, cordoning/uncordoning, and draining nodes, it requires high-level access to control the cluster. To simplify it, we bind a cluster-admin role to the service account, but creating a separate Cluster Role explicitly for `node-refiner` is recommended.


## YAML Files Structure

![Architecture](img/yaml-structure.png)