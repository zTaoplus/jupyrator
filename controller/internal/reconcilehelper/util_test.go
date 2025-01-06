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
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCopyConfigMapFields(t *testing.T) {
	tests := []struct {
		name           string
		from           *corev1.ConfigMap
		to             *corev1.ConfigMap
		expectedUpdate bool
		expectedTo     *corev1.ConfigMap
	}{
		{
			name: "Data changed",
			from: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      map[string]string{"key1": "value1"},
					Annotations: map[string]string{"key2": "value2"},
				},
				Data:       map[string]string{"key3": "new_value3"},
				BinaryData: map[string][]byte{"key4": []byte("value4")},
			},
			to: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      map[string]string{"key1": "value1"},
					Annotations: map[string]string{"key2": "value2"},
				},
				Data:       map[string]string{"key3": "value3"},
				BinaryData: map[string][]byte{"key4": []byte("value4")},
			},
			expectedUpdate: true,
			expectedTo: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      map[string]string{"key1": "value1"},
					Annotations: map[string]string{"key2": "value2"},
				},
				Data:       map[string]string{"key3": "new_value3"},
				BinaryData: map[string][]byte{"key4": []byte("value4")},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			updated := CopyConfigMapFields(tt.from, tt.to)
			if updated != tt.expectedUpdate {
				t.Errorf("expected update to be %v, got %v", tt.expectedUpdate, updated)
			}
			if !reflect.DeepEqual(tt.to, tt.expectedTo) {
				t.Errorf("expected ConfigMap to be %v, got %v", tt.expectedTo, tt.to)
			}
		})
	}
}

// TestConvertToSnakeCase tests the ConvertToSnakeCase function
func TestConvertToSnakeCase(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"camelCase", "camel_case"},
		{"CamelCase", "camel_case"},
		{"camelCaseTest", "camel_case_test"},
		{"CamelCaseTest", "camel_case_test"},
		{"simple", "simple"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := ConvertToSnakeCase(tt.input)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

// TestToSnakeCaseJSON tests the ToSnakeCaseJSON function
func TestToSnakeCaseJSON(t *testing.T) {
	type TestStruct struct {
		CamelCaseField string `json:"camelCaseField"`
		SimpleField    string `json:"simpleField"`
	}

	tests := []struct {
		input    TestStruct
		expected string
	}{
		{
			input:    TestStruct{CamelCaseField: "value1", SimpleField: "value2"},
			expected: `{"camel_case_field":"value1","simple_field":"value2"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result, err := ToSnakeCaseJSON(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			var resultMap map[string]interface{}
			var expectedMap map[string]interface{}
			if err := json.Unmarshal([]byte(result), &resultMap); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if err := json.Unmarshal([]byte(tt.expected), &expectedMap); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(resultMap, expectedMap) {
				t.Errorf("expected %v, got %v", expectedMap, resultMap)
			}
		})
	}
}
