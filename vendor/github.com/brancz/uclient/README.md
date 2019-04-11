# uclient

This go package contains tooling around [k8s.io/client-go][client-go] to make working with unstructured data in Kubernetes easier. The motivation in particular is the case of having Kubernetes manifests available and not caring about unmarshaling it into actual go structs, but just wanting to work with the Kubernetes API.

## Usage

```
go get github.com/brancz/uclient
```

You can simply get an unstructured client to work with the Kubernetes API by passing in the unstructured object.

```

```
