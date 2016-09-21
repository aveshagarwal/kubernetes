/*
Copyright 2016 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package nodeenv

import (
	"fmt"
	"io"
	"reflect"

	"github.com/golang/glog"

	"k8s.io/kubernetes/pkg/admission"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/client/cache"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/yaml"
	"k8s.io/kubernetes/pkg/watch"
)

var NamespaceNodeSelectors = []string{"kubernetes.io/node-selector"}

func init() {
	admission.RegisterPlugin("PodNodeEnvironment", func(client clientset.Interface, config io.Reader) (admission.Interface, error) {
		pluginConfig := readConfig(config)
		plugin := NewPodNodeEnvironment(client, pluginConfig.PodNodeEnvironmentPluginConfig["clusterDefaultNodeSelector"])
		plugin.reflector.Run()
		return plugin, nil
	})
}

// podNodeEnvironment is an implementation of admission.Interface.
type podNodeEnvironment struct {
	*admission.Handler
	client    clientset.Interface
	store     cache.Store
	reflector *cache.Reflector
	// global default node selector in a cluster, If a namespace is
	// not assigned any node selector, it gets this by default.
	clusterDefaultNodeSelector string
}

type pluginConfig struct {
	PodNodeEnvironmentPluginConfig map[string]string
}

// readConfig reads default value of clusterDefaultNodeSelector
// from the file provided with --admission-control-config-file
// If the file is not supplied, it defaults to ""
// The format in a file:
// podNodeEnvironmentPluginConfig:
//  clusterDefaultNodeSelector: <node-selectors-labels>
func readConfig(config io.Reader) *pluginConfig {
	defaultConfig := &pluginConfig{}
	if config == nil || reflect.ValueOf(config).IsNil() {
		return defaultConfig
	}
	d := yaml.NewYAMLOrJSONDecoder(config, 4096)
	for {
		if err := d.Decode(defaultConfig); err != nil {
			if err != io.EOF {
				continue
			}
		}
		break
	}
	return defaultConfig
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
		glog.Errorf("expected pod but got %s", a.GetKind().Kind)
		return nil
	}

	name := pod.Name
	nsName := a.GetNamespace()
	var namespace *api.Namespace

	namespaceObj, exists, err := p.store.Get(&api.Namespace{
		ObjectMeta: api.ObjectMeta{
			Name:      nsName,
			Namespace: "",
		},
	})
	if err != nil {
		return errors.NewInternalError(err)
	}
	if exists {
		namespace = namespaceObj.(*api.Namespace)
	} else {
		namespace, err = p.defaultGetNamespace(nsName)
		if err != nil {
			if errors.IsNotFound(err) {
				return err
			}
			return errors.NewInternalError(err)
		}
	}
	namespaceNodeSelector, err := p.getNodeSelectorMap(namespace)
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

func NewPodNodeEnvironment(client clientset.Interface, clusterDefaultNodeSelector string) *podNodeEnvironment {
	// TODO: make it a shared cache to use between admission plugins
	store := cache.NewStore(cache.MetaNamespaceKeyFunc)
	reflector := cache.NewReflector(
		&cache.ListWatch{
			ListFunc: func(options api.ListOptions) (runtime.Object, error) {
				return client.Core().Namespaces().List(options)
			},
			WatchFunc: func(options api.ListOptions) (watch.Interface, error) {
				return client.Core().Namespaces().Watch(options)
			},
		},
		&api.Namespace{},
		store,
		0,
	)
	return &podNodeEnvironment{
		Handler: admission.NewHandler(admission.Create),
		client:  client,
		store:   store,
		clusterDefaultNodeSelector: clusterDefaultNodeSelector,
		reflector:                  reflector,
	}
}

func (p *podNodeEnvironment) defaultGetNamespace(name string) (*api.Namespace, error) {
	namespace, err := p.client.Core().Namespaces().Get(name)
	if err != nil {
		return nil, fmt.Errorf("namespace %s does not exist", name)
	}
	return namespace, nil
}

func (p *podNodeEnvironment) getNodeSelectorMap(namespace *api.Namespace) (map[string]string, error) {
	selector := map[string]string{}
	labelsMap := map[string]string{}
	var err error
	found := false
	if len(namespace.ObjectMeta.Annotations) > 0 {
		for _, annotation := range NamespaceNodeSelectors {
			if ns, ok := namespace.ObjectMeta.Annotations[annotation]; ok {
				labelsMap, err = labels.ConvertSelectorToLabelsMap(ns)
				if err != nil {
					return map[string]string{}, err
				}

				if labels.Conflicts(selector, labelsMap) {
					nsName := namespace.ObjectMeta.Name
					return map[string]string{}, fmt.Errorf("%s annotations' node label selectors conflict", nsName)
				}
				selector = labels.Merge(selector, labelsMap)
				found = true
			}
		}
	}
	if !found {
		selector, err = labels.ConvertSelectorToLabelsMap(p.clusterDefaultNodeSelector)
		if err != nil {
			return map[string]string{}, err
		}
	}
	return selector, nil
}
