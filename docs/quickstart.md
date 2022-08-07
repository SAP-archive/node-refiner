# Quickstart

Installing node-refiner is a straight forward process, you can use pre-prepared kustomize files in this repository by invoking this command

```shell
kubectl apply -k manifests/base
```

You can specify the image in the [kustomization](../manifests/base/kustomization.yaml) file.

```yaml:../manifests/base/kustomization.yaml [26-29]

```