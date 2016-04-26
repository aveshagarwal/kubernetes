package nodeenv

import (
	"fmt"
	"io"

	"k8s.io/kubernetes/pkg/admission"
	"k8s.io/kubernetes/pkg/api"
	apierrors "k8s.io/kubernetes/pkg/api/errors"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"

	"k8s.io/kubernetes/plugin/pkg/admission/nodeenv/labelselector"
)

func init() {
	admission.RegisterPlugin("PodNodeEnvironment", func(client clientset.Interface, config io.Reader) (admission.Interface, error) {
		return NewPodNodeEnvironment(client, nil)
	})
}

const (
	ProjectNodeSelector = "kubernetes.io/node-selector"
	DefaultNodeSelector = ""
)

// podNodeEnvironment is an implementation of admission.Interface.
type podNodeEnvironment struct {
	*admission.Handler
	client    clientset.Interface
	namespace GetNamespaceCache
}

// Admit enforces that pod and its project node label selectors matches at least a node in the cluster.
func (p *podNodeEnvironment) Admit(a admission.Attributes) (err error) {
	resource := a.GetResource()
	if resource != api.Resource("pods") {
		return nil
	}
	if a.GetSubresource() != "" {
		// only run the checks below on pods proper and not subresources
		return nil
	}

	obj := a.GetObject()
	pod, ok := obj.(*api.Pod)
	if !ok {
		return nil
	}

	name := pod.Name

	if p.namespace == nil {
		p.namespace = &DefaultGetNamespaceCache{client: p.client}
	}

	namespace, err := p.namespace.GetNamespace(a.GetNamespace())
	if err != nil {
		return err
	}

	projectNodeSelector, err := GetNodeSelectorMap(namespace)
	if err != nil {
		return err
	}

	if labelselector.Conflicts(projectNodeSelector, pod.Spec.NodeSelector) {
		return apierrors.NewForbidden(resource, name, fmt.Errorf("pod node label selector conflicts with its project node label selector"))
	}

	// modify pod node selector = project node selector + current pod node selector
	pod.Spec.NodeSelector = labelselector.Merge(projectNodeSelector, pod.Spec.NodeSelector)

	return nil
}

func NewPodNodeEnvironment(client clientset.Interface, nsCache GetNamespaceCache) (admission.Interface, error) {

	return &podNodeEnvironment{
		Handler:   admission.NewHandler(admission.Create),
		client:    client,
		namespace: nsCache,
	}, nil
}

type DefaultGetNamespaceCache struct {
	client clientset.Interface
}

// ensure DefaultGetNamespacecache implements the GetNamespaceCache interface.
var _ GetNamespaceCache = &DefaultGetNamespaceCache{}

func (d *DefaultGetNamespaceCache) GetNamespace(name string) (*api.Namespace, error) {
	namespace, err := d.client.Core().Namespaces().Get(name)
	if err != nil {
		return nil, fmt.Errorf("namespace %s does not exist", name)
	}
	return namespace, nil
}

func GetNodeSelector(namespace *api.Namespace) string {
	selector := ""
	found := false
	if len(namespace.ObjectMeta.Annotations) > 0 {
		if ns, ok := namespace.ObjectMeta.Annotations[ProjectNodeSelector]; ok {
			selector = ns
			found = true
		}
	}
	if !found {
		selector = DefaultNodeSelector
	}
	return selector
}

func GetNodeSelectorMap(namespace *api.Namespace) (map[string]string, error) {
	selector := GetNodeSelector(namespace)
	labelsMap, err := labelselector.Parse(selector)
	if err != nil {
		return map[string]string{}, err
	}
	return labelsMap, nil
}
