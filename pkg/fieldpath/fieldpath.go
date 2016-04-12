/*
Copyright 2015 The Kubernetes Authors All rights reserved.

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

package fieldpath

import (
	"fmt"
	"reflect"
	"regexp"

	"k8s.io/kubernetes/pkg/api/meta"
	"k8s.io/kubernetes/pkg/util/jsonpath"
)

// formatMap formats map[string]string to a string.
func formatMap(m map[string]string) (fmtStr string) {
	for key, value := range m {
		fmtStr += fmt.Sprintf("%v=%q\n", key, value)
	}

	return
}

// ExtractFieldPathAsString extracts the field from the given object
// and returns it as a string.  The object must be a pointer to an
// API type.
//
// Currently, this API is limited to supporting the fieldpaths:
//
// 1.  metadata.name - The name of an API object
// 2.  metadata.namespace - The namespace of an API object
func ExtractFieldPathAsString(obj interface{}, fieldPath string) (string, error) {
	accessor, err := meta.Accessor(obj)
	if err != nil {
		return "", nil
	}

	switch fieldPath {
	case "metadata.annotations":
		return formatMap(accessor.GetAnnotations()), nil
	case "metadata.labels":
		return formatMap(accessor.GetLabels()), nil
	case "metadata.name":
		return accessor.GetName(), nil
	case "metadata.namespace":
		return accessor.GetNamespace(), nil
	}

	return "", fmt.Errorf("Unsupported fieldPath: %v", fieldPath)
}

var jsonRegexp = regexp.MustCompile("^\\{\\.?([^{}]+)\\}$|^\\.?([^{}]+)$")

func ExtractJSONFieldSelectorValue(obj interface{}, fieldPath string) (string, error) {
	parser := jsonpath.New("downward APIs")
	tmpFieldPath, err := jsonpath.MassageJSONPath(fieldPath, jsonRegexp)
	if err != nil {
		return "", err
	}

	if err := parser.Parse(tmpFieldPath); err != nil {
		return "", err
	}

	values, err := parser.FindResults(reflect.ValueOf(obj).Elem().Interface())
	if err != nil {
		return "", err
	}
	if len(values) == 0 {
		return "", fmt.Errorf("couldn't find any field with path: %s", tmpFieldPath)
	}

	return fmt.Sprintf("%s", values[0][0]), nil
}
