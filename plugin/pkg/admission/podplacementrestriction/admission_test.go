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

package podplacementrestriction

import (
	"encoding/json"
	"testing"
	"time"

	"k8s.io/kubernetes/pkg/admission"
	"k8s.io/kubernetes/pkg/api"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/fake"
	"k8s.io/kubernetes/pkg/controller/framework/informers"
	"k8s.io/kubernetes/pkg/util/wait"
)

// TestPodAdmission verifies various scenarios involving pod/namespace tolerations
func TestPodAdmission(t *testing.T) {
	namespace := &api.Namespace{
		ObjectMeta: api.ObjectMeta{
			Name:      "testNamespace",
			Namespace: "",
		},
	}

	mockClient := &fake.Clientset{}
	handler, informerFactory, err := newHandlerForTest(mockClient)
	if err != nil {
		t.Errorf("unexpected error initializing handler: %v", err)
	}
	informerFactory.Start(wait.NeverStop)

	pod := &api.Pod{
		ObjectMeta: api.ObjectMeta{Name: "testPod", Namespace: "testNamespace"},
	}

	tests := []struct {
		defaultClusterTolerations  []api.Toleration
		namespaceTolerations       []api.Toleration
		podTolerations             []api.Toleration
		mergedTolerations          []api.Toleration
		ignoreNamespaceTolerations bool
		admit                      bool
		testName                   string
	}{
		{
			defaultClusterTolerations:  []api.Toleration{{"testKey", "", "testValue", ""}},
			namespaceTolerations:       []api.Toleration{},
			podTolerations:             []api.Toleration{{"testKey", "", "testValue", ""}},
			mergedTolerations:          []api.Toleration{{"testKey", "", "testValue", ""}},
			ignoreNamespaceTolerations: false,
			admit:    true,
			testName: "default cluster tolerations",
		},
		{
			defaultClusterTolerations:  []api.Toleration{},
			namespaceTolerations:       []api.Toleration{{"testKey", "", "testValue", ""}},
			podTolerations:             []api.Toleration{{"testKey", "", "testValue", ""}},
			mergedTolerations:          []api.Toleration{{"testKey", "", "testValue", ""}},
			ignoreNamespaceTolerations: false,
			admit:    true,
			testName: "namespace tolerations",
		},
		{
			defaultClusterTolerations:  []api.Toleration{},
			namespaceTolerations:       []api.Toleration{{"testKey", "", "testValue", ""}},
			podTolerations:             []api.Toleration{},
			mergedTolerations:          []api.Toleration{{"testKey", "", "testValue", ""}},
			ignoreNamespaceTolerations: false,
			admit:    true,
			testName: "no pod tolerations",
		},
	}
	for _, test := range tests {
		if !test.ignoreNamespaceTolerations && len(test.namespaceTolerations) > 0 {
			tolerationStr, err := json.Marshal(test.namespaceTolerations)
			if err != nil {
				t.Errorf("error in marshalling namespaceTolerations %v", test.namespaceTolerations)
			}
			namespace.Annotations = map[string]string{api.TolerationsAnnotationKey: string(tolerationStr)}
		}
		handler.clusterDefaultTolerations = test.defaultClusterTolerations

		if len(test.podTolerations) > 0 {
			podTolerationStr, err := json.Marshal(test.podTolerations)
			if err != nil {
				t.Errorf("error in marshalling podTolerations %v", test.podTolerations)
			}
			pod.Annotations = map[string]string{api.TolerationsAnnotationKey: string(podTolerationStr)}
		}

		err := handler.Admit(admission.NewAttributesRecord(pod, nil, api.Kind("Pod").WithVersion("version"), "testNamespace", namespace.ObjectMeta.Name, api.Resource("pods").WithVersion("version"), "", admission.Create, nil))
		if test.admit && err != nil {
			t.Errorf("Test: %s, expected no error but got: %s", test.testName, err)
		} else if !test.admit && err == nil {
			t.Errorf("Test: %s, expected an error", test.testName)
		}

		updatedPodTolerations, err := api.GetTolerationsFromAnnotations(pod.Annotations)
		if err != nil {
			t.Errorf("error in unmarshalling merged pod tolerations %s", pod.Annotations[api.TolerationsAnnotationKey])
		}

		if test.admit && !equalTolerations(updatedPodTolerations, test.mergedTolerations) {
			t.Errorf("Test: %s, expected: %#v but got: %#v", test.testName, test.mergedTolerations, updatedPodTolerations)
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
		ptPlugin := NewPodTolerationsPlugin(nil, nil)
		if e, a := shouldHandle, ptPlugin.Handles(op); e != a {
			t.Errorf("%v: shouldHandle=%t, handles=%t", op, e, a)
		}
	}
}

// newHandlerForTest returns the admission controller configured for testing.
func newHandlerForTest(c clientset.Interface) (*podTolerationsPlugin, informers.SharedInformerFactory, error) {
	f := informers.NewSharedInformerFactory(c, 5*time.Minute)
	handler := NewPodTolerationsPlugin(c, nil)
	plugins := []admission.Interface{handler}
	pluginInitializer := admission.NewPluginInitializer(f)
	pluginInitializer.Initialize(plugins)
	err := admission.Validate(plugins)
	return handler, f, err
}
