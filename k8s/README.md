# Kubernetes

Simple [k3s](https://k3s.io/) cluster for my homelab.

## Setup

First, make sure you're using the correct kubeconfig.
You should see relevant info when running `kubectl config view`,

```shell
$ kc config view
apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: DATA+OMITTED
    server: https://cluster.home:6443
  name: default
contexts:
- context:
    cluster: default
    user: default
  name: default
current-context: default
kind: Config
preferences: {}
users:
- name: default
  user:
    client-certificate-data: DATA+OMITTED
    client-key-data: DATA+OMITTED
```

## Deploying changes

Changes are deployed incrementally.
To deploy an application, for example `navidrome`, simply `kubectl apply`,

```shell
kubectl apply -f navidrome/
```
