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
	"testing"

	"k8s.io/kubernetes/pkg/admission"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/cache"
	clientsetfake "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/fake"
	"k8s.io/kubernetes/pkg/labels"
)

// TestPodAdmission verifies various scenarios involving pod/namespace/global node label selectors
func TestPodAdmission(t *testing.T) {
	namespace := &api.Namespace{
		ObjectMeta: api.ObjectMeta{
			Name:      "testNamespace",
			Namespace: "",
		},
	}

	store := cache.NewStore(cache.MetaNamespaceKeyFunc)
	store.Add(namespace)
	mockClientset := clientsetfake.NewSimpleClientset()
	handler := &podNodeEnvironment{client: mockClientset, store: store}
	pod := &api.Pod{
		ObjectMeta: api.ObjectMeta{Name: "testPod", Namespace: "testNamespace"},
	}

	tests := []struct {
		defaultNodeSelector             string
		namespaceNodeSelector           string
		podNodeSelector                 map[string]string
		mergedNodeSelector              map[string]string
		ignoreTestNamespaceNodeSelector bool
		admit                           bool
		testName                        string
	}{
		{
			defaultNodeSelector:             "",
			podNodeSelector:                 map[string]string{},
			mergedNodeSelector:              map[string]string{},
			ignoreTestNamespaceNodeSelector: true,
			admit:    true,
			testName: "No node selectors",
		},
		{
			defaultNodeSelector:             "infra = false",
			podNodeSelector:                 map[string]string{},
			mergedNodeSelector:              map[string]string{"infra": "false"},
			ignoreTestNamespaceNodeSelector: true,
			admit:    true,
			testName: "Default node selector and no conflicts",
		},
		{
			defaultNodeSelector:   "",
			namespaceNodeSelector: " infra = false ",
			podNodeSelector:       map[string]string{},
			mergedNodeSelector:    map[string]string{"infra": "false"},
			admit:                 true,
			testName:              "TestNamespace node selector with whitespaces and no conflicts",
		},
		{
			defaultNodeSelector:   "infra = false",
			namespaceNodeSelector: "",
			podNodeSelector:       map[string]string{},
			mergedNodeSelector:    map[string]string{},
			admit:                 true,
			testName:              "Empty namespace node selector and no conflicts",
		},
		{
			defaultNodeSelector:   "infra = false",
			namespaceNodeSelector: "infra=true",
			podNodeSelector:       map[string]string{},
			mergedNodeSelector:    map[string]string{"infra": "true"},
			admit:                 true,
			testName:              "Default and namespace node selector, no conflicts",
		},
		{
			defaultNodeSelector:   "infra = false",
			namespaceNodeSelector: "infra=true",
			podNodeSelector:       map[string]string{"env": "test"},
			mergedNodeSelector:    map[string]string{"infra": "true", "env": "test"},
			admit:                 true,
			testName:              "TestNamespace and pod node selector, no conflicts",
		},
		{
			defaultNodeSelector:   "env = test",
			namespaceNodeSelector: "infra=true",
			podNodeSelector:       map[string]string{"infra": "false"},
			admit:                 false,
			testName:              "Conflicting pod and namespace node selector, one label",
		},
		{
			defaultNodeSelector:   "env=dev",
			namespaceNodeSelector: "infra=false, env = test",
			podNodeSelector:       map[string]string{"env": "dev", "color": "blue"},
			admit:                 false,
			testName:              "Conflicting pod and namespace node selector, multiple labels",
		},
	}
	for _, test := range tests {
		if !test.ignoreTestNamespaceNodeSelector {
			namespace.ObjectMeta.Annotations = map[string]string{"kubernetes.io/node-selector": test.namespaceNodeSelector}
		}
		handler.store.Update(namespace)
		handler.clusterDefaultNodeSelector = test.defaultNodeSelector
		pod.Spec = api.PodSpec{NodeSelector: test.podNodeSelector}

		err := handler.Admit(admission.NewAttributesRecord(pod, nil, api.Kind("Pod").WithVersion("version"), "testNamespace", namespace.ObjectMeta.Name, api.Resource("pods").WithVersion("version"), "", admission.Create, nil))
		if test.admit && err != nil {
			t.Errorf("Test: %s, expected no error but got: %s", test.testName, err)
		} else if !test.admit && err == nil {
			t.Errorf("Test: %s, expected an error", test.testName)
		}

		if test.admit && !labels.Equals(test.mergedNodeSelector, pod.Spec.NodeSelector) {
			t.Errorf("Test: %s, expected: %s but got: %s", test.testName, test.mergedNodeSelector, pod.Spec.NodeSelector)
		}
	}
}

func TestHandles(t *testing.T) {
	for op, shouldHandle := range map[admission.Operation]bool{
		admission.Create:  true,
		admission.Update:  false,
		admission.Connect: false,
		admission.Delete:  false,
	} {
		nodeEnvionment := NewPodNodeEnvironment(nil, "")
		if e, a := shouldHandle, nodeEnvionment.Handles(op); e != a {
			t.Errorf("%v: shouldHandle=%t, handles=%t", op, e, a)
		}
	}
}
