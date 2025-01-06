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

package v1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// KernelManagerSpec defines the desired state of KernelManager.
type KernelManagerSpec struct {
	// Template describes the kernelmanagers that will be created.
	Template KernelManagerTemplateSpec `json:"template,omitempty"`
	// IdleTimeoutSeconds is the number of seconds of inactivity before a kernelmanager is automatically deleted.
	// If provided, the controller will create a sidecar container to monitor the kernelmanager's activity.
	// When using this feature, the `ConnectionConfig` and `ConnectionFile` must be provided
	// +optional
	IdleTimeoutSeconds int32 `json:"idleTimeoutSeconds,omitempty"`
	// CullingIntervalSeconds is the number of seconds between checking for idle kernelmanagers. default is 60 seconds.
	CullingIntervalSeconds int32 `json:"cullingIntervalSeconds,omitempty"`
	// KernelConnectionConfig is the connection information for the kernelmanager's kernel.
	// This is used to connect to the kernelmanager's kernel.
	// If provided, the controller will create a configMap same sa CR and mount to `ConnectionFile`.
	// +optional
	ConnectionConfig KernelConnectionConfig `json:"kernelConnectionConfig,omitempty"`
	// MountPath is the path to the connection file. Controller will mount the connection file to the Path.
	// When using this feature, the `ConnectionConfig` must be provided.
	// +optional
	MountPath string `json:"mountPath,omitempty"`
}

type KernelManagerTemplateSpec struct {
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	Metadata metav1.ObjectMeta `json:"metadata,omitempty"`
	// Specification of the desired behavior of the kernelmanager.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	Spec corev1.PodSpec `json:"spec,omitempty"`
}

type KernelConnectionConfig struct {
	// The IP address of the kernelmanager's kernel.
	IP string `json:"ip,omitempty"`
	// The shell port of the kernelmanager's kernel.
	ShellPort int32 `json:"shellPort,omitempty"`
	// The iopub port of the kernelmanager's kernel.
	IOPubPort int32 `json:"iopubPort,omitempty"`
	// The stdin port of the kernelmanager's kernel.
	StdinPort int32 `json:"stdinPort,omitempty"`
	// The control port of the kernelmanager's kernel.
	ControlPort int32 `json:"controlPort,omitempty"`
	// The hb port of the kernelmanager's kernel.
	HBPort int32 `json:"hbPort,omitempty"`
	// The kernel id of the kernelmanager's kernel.
	KernelId string `json:"kernelId,omitempty"`
	// The kernel name of the kernelmanager's kernel.
	KernelName string `json:"kernelName,omitempty"`
	// The key of the kernelmanager's kernel.
	Key string `json:"key,omitempty"`
	// The transport of the kernelmanager's kernel.
	Transport string `json:"transport,omitempty"`
	// The signature scheme of the kernelmanager's kernel.
	SignatureScheme string `json:"signatureScheme,omitempty"`
}

// KernelManagerStatus defines the observed state of KernelManager.
type KernelManagerStatus struct {
	// Conditions is an array of current conditions
	Conditions []KernelManagerCondition `json:"conditions"`
	// ContainerState is the state of underlying container.
	ContainerState corev1.ContainerState `json:"containerState"`
	Phase          corev1.PodPhase       `json:"phase"`
	// IP is the IP address of the kernelmanager.
	IP string `json:"ip"`
}

type KernelManagerCondition struct {
	// Type is the type of the condition. Possible values are Running|Waiting|Terminated
	Type string `json:"type"`
	// Status is the status of the condition. Can be True, False, Unknown.
	Status string `json:"status"`
	// Last time we probed the condition.
	// +optional
	LastProbeTime metav1.Time `json:"lastProbeTime,omitempty"`
	// Last time the condition transitioned from one status to another.
	// +optional
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`
	// (brief) reason the container is in the current state
	// +optional
	Reason string `json:"reason,omitempty"`
	// Message regarding why the container is in the current state.
	// +optional
	Message string `json:"message,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=kernelmanagers,scope=Namespaced,shortName=km
// +kubebuilder:printcolumn:name="PHASE",type="string",JSONPath=".status.phase",description="The phase of the kernelmanager"
// +kubebuilder:printcolumn:name="IP",type="string",JSONPath=".status.ip",description="The IP address of the kernelmanager"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// KernelManager is the Schema for the kernelmanagers API.
type KernelManager struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KernelManagerSpec   `json:"spec,omitempty"`
	Status KernelManagerStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// KernelManagerList contains a list of KernelManager.
type KernelManagerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KernelManager `json:"items"`
}

func init() {
	SchemeBuilder.Register(&KernelManager{}, &KernelManagerList{})
}
