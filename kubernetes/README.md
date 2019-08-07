<!-- SPDX-License-Identifier: MIT -->
# Kosmoo Deployment

*Kosmoo* uses [Kustomize](https://github.com/kubernetes-sigs/kustomize) to manage kubernetes yaml files.
*Kustomize* is included in `kubectl` since version v1.14. 
Because of that the following documentation requires `kubectl` in version v1.14 or newer.

## Usage

Execute from the `kosmoo` directory, to generate the base deployment:
```
kubectl kustomize kubernetes/base
```
To apply the base deployment to your cluster:
```
kubectl apply -k kubernetes/base
```
To generate the deployment with patches for the container image:
```
kubectl kustomize kubernetes/overlays/examples
```
To apply the deployment with patches for the container image to your cluster:
```
kubectl apply -k kubernetes/overlays/examples
```

# License

