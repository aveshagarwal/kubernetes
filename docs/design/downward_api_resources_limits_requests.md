# Downward APIs for resource limits and requests

## Background
Currently downward APIs (via environment variables and volume plugin) only support exposing a Pod's name, namespace, annotations, labels and its IP. This document explains the need and design to extend them to expose resources (e.g. cpu, memory, storage) limits and requests.

## Motivation
By exposing information about computing resource requests & limits into pods, image/application authors can be aware of the computing environment in which the pods run, and could modify container images accordingly.

## Design
This is mostly driven by the discussion in https://github.com/kubernetes/kubernetes/issues/9473.

### API with full json path selectors

Full json path selectors specify the complete path to the resources limits and requests relative to pod spec.

#### Environment variables

This table shows how selectors can be used for various requests and limits to be exposed as environment variables. 

| Name | Selectors |
| ---- | ------------------- |
| CPU_LIMIT | .spec.containers[?(@.name=="container-name")].resources.limits.cpu |
| MEMORY_LIMIT | .spec.containers[?(@.name=="container-name")].resources.limits.memory |
| STORAGE_LIMIT | .spec.containers[?(@.name=="container-name")].resources.limits.storage |
| CPU_REQUEST | .spec.containers[?(@.name=="container-name")].resources.requests.cpu |
| MEMORY_REQUEST | .spec.containers[?(@.name=="container-name")].resources.requests.memory |
| STORAGE_REQUEST | .spec.containers[?(@.name=="container-name")].resources.requests.storage |

#### Volume plugin

This table shows how selectors can be used for various requests and limits to be exposed as volumes. 

| Path | Selectors |
| ---- | ------------------- |
| cpu_limit | .spec.containers[?(@.name=="container-name")].resources.limits.cpu |
| memory_limit | .spec.containers[?(@.name=="container-name")].resources.limits.cpu  |
| storage_limit | .spec.containers[?(@.name=="container-name")].resources.limits.cpu  |
| cpu_request | .spec.containers[?(@.name=="container-name")].resources.limits.cpu  |
| memory_request | .spec.containers[?(@.name=="container-name")].resources.limits.cpu  |
| storage_request | .spec.containers[?(@.name=="container-name")].resources.limits.cpu  |

Volumes are pod scoped, so a selector should be specified with a particular container name.

Note: environment variables and volume path names could be anything and not necessarily as specified above.

Full json path selectors will use existing `type ObjectFieldSelector` to extend the current implementation for resources requests and limits.

```
// ObjectFieldSelector selects an APIVersioned field of an object.
type ObjectFieldSelector struct {
        // Required: Version of the schema the FieldPath is written in terms of.
        // If no value is specified, it will be defaulted to the APIVersion of the
        // enclosing object.
        APIVersion string `json:"apiVersion"`
        // Required: Path of the field to select in the specified API version
        FieldPath string `json:"fieldPath"`
}
```

#### Examples

```
apiVersion: v1
kind: Pod
metadata:
  name: dapi-test-pod
spec:
  containers:
    - name: test-container
      image: gcr.io/google_containers/busybox
      command: [ "/bin/sh", "-c", "env" ]
      env:
        - name: CPU_LIMIT
          valueFrom:
            fieldRef:
              fieldPath: .spec.containers[?(@.name=="container-name")].resources.limits.cpu
  restartPolicy: Never
```

```
apiVersion: v1
kind: Pod
metadata:
  name: kubernetes-downwardapi-volume-example
spec:
  containers:
    - name: client-container
      image: gcr.io/google_containers/busybox
      command: ["sh", "-c", "while true; do if [[ -e /etc/labels ]]; then cat /etc/labels; fi; if [[ -e /etc/annotations ]]; then cat /etc/annotations; fi; sleep 5; done"]
      volumeMounts:
        - name: podinfo
          mountPath: /etc
          readOnly: false
  volumes:
    - name: podinfo
      downwardAPI:
        items:
          - path: "cpu_limit"
            fieldRef:
              fieldPath: .spec.containers[?(@.name=="container-name")].resources.limits.cpu
```

### API with partial json path selectors

Partial json path selectors specify paths to resources limits and requests relative to the container spec.

#### Environment variables

This table shows how selectors can be used for various requests and limits to be exposed as environment variables.

| Name | Selectors |
| -------------------- | ------------------- |
| CPU_LIMIT | resources.limits.cpu |
| MEMORY_LIMIT | resources.limits.memory |
| STORAGE_LIMIT | resources.limits.storage |
| CPU_REQUEST | resources.requests.cpu |
| MEMORY_REQUEST | resources.requests.memory |
| STORAGE_REQUEST | resources.requests.storage |

Since envionment variables are container scoped, so there is no need to specify container name as part of the partial selectors as they are relative to container spec.

#### Volume plugin

| Path | Selectors |
| -------------------- | ------------------- | 
| container_name/cpu_limit | resources.limits.cpu | 
| container_name/memory_limit | resources.limits.memory | 
| container_name/storage_limit | resources.limits.storage | 
| container_name/cpu_request | resources.requests.cpu | 
| container_name/memory_request | resources.requests.memory | 
| container_name/storage_request | resources.requests.memory |

Since envionment variables are container scoped, so container name must be specified as part of their path name. The format is that container name and path name must be separated by slash (`/`).

Note: Also, environment variables and volume path names could be anything and not necessarily as specified above.

Partial json selectors will be implemented by introducing `containerFieldRef` to extend the current implementation for resources requests and limits, and will be part of `type DownwardAPIVolumeFile` and `type EnvVarSource` as follows:

```
/ Represents a single file containing information from the downward API
type DownwardAPIVolumeFile struct {
        // Required: Path is  the relative path name of the file to be created. Must not be absolute or contain the '..' path. Must be utf-8 encoded. The first item of the relative path must not start with '..'
        Path string `json:"path"`
        // Required: Selects a field of the pod: only annotations, labels, name and  namespace are supported.
        FieldRef ObjectFieldSelector `json:"fieldRef"`
        // Required: Selects a field of the container: only resources limits and requests (cpu, memory, storage) are supported.
        ContainerFieldRef ObjectFieldSelector `json:"containerFieldRef,omitempty"`
}


// EnvVarSource represents a source for the value of an EnvVar.
// Only one of its fields may be set.
type EnvVarSource struct {
        // Required: Selects a field of the container: only resources limits and requests (cpu, memory, storage) are supported.
        ContainerFieldRef *ObjectFieldSelector `json:"containerfieldRef,omitempty"`
        // Selects a field of the pod; only name and namespace are supported.
        FieldRef *ObjectFieldSelector `json:"fieldRef,omitempty"`
        // Selects a key of a ConfigMap.
        ConfigMapKeyRef *ConfigMapKeySelector `json:"configMapKeyRef,omitempty"`
        // Selects a key of a secret in the pod's namespace.
        SecretKeyRef *SecretKeySelector `json:"secretKeyRef,omitempty"`
}

// ObjectFieldSelector selects an APIVersioned field of an object.
type ObjectFieldSelector struct {
        // Required: Version of the schema the FieldPath is written in terms of.
        // If no value is specified, it will be defaulted to the APIVersion of the
        // enclosing object.
        APIVersion string `json:"apiVersion"`
        // Required: Path of the field to select in the specified API version
        FieldPath string `json:"fieldPath"`
}
```

#### Examples

```
apiVersion: v1
kind: Pod
metadata:
  name: dapi-test-pod
spec:
  containers:
    - name: test-container
      image: gcr.io/google_containers/busybox
      command: [ "/bin/sh", "-c", "env" ]
      env:
        - name: CPU_LIMIT
          valueFrom:
            containerFieldRef:
              fieldPath: resources.limits.cpu
  restartPolicy: Never
```

```
apiVersion: v1
kind: Pod
metadata:
  name: kubernetes-downwardapi-volume-example
spec:
  containers:
    - name: client-container
      image: gcr.io/google_containers/busybox
      command: ["sh", "-c", "while true; do if [[ -e /etc/labels ]]; then cat /etc/labels; fi; if [[ -e /etc/annotations ]]; then cat /etc/annotations; fi; sleep 5; done"]
      volumeMounts:
        - name: podinfo
          mountPath: /etc
          readOnly: false
  volumes:
    - name: podinfo
      downwardAPI:
        items:
          - path: "container_name/cpu_limit"
            containerFieldRef:
              fieldPath: resources.limits.cpu
```
## Output Format
The output format for resources limits and requests will be same as cgroups output format.

## Validations

1. For APIs with full json path selectors, verify that the selector is valid relative to pod spec and containes container name for the current container.
2. For APIs with partial json path selectors, verify that the selector is valid relative to container spec. Also verify that volume's path name contains container name and its valid.
