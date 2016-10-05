<!-- BEGIN MUNGE: UNVERSIONED_WARNING -->

<!-- BEGIN STRIP_FOR_RELEASE -->

<img src="http://kubernetes.io/kubernetes/img/warning.png" alt="WARNING"
     width="25" height="25">
<img src="http://kubernetes.io/kubernetes/img/warning.png" alt="WARNING"
     width="25" height="25">
<img src="http://kubernetes.io/kubernetes/img/warning.png" alt="WARNING"
     width="25" height="25">
<img src="http://kubernetes.io/kubernetes/img/warning.png" alt="WARNING"
     width="25" height="25">
<img src="http://kubernetes.io/kubernetes/img/warning.png" alt="WARNING"
     width="25" height="25">

<h2>PLEASE NOTE: This document applies to the HEAD of the source tree</h2>

If you are using a released version of Kubernetes, you should
refer to the docs that go with that version.

<!-- TAG RELEASE_LINK, added by the munger automatically -->
<strong>
The latest release of this document can be found
[here](http://releases.k8s.io/release-1.4/docs/design/websockets-watches-proposal.md).

Documentation for other releases can be found at
[releases.k8s.io](http://releases.k8s.io).
</strong>
--

<!-- END STRIP_FOR_RELEASE -->

<!-- END MUNGE: UNVERSIONED_WARNING -->

# WebSockets Watches Proposal: DRAFT

## Abstract

This proposal aims to enable kubernetes watch APIs to allow watching more than one resource
over a WebSocket connection to a single domain.

## Background

Kubernetes provides to its clients watch APIs that help them to be notified of any changes to kubernetes
resources (such as pods, replication controllers, nodes, services etc.) when they happen. These watch APIs
can be accessed over HTTP(S) or WebSocket (secure, non-secure) connections. However, due to the nature
(notifications, updates like pushes from a server) of watches, WebSockets are more suitable for them and
hence are used more commonly. As an example, Openshift (built on kubernetes) web client uses WebSockets
for accessing watch APIs.

## Problem Statement

Web based clients, like web browsers (IE, Chrome, and most likely others too), limit the number of open
WebSocket connections to a single domain to 6 connections. As the current watch APIs only allow watching
one resource per connection, this means that WebSocket connections to watch resources are then limited to
watching only 6 different resources. Since the inception of kubernetes, its resource types have grown to
a large number (for example PetSets/ReplicaSets/Deployments) and are expected to keep growing in the near
future too. Due to this growth, the limitation to watching 6 resources at one time is becoming a major
concern for web based clients.

## Motivation

To allow kubernetes' web based clients to be able to watch more than 6 resources simultaneously
to a single domain.

## Use cases

1. A user/admin needing to watch on multiple resources through a single WebSocket connection.
2. 1. web based system monitoring where it may be desired to watch more than 6 resources simultaneously
to have a more comprehensive view of system health.

## Solutions

Here we discuss two solutions to address the problem.

### A watch request with multiple resources 

The steps in this solution are:

* Clients should be able to specify watch request as follows:

```
/api/v1/watch/pods,nodes,rc,services
/api/v1/watch/pods?resourceVersion=4&timeoutSeconds=319,nodes?resourceVersion=6&timeoutSeconds=280
```

  A client opens a WebSocket connection to a kube api server and sends the above watch request to the server.

* The API server's resthandler in `pkg/apiserver/resthandler.go` identifies it as a watch on multiple resources. 
The request is routed to WebSocket handler `in pkg/apiserver/watch.go` via `serveWatch()` in `pkg/apiserver/resthandler.go`.

* The `serveWatch()` in `pkg/apiserver/resthandler.go` parses the request and creates a WebSocket
server to handles multiple watches. 

* The WebSocket server in `pkg/apiserver/watch.go` creates a slice of watches for resources.

```
type WatchServer struct {
watching []watch.Interface
.
.
}
```

* The WebSocket handler HandleWS in `pkg/apiserver/watch.go` handles all watches.

#### Validation

The resthandler validates that requested resources are indeed one of the existing kube resources.

#### Limiting number of resources per request

This solution also requires limiting number of resources to watch per request, as allowing
number of resources without upper bound would lead to an increased amount of response data that
might overwhelm the connection.

```
type WatchServer struct {
watching []watch.Interface
.
.
maxAllowedRequest int32
}
```

Where `maxAllowedRequest` could be something `5` or `10` (or something else).

### Multiple watch requests at different times over a WebSocket Connection

In this solution, the expected steps are:

1. A client opens a WebSocket connection to a Kubernetes API server.

2. The client then sends a watch request for a particular resource over the connection.

3. The api server responds to the request.

4. The client then may open another watch request on the same connection for a different resource
while the previous watch is still open over the same connection.

5. The api server responds to the another watch request over the same connection.

6. The steps 4 and 5 are repeated as needed.

7. The server may close the connection if all watches are finished for whatever reason or
the client closes the connection.

This solution is different from the previous one because watch requests are not sent
in bulk in the begining as in the previous solution.

#### Implementation details

To-Do

## Open Issues

### Impact of more watches on etcd

In kubernetes, when a client sends a watch request to kubernetes apiserver, the apiserver opens an watch on etcd
for each watch request. With the growing number of resource types in kubernetes, and the need for watching more
resources simultaneously, it is going to put even more burden on etcd's performance and kubernetes performance
in general. This performance impact on etcd needs to be addressed and there is a proposal [apiserver-watch](https://github.com/kubernetes/kubernetes/blob/release-1.4/docs/proposals/apiserver-watch.md)
that deals with it.

Although the above solution requires opening more watches simultaneously, the current/short plan for now is to not
the addresses the performance impact on etcd (if any) as part of this proposal.

### Security Considerations

What client/user is allowed to request watches on what resources.

## Roadmap

1. The firt solution above for kube 1.5.
2. The other solution for kube 1.6 and beyond.

## References

1. https://github.com/kubernetes/kubernetes/issues/1685
2. https://github.com/kubernetes/kubernetes/blob/release-1.4/docs/proposals/apiserver-watch.md

<!-- BEGIN MUNGE: GENERATED_ANALYTICS -->
[![Analytics](https://kubernetes-site.appspot.com/UA-36037335-10/GitHub/docs/design/websockets-watches-proposal.md?pixel)]()
<!-- END MUNGE: GENERATED_ANALYTICS -->
