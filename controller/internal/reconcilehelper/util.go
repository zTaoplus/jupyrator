/*
Copyright 2024.

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

package reconcilehelper

import (
	"encoding/json"
	"reflect"
	"strings"

	corev1 "k8s.io/api/core/v1"
)

// CopyConfigMapFields copies the owned fields from one ConfigMap to another
func CopyConfigMapFields(from, to *corev1.ConfigMap) bool {
	requireUpdate := false
	for k, v := range to.Labels {
		if from.Labels[k] != v {
			requireUpdate = true
		}
	}
	to.Labels = from.Labels

	for k, v := range to.Annotations {
		if from.Annotations[k] != v {
			requireUpdate = true
		}
	}
	to.Annotations = from.Annotations

	// Don't copy the entire Spec, because we can't overwrite the clusterIp field

	if !reflect.DeepEqual(to.Data, from.Data) {
		requireUpdate = true
	}
	to.Data = from.Data

	if !reflect.DeepEqual(to.BinaryData, from.BinaryData) {
		requireUpdate = true
	}
	to.BinaryData = from.BinaryData

	return requireUpdate
}

// ConvertToSnakeCase converts a camelCase string to snake_case
func ConvertToSnakeCase(str string) string {
	var result []rune
	for i, r := range str {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result = append(result, '_', r+('a'-'A'))
		} else {
			result = append(result, r)
		}
	}
	return strings.ToLower(string(result))
}

// ToSnakeCaseJSON converts a struct to JSON with snake_case keys
func ToSnakeCaseJSON(v interface{}) (string, error) {
	// Marshal the struct to JSON
	data, err := json.Marshal(v)
	if err != nil {
		return "", err
	}

	// Unmarshal the JSON to a map
	var mapData map[string]interface{}
	if err := json.Unmarshal(data, &mapData); err != nil {
		return "", err
	}

	// Create a new map with snake_case keys
	snakeCaseMap := make(map[string]interface{})
	for k, v := range mapData {
		snakeCaseMap[ConvertToSnakeCase(k)] = v
	}

	// Marshal the new map to JSON
	snakeCaseJSON, err := json.Marshal(snakeCaseMap)
	if err != nil {
		return "", err
	}

	return string(snakeCaseJSON), nil
}
