# Quickstart

Installing node-refiner is a straight forward process, you can use pre-prepared kustomize files in this repository by invoking this command

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