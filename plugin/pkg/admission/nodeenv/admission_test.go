package nodeenv

import (
	"testing"

	"k8s.io/kubernetes/pkg/admission"
	"k8s.io/kubernetes/pkg/api"
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

	mockClientset := clientsetfake.NewSimpleClientset(namespace)
	handler := &podNodeEnvironment{client: mockClientset}
	pod := &api.Pod{
		ObjectMeta: api.ObjectMeta{Name: "testPod"},
	}

	tests := []struct {
		namespaceNodeSelector           string
		podNodeSelector                 map[string]string
		mergedNodeSelector              map[string]string
		ignoreTestNamespaceNodeSelector bool
		admit                           bool
		testName                        string
	}{
		{
			podNodeSelector:                 map[string]string{},
			mergedNodeSelector:              map[string]string{},
			ignoreTestNamespaceNodeSelector: true,
			admit:    true,
			testName: "No node selectors",
		},
		{
			namespaceNodeSelector: "infra=false",
			podNodeSelector:       map[string]string{},
			mergedNodeSelector:    map[string]string{"infra": "false"},
			admit:                 true,
			testName:              "Default node selector and no conflicts",
		},
		{
			namespaceNodeSelector: "infra=false",
			podNodeSelector:       map[string]string{},
			mergedNodeSelector:    map[string]string{"infra": "false"},
			admit:                 true,
			testName:              "TestNamespace node selector and no conflicts",
		},
		{
			namespaceNodeSelector: "",
			podNodeSelector:       map[string]string{},
			mergedNodeSelector:    map[string]string{},
			admit:                 true,
			testName:              "Empty namespace node selector and no conflicts",
		},
		{
			namespaceNodeSelector: "infra=true",
			podNodeSelector:       map[string]string{},
			mergedNodeSelector:    map[string]string{"infra": "true"},
			admit:                 true,
			testName:              "Default and namespace node selector, no conflicts",
		},
		{
			namespaceNodeSelector: "infra=true",
			podNodeSelector:       map[string]string{"env": "test"},
			mergedNodeSelector:    map[string]string{"infra": "true", "env": "test"},
			admit:                 true,
			testName:              "TestNamespace and pod node selector, no conflicts",
		},
		{
			namespaceNodeSelector: "infra=true",
			podNodeSelector:       map[string]string{"infra": "false"},
			mergedNodeSelector:    map[string]string{"infra": "false"},
			admit:                 false,
			testName:              "Conflicting pod and namespace node selector, one label",
		},
		{
			namespaceNodeSelector: "infra=false, env = test",
			podNodeSelector:       map[string]string{"env": "dev", "color": "blue"},
			mergedNodeSelector:    map[string]string{"env": "dev", "color": "blue"},
			admit:                 false,
			testName:              "Conflicting pod and namespace node selector, multiple labels",
		},
	}
	for _, test := range tests {
		if !test.ignoreTestNamespaceNodeSelector {
			namespace.ObjectMeta.Annotations = map[string]string{"kubernetes.io/node-selector": test.namespaceNodeSelector}
		}
		pod.Spec = api.PodSpec{NodeSelector: test.podNodeSelector}

		err := handler.Admit(admission.NewAttributesRecord(pod, api.Kind("Pod").WithVersion("version"), "testNamespace", namespace.ObjectMeta.Name, api.Resource("pods").WithVersion("version"), "", admission.Create, nil))
		if test.admit && err != nil {
			t.Errorf("Test: %s, expected no error but got: %s", test.testName, err)
		} else if !test.admit && err == nil {
			t.Errorf("Test: %s, expected an error", test.testName)
		}

		if !labels.Equals(test.mergedNodeSelector, pod.Spec.NodeSelector) {
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
		nodeEnvionment, err := NewPodNodeEnvironment(nil, nil)
		if err != nil {
			t.Errorf("%v: error getting node environment: %v", op, err)
			continue
		}

		if e, a := shouldHandle, nodeEnvionment.Handles(op); e != a {
			t.Errorf("%v: shouldHandle=%t, handles=%t", op, e, a)
		}
	}
}
