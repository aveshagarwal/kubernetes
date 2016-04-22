<!-- BEGIN MUNGE: UNVERSIONED_WARNING -->

<!-- BEGIN STRIP_FOR_RELEASE -->

<img src="http://kubernetes.io/img/warning.png" alt="WARNING"
     width="25" height="25">
<img src="http://kubernetes.io/img/warning.png" alt="WARNING"
     width="25" height="25">
<img src="http://kubernetes.io/img/warning.png" alt="WARNING"
     width="25" height="25">
<img src="http://kubernetes.io/img/warning.png" alt="WARNING"
     width="25" height="25">
<img src="http://kubernetes.io/img/warning.png" alt="WARNING"
     width="25" height="25">

<h2>PLEASE NOTE: This document applies to the HEAD of the source tree</h2>

If you are using a released version of Kubernetes, you should refer to
the docs that go with that version.

Documentation for other releases can be found at
[releases.k8s.io](http://releases.k8s.io).
</strong>
--

<!-- END STRIP_FOR_RELEASE -->

<!-- END MUNGE: UNVERSIONED_WARNING -->

# Downward API for resource limits and requests

## Background

Currently the downward API (via environment variables and volume plugin) only
supports exposing a Pod's name, namespace, annotations, labels and its IP
([see details](http://kubernetes.io/docs/user-guide/downward-api/)). This
document explains the need and design to extend them to expose resources
(e.g. cpu, memory) limits and requests.

## Motivation

Software applications require configuration to work optimally with the resources they're allowed to use.
Exposing the requested and limited amounts of available resources inside containers will allow
these applications to be configured more easily. Although docker already
exposes some of this information inside containers, the downward API helps
exposing this information in a runtime-agnostic manner in Kubernetes.

## Design

This is mostly driven by the discussion in [this issue](https://github.com/kubernetes/kubernetes/issues/9473).
There are three approaches discussed in this document to obtain resources limits
and requests to be exposed as environment variables and volumes inside
containers:

1. The first approach requires users to specify full json path selectors
in which selectors are relative to the pod spec. The benefit of this
approach is to specify pod level resources, and since containers are
also part of a pod spec, it can be used to specify container level
resources too.

2. The second approach requires specifying partial json path selectors
which are relative to the container spec. This approach helps
in retrieving a container specific resource limits and requests, and at
the same time, it is simpler to specify than full json path selectors.

3. In this approach, users specify fixed strings to retrieve
resources limits and requests and do not specify any json path
selectors. This approach is similar to the existing downward API
implementation approach. The advantages of this approach is that it is
simpler to specify that the first two, and does not require any type of
conversion between internal and versioned objects or json selectors as
discussed below.

Before discussing a bit more about merits of each approach, here is a
brief discussion about json path selectors and some implications related
to their use.

#### JSONpath selectors

Versioned objects in kubernetes have json tags as part of their golang fields.
Although currently internal objects in kubernetes also have json tags but these
tags should be removed in future (see [3933](https://github.com/kubernetes/kubernetes/issues/3933)
for discussion). So for discussion in this proposal, we assume that
internal objects do not have json tags. In the first two approaches
(full and partial json selectors), when a user creates a pod and its
containers, the user specifies a json path selector in the pod's
spec to retrieve values of its limits and requests. The selector
is composed of json tags similar to json paths used with kubectl
([json](http://kubernetes.io/docs/user-guide/jsonpath/)). This proposal
uses kubernetes' json path library to process the selectors to retrieve
the values. As kubelet operates on internal objects (without json tags),
and the selectors are part of versioned objects, retrieving values of
the limits and requests can be handled using these two solutions:

1. By converting an internal object to versioned obejct, and then using
the json path library to retrieve the values from the versioned object
by processing the selector.

2. By converting a json selector of the versioned objects to internal
object's golang expression and then using the json path library to
retrieve the values from the internal object by processing the golang
expression. However, converting a json selector of the versioned objects
to internal object's golang expression will still require an instance
of the versioned object, so it seems more work from the first solution
unless there is another way without requiring the versioned object.

So there is a one time conversion cost associated with the first (full
path) and second (partial path) approaches, whereas the third approach
(no selectors) does not require any such conversion and can directly
work on internal objects. If we want to avoid conversion cost and to
have implementation simplicity, my opinion is that no selector approach
is relatively easiest to implement to expose limits and requests with
least impact on existing functionality.

To summarize merits/demerits of each approach:

|Approach | Scope | Conversion cost | JSON selectors | Future extension|
| ---------- | ------------------- | -------------------| ------------------- | ------------------- |
|Full selectors | Pod/Container | Yes | Yes | Possible |
|Partial selectors | Container | Yes | Yes | Possible |
|No selectors | Pod/Container | No | No | Possible|

### API with full json path selectors

Full json path selectors specify the complete path to the resources
limits and requests relative to pod spec.

#### Environment variables

This table shows how selectors can be used for various requests and
limits to be exposed as environment variables.

| Name | Selectors |
| ---- | ------------------- |
| CPU_LIMIT | spec.containers[?(@.name=="container-name")].resources.limits.cpu|
| MEMORY_LIMIT | spec.containers[?(@.name=="container-name")].resources.limits.memory|
| CPU_REQUEST | spec.containers[?(@.name=="container-name")].resources.requests.cpu|
| MEMORY_REQUEST | spec.containers[?(@.name=="container-name")].resources.requests.memory |

#### Volume plugin

This table shows how selectors can be used for various requests and
limits to be exposed as volumes.

| Path | Selectors |
| ---- | ------------------- |
| cpu_limit | spec.containers[?(@.name=="container-name")].resources.limits.cpu|
| memory_limit| spec.containers[?(@.name=="container-name")].resources.limits.memory|
| cpu_request | spec.containers[?(@.name=="container-name")].resources.requests.cpu|
| memory_request |spec.containers[?(@.name=="container-name")].resources.limits.memory|

Volumes are pod scoped, so a selector should be specified with a
particular container name.

Note: environment variables and volume path names are examples only and
not necessarily as specified above, and the selectors do not have to
start with dot.

Full json path selectors will use existing `type ObjectFieldSelector`
to extend the current implementation for resources requests and limits.

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
      command: [ "/bin/sh","-c", "env" ]
      resources:
        requests:
          memory: "64Mi"
          cpu: "250m"
        limits:
          memory: "128Mi"
          cpu: "500m"
      env:
        - name: CPU_LIMIT
          valueFrom:
            fieldRef:
              fieldPath: spec.containers[?(@.name=="container-name")].resources.limits.cpu
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
      command: ["sh", "-c", "while true; do if [[ -e /etc/labels ]]; then cat /etc/labels; fi; if [[ -e /etc/annotations ]]; then cat /etc/annotations; fi;sleep 5; done"]
      resources:
        requests:
          memory: "64Mi"
          cpu: "250m"
        limits:
          memory: "128Mi"
          cpu: "500m"
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
              fieldPath: spec.containers[?(@.name=="container-name")].resources.limits.cpu
```
#### Validations

For APIs with full json path selectors, verify that the selector is
valid relative to pod spec.


### API with partial json path selectors

Partial json path selectors specify paths to resources limits and requests
relative to the container spec.

#### Environment variables

This table shows how partial selectors can be used for various requests and
limits to be exposed as environment variables.

| Env Var Name | Container Field Reference |
| -------------------- | -------------------|
| CPU_LIMIT | container-name|resources.limits.cpu |
| MEMORY_LIMIT | resources.limits.memory |
| CPU_REQUEST | resources.requests.cpu |
| MEMORY_REQUEST | resources.requests.memory |

Since environment variables are container scoped, there is no need
to specify container name as part of the partial selectors as they are
relative to container spec.

#### Volume plugin

| Path | Selectors |
| -------------------- | -------------------|
| cpu_limit | resources.limits.cpu |
| memory_limit | resources.limits.memory |
| cpu_request | resources.requests.cpu |
| memory_request | resources.requests.memory |

Since environment variables are container scoped, the container name must
be specified as part of their path name. The format is that container
name and path name must be separated by slash (`/`).

Note: Also, environment variables and volume path names are examples
only and not necessarily as specified above, and the selectors do not
have to start with dot.

Partial json selectors will be implemented by introducing
`containerFieldRef` to extend the current implementation for resources
requests and limits, and will be part of `type DownwardAPIVolumeFile`
and `type EnvVarSource` as follows:

```
// Represents a single file containing information from the downward API
type DownwardAPIVolumeFile struct {
     // Required: Path is  the relative path name of the file to be created.
     Path string `json:"path"`
     // Selects a field of the pod: only annotations, labels, name and  namespace are supported.
     FieldRef *ObjectFieldSelector `json:"fieldRef, omitempty"`
     // Selects a field of the container: only resources limits and requests
     // (cpu, memory) are currently supported.
     ContainerFieldRef *ObjectFieldSelector `json:"containerFieldRef,omitempty"`
}


// EnvVarSource represents a source for the value of an EnvVar.
// Only one of its fields may be set.
type EnvVarSource struct {
   // Required: Selects a field of the container: only resources limits and requests (cpu, memory) are supported.
   ContainerFieldRef *ObjectFieldSelector `json:"containerFieldRef,omitempty"`
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
      image: gcr.io/google_containers/busybox command: [ "/bin/sh","-c", "env" ]
      resources:
        requests:
          memory: "64Mi"
          cpu: "250m"
        limits:
          memory: "128Mi"
          cpu: "500m"
      env:
        - name: CPU_LIMIT
          valueFrom:
            containerFieldRef:
              fieldPath: resources.limits.cpu
```

```
apiVersion: v1
kind: Pod
metadata:
  name: kubernetes-downwardapi-volume-example
spec:
  containers:
    - name: client-container
      image: gcr.io/google_containers/busybox command: ["sh", "-c", "while true; do if [[ -e /etc/labels ]]; then cat /etc/labels; fi; if [[ -e /etc/annotations ]]; then cat /etc/annotations; fi; sleep 5; done"]
     resources:
        requests:
          memory: "64Mi"
	  cpu: "250m"
        limits:
          memory: "128Mi"
          cpu: "500m"
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
#### Validations

For APIs with partial json path selectors, verify
that the selector is valid relative to container spec.


### API with no selectors

In this approach, users specify particular strings to retrieve resources
limits and requests. This approach is similar to the existing downward
API implementation approach.

#### Environment variables

This table shows how selectors can be used for various requests and
limits to be exposed as environment variables.

| Name | Selectors |
| -------------------- | -------------------|
| CPU_LIMIT | resources.limits.cpu |
| MEMORY_LIMIT |resources.limits.memory |
| CPU_REQUEST | resources.requests.cpu |
|MEMORY_REQUEST | resources.requests.memory |

Since environment variables are container scoped, there is no need
to specify container name as part of this approach as they are relative
to container spec.

#### Volume plugin

| Path | Selectors |
| -------------------- | -------------------|
| container_name/cpu_limit | resources.limits.cpu |
|container_name/memory_limit | resources.limits.memory|
|container_name/cpu_request | resources.requests.cpu |
|container_name/memory_request | resources.requests.memory |

Please note that in this case, the selectors specified are similar to
partial selector approach, but there is a key difference between these
selectors and partial selectors how they are processed. Here the selectors
are processed as fix strings, whereas in partial selectors approach, the
similar selectors are processed as a json path over a versioned object.

Since environment variables are container scoped, the container name must
be specified as part of their path name. The format is that container
name and path name must be separated by slash (`/`). This approach could
be used for pod level resources in the future by not specifying container
name as part of the path.

Note: Also, environment variables and volume path names are examples
only and not necessarily as specified above.

This API will be implemented by introducing `resourceFieldRef` to extend
the current implementation for resources requests and limits, and will be
part of `type DownwardAPIVolumeFile` and `type EnvVarSource` as follows:

```
// Represents a single file containing information from the downward API
type DownwardAPIVolumeFile struct {
        // Required: Path is  the relative path name of the file to be created. Must not be absolute or contain the '..' path. Must be utf-8 encoded. The first item of the relative path must not start with '..'
        Path string `json:"path"`
        // Required: Selects a field of the pod: only annotations, labels, name and namespace are supported.
        FieldRef *ObjectFieldSelector `json:"fieldRef,omitempty"`
        // Required: Selects a field of the container: only resources limits and requests (cpu, memory) are supported.
        ResourceFieldRef *ObjectFieldSelector `json:"resourceFieldRef,omitempty"`
}


// EnvVarSource represents a source for the value of an EnvVar.
// Only one of its fields may be set.
type EnvVarSource struct {
        // Required: Selects a field of the container: only resources limits and requests (cpu, memory) are supported.
        ResourceFieldRef *ObjectFieldSelector `json:"resourceFieldRef,omitempty"`
        //Selects a field of the pod; only name and namespace are supported.
        FieldRef *ObjectFieldSelector `json:"fieldRef,omitempty"`
        // Selects a key of a ConfigMap.  ConfigMapKeyRef
        *ConfigMapKeySelector `json:"configMapKeyRef,omitempty"`
        //Selects a key of a secret in the pod's namespace.
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
      image: gcr.io/google_containers/busybox command: [ "/bin/sh","-c", "env" ]
      resources:
        requests:
          memory: "64Mi"
          cpu: "250m"
        limits:
          memory: "128Mi"
          cpu: "500m"
      env:
        - name: CPU_LIMIT
          valueFrom:
            resourceFieldRef:
              fieldPath: resources.limits.cpu
```

```
apiVersion: v1
kind: Pod
metadata:
  name: kubernetes-downwardapi-volume-example
spec:
  containers:
    - name: client-container
      image: gcr.io/google_containers/busybox command: ["sh", "-c","while true; do if [[ -e /etc/labels ]]; then cat /etc/labels; fi; if [[ -e /etc/annotations ]]; then cat /etc/annotations; fi; sleep 5; done"]
      resources:
        requests:
          memory: "64Mi"
          cpu: "250m"
        limits:
          memory: "128Mi"
          cpu: "500m"
      volumeMounts:
        - name: podinfo
          mountPath: /etc
          readOnly: false
  volumes:
    - name: podinfo
      downwardAPI:
        items:
          - path: "container_name/cpu_limit"
            resourceFieldRef:
              fieldPath: resources.limits.cpu
```
#### Validations

For APIs with no selectors, verify that the selector is valid and is one
of cpu/request_limit and cpu/memory_request.

## Output Format

The output format for resources limits and requests will be same as
cgroups output format, i.e. cpu in cpu shares (cores multiplied by 1024
and rounded to integer) and memory in bytes. For example, memory request
or limit of `64Mi` in the container spec will be output as `67108864`
bytes, and cpu request or limit of `250m` (milicores) will be output as
`256` of cpu shares.


<!-- BEGIN MUNGE: GENERATED_ANALYTICS -->
[![Analytics](https://kubernetes-site.appspot.com/UA-36037335-10/GitHub/docs/design/downward_api_resources_limits_requests.md?pixel)]()
<!-- END MUNGE: GENERATED_ANALYTICS -->
