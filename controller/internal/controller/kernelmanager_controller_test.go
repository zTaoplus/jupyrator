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

package controller

import (
	"reflect"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"

	v1 "github.com/kernel-manager-controller/api/v1"
)

func TestKMNameFromInvolvedObject(t *testing.T) {
	testPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Namespace: "default",
			Labels: map[string]string{
				KernelManagerNameLabel: "foo",
			},
		},
	}

	podEvent := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pod-event",
		},
		InvolvedObject: corev1.ObjectReference{
			Kind:      "Pod",
			Name:      "foo",
			Namespace: "default",
		},
	}

	tests := []struct {
		name           string
		event          *corev1.Event
		expectedKMName string
	}{
		{
			name:           "pod event",
			event:          podEvent,
			expectedKMName: "foo",
		},
	}

	objects := []runtime.Object{testPod}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c := fake.NewFakeClient(objects...)
			kmName, err := kmNameFromInvolvedObject(c, &test.event.InvolvedObject)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if kmName != test.expectedKMName {
				t.Fatalf("Got %v, Expected %v", kmName, test.expectedKMName)
			}
		})
	}
}

func TestCreateKernelManagerStatus(t *testing.T) {
	tests := []struct {
		name             string
		currentKm        v1.KernelManager
		pod              corev1.Pod
		expectedKMStatus v1.KernelManagerStatus
	}{
		{
			name: "KernelManagerStatusInitialization",
			currentKm: v1.KernelManager{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "default",
				},
				Status: v1.KernelManagerStatus{},
			},
			pod: corev1.Pod{},
			expectedKMStatus: v1.KernelManagerStatus{
				Conditions:     []v1.KernelManagerCondition{},
				ContainerState: corev1.ContainerState{},
			},
		},
		{
			name: "KernelManagerContainerState",
			currentKm: v1.KernelManager{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "default",
				},
				Status: v1.KernelManagerStatus{},
			},
			pod: corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "default",
				},
				Status: corev1.PodStatus{
					ContainerStatuses: []corev1.ContainerStatus{
						{
							Name: "foo",
							State: corev1.ContainerState{
								Running: &corev1.ContainerStateRunning{
									StartedAt: metav1.Time{},
								},
							},
						},
					},
				},
			},
			expectedKMStatus: v1.KernelManagerStatus{
				Conditions: []v1.KernelManagerCondition{},
				ContainerState: corev1.ContainerState{
					Running: &corev1.ContainerStateRunning{
						StartedAt: metav1.Time{},
					},
				},
			},
		},
		{
			name: "mirroringPodConditions",
			pod: corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "default",
				},
				Status: corev1.PodStatus{
					Conditions: []corev1.PodCondition{
						{
							Type:               "Running",
							LastProbeTime:      metav1.Date(2024, time.Month(12), 30, 1, 10, 30, 0, time.UTC),
							LastTransitionTime: metav1.Date(2024, time.Month(12), 30, 1, 10, 30, 0, time.UTC),
						},
						{
							Type:               "Waiting",
							LastProbeTime:      metav1.Date(2024, time.Month(12), 30, 1, 10, 30, 0, time.UTC),
							LastTransitionTime: metav1.Date(2024, time.Month(12), 30, 1, 10, 30, 0, time.UTC),
							Reason:             "PodInitializing",
						},
					},
				},
			},
			expectedKMStatus: v1.KernelManagerStatus{
				Conditions: []v1.KernelManagerCondition{
					{
						Type:               "Running",
						LastProbeTime:      metav1.Date(2024, time.Month(12), 30, 1, 10, 30, 0, time.UTC),
						LastTransitionTime: metav1.Date(2024, time.Month(12), 30, 1, 10, 30, 0, time.UTC),
					},
					{
						Type:               "Waiting",
						LastProbeTime:      metav1.Date(2024, time.Month(12), 30, 1, 10, 30, 0, time.UTC),
						LastTransitionTime: metav1.Date(2024, time.Month(12), 30, 1, 10, 30, 0, time.UTC),
						Reason:             "PodInitializing",
					},
				},
				ContainerState: corev1.ContainerState{},
			},
		},
		{
			name: "unschedulablePod",
			pod: corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "default",
				},
				Status: corev1.PodStatus{
					Conditions: []corev1.PodCondition{
						{
							Type:               "PodScheduled",
							LastProbeTime:      metav1.Date(2024, time.Month(4), 21, 1, 10, 30, 0, time.UTC),
							LastTransitionTime: metav1.Date(2024, time.Month(4), 21, 1, 10, 30, 0, time.UTC),
							Message:            "0/1 nodes are available: 1 Insufficient cpu.",
							Status:             "false",
							Reason:             "Unschedulable",
						},
					},
				},
			},
			expectedKMStatus: v1.KernelManagerStatus{
				Conditions: []v1.KernelManagerCondition{
					{
						Type:               "PodScheduled",
						LastProbeTime:      metav1.Date(2024, time.Month(4), 21, 1, 10, 30, 0, time.UTC),
						LastTransitionTime: metav1.Date(2024, time.Month(4), 21, 1, 10, 30, 0, time.UTC),
						Message:            "0/1 nodes are available: 1 Insufficient cpu.",
						Status:             "false",
						Reason:             "Unschedulable",
					},
				},
				ContainerState: corev1.ContainerState{},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r := createMockReconciler()
			req := ctrl.Request{}
			status := createKernelManagerStatus(r, &test.currentKm, &test.pod, req)
			if !reflect.DeepEqual(status, test.expectedKMStatus) {
				t.Errorf("\nExpect: %v; \nOutput: %v", test.expectedKMStatus, status)
			}
		})
	}
}

func createMockReconciler() *KernelManagerReconciler {
	return &KernelManagerReconciler{
		Scheme: runtime.NewScheme(),
		Log:    ctrl.Log,
	}
}

func TestGenerateConfigMap(t *testing.T) {
	tests := []struct {
		name           string
		km             v1.KernelManager
		expectedConfig corev1.ConfigMap
	}{
		{
			name: "GenerateConfigMap",
			km: v1.KernelManager{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "kernel-00000000-0000-0000-0000-000000000000",
					Labels:    map[string]string{"kernelmanager": "test"},
					Namespace: "default",
				},
				Spec: v1.KernelManagerSpec{
					Template: v1.KernelManagerTemplateSpec{},
					ConnectionConfig: v1.KernelConnectionConfig{
						IP:              "0.0.0.0",
						ShellPort:       57939,
						IOPubPort:       41453,
						StdinPort:       35125,
						ControlPort:     44733,
						HBPort:          57001,
						KernelId:        "00000000-0000-0000-0000-000000000000",
						KernelName:      "",
						Key:             "00000000-0000-0000-0000-000000000000",
						Transport:       "tcp",
						SignatureScheme: "hmac-sha256",
					},
				},
			},
			expectedConfig: corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "kernel-00000000-0000-0000-0000-000000000000",
					Labels:    map[string]string{"kernelmanager": "test"},
					Namespace: "default",
				},
				Data: map[string]string{
					"kernel-00000000-0000-0000-0000-000000000000.json": "{\"control_port\":44733,\"hb_port\":57001,\"iopub_port\":41453,\"ip\":\"0.0.0.0\",\"kernel_id\":\"00000000-0000-0000-0000-000000000000\",\"key\":\"00000000-0000-0000-0000-000000000000\",\"shell_port\":57939,\"signature_scheme\":\"hmac-sha256\",\"stdin_port\":35125,\"transport\":\"tcp\"}",
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			configMap, err := generateConfigMap(&test.km)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if !reflect.DeepEqual(configMap.Data, test.expectedConfig.Data) {
				t.Errorf("Expected: %v; Got: %v", test.expectedConfig.Data, configMap.Data)
			}
		})
	}
}
