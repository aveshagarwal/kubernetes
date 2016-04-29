package nodeenv

import (
	"fmt"
	"io"

	"k8s.io/kubernetes/pkg/admission"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/errors"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/labels"
)

func init() {
	admission.RegisterPlugin("PodNodeEnvironment", func(client clientset.Interface, config io.Reader) (admission.Interface, error) {
		return NewPodNodeEnvironment(client, nil)
	})
}

const (
	NamespaceNodeSelector = "kubernetes.io/node-selector"
	DefaultNodeSelector   = ""
)

// podNodeEnvironment is an implementation of admission.Interface.
type podNodeEnvironment struct {
	*admission.Handler
	client  clientset.Interface
	nsCache NamespaceCache
}

// Admit enforces that pod and its namespace node label selectors matches at least a node in the cluster.
func (p *podNodeEnvironment) Admit(a admission.Attributes) error {
	resource := a.GetResource().GroupResource()
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
	nsName := a.GetNamespace()

	var namespace *api.Namespace
	var err error
	if p.nsCache != nil {
		namespace, err = p.nsCache.GetNamespace(nsName)
		if err != nil {
			return err
		}
	}

	if namespace == nil {
		namespace, err = p.DefaultGetNamespace(nsName)
		if err != nil {
			return err
		}
	}

	namespaceNodeSelector, err := GetNodeSelectorMap(namespace)
	if err != nil {
		return err
	}

	if labels.Conflicts(namespaceNodeSelector, pod.Spec.NodeSelector) {
		return errors.NewForbidden(resource, name, fmt.Errorf("pod node label selector conflicts with its namespace node label selector"))
	}

	// modify pod node selector = namespace node selector + current pod node selector
	pod.Spec.NodeSelector = labels.Merge(namespaceNodeSelector, pod.Spec.NodeSelector)

	return nil
}

func NewPodNodeEnvironment(client clientset.Interface, nsCache NamespaceCache) (admission.Interface, error) {
	return &podNodeEnvironment{
		Handler: admission.NewHandler(admission.Create),
		client:  client,
		nsCache: nsCache,
	}, nil
}

func (p *podNodeEnvironment) DefaultGetNamespace(name string) (*api.Namespace, error) {
	namespace, err := p.client.Core().Namespaces().Get(name)
	if err != nil {
		return nil, fmt.Errorf("namespace %s does not exist", name)
	}
	return namespace, nil
}

func GetNodeSelectorMap(namespace *api.Namespace) (map[string]string, error) {
	selector := ""
	found := false
	if len(namespace.ObjectMeta.Annotations) > 0 {
		if ns, ok := namespace.ObjectMeta.Annotations[NamespaceNodeSelector]; ok {
			selector = ns
			found = true
		}
	}
	if !found {
		selector = DefaultNodeSelector
	}

	labelsMap, err := labels.ConvertSelectortoLabelsMap(selector)
	if err != nil {
		return map[string]string{}, err
	}
	return labelsMap, nil
}
