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
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-logr/logr"
	v1 "github.com/kernel-manager-controller/api/v1"
	"github.com/kernel-manager-controller/internal/metrics"
	"github.com/kernel-manager-controller/internal/reconcilehelper"
)

const KernelManagerNameLabel = "jupyrator.org/kernelmanager-name"
const KernelManagerIdleLabel = "jupyrator.org/kernelmanager-idle"

const DefaultServiceAccount = "default-editor"

func ignoreNotFound(err error) error {
	if apierrs.IsNotFound(err) {
		return nil
	}
	return err
}

// KernelManagerReconciler reconciles a KernelManager object
type KernelManagerReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	Log           logr.Logger
	Metrics       *metrics.Metrics
	EventRecorder record.EventRecorder
	MonitorImage  string
}

// +kubebuilder:rbac:groups=core,resources=events,verbs=get;list;watch;create;patch
// +kubebuilder:rbac:groups=core,resources=pods,verbs='*'
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs='*'
// +kubebuilder:rbac:groups=jupyrator.org,resources=kernelmanagers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=jupyrator.org,resources=kernelmanagers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=jupyrator.org,resources=kernelmanagers/finalizers,verbs=update
func (r *KernelManagerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("Kernel Manager", req.NamespacedName)
	log.Info("Reconciliation loop started")
	event := &corev1.Event{}
	getEventErr := r.Get(ctx, req.NamespacedName, event)
	if getEventErr == nil {
		log.Info("Found event for KernelManager. Re-emitting...")

		// Find the KernelManager that corresponds to the triggered event
		involvedKernelManager := &v1.KernelManager{}
		kmName, err := kmNameFromInvolvedObject(r.Client, &event.InvolvedObject)
		if err != nil {
			return ctrl.Result{}, err
		}

		involvedKernelManagerKey := types.NamespacedName{Name: kmName, Namespace: req.Namespace}
		if err := r.Get(ctx, involvedKernelManagerKey, involvedKernelManager); err != nil {
			log.Error(err, "unable to fetch KernelManager by looking at event")
			return ctrl.Result{}, ignoreNotFound(err)
		}

		// re-emit the event in the KernelManager CR
		log.Info("Emitting KernelManager Event.", "Event", event)
		r.EventRecorder.Eventf(involvedKernelManager, event.Type, event.Reason,
			"Reissued from %s/%s: %s", strings.ToLower(event.InvolvedObject.Kind), event.InvolvedObject.Name, event.Message)
		return ctrl.Result{}, nil
	}

	if !apierrs.IsNotFound(getEventErr) {
		return ctrl.Result{}, getEventErr
	}
	// If not found, continue. Is not an event.
	instance := &v1.KernelManager{}
	if err := r.Get(ctx, req.NamespacedName, instance); err != nil {
		// If the KernelManager resource is not found, return without requeuing
		// NotFound, it may be because the user deleted it or it was recycled after not being used for a long time.
		// So there is no need to log this kind of error here.
		if ignoreNotFound(err) != nil {
			log.Error(err, "unable to fetch KernelManager")
		}
		return ctrl.Result{}, ignoreNotFound(err)
	}

	// Cull idle KernelManagers if kernel manager label is present
	if instance.Labels[KernelManagerIdleLabel] == "true" {
		t := time.Now()
		log.Info("Culling idle KernelManager", "namespace", instance.Namespace, "name", instance.Name)
		if err := r.Delete(ctx, instance); err != nil {
			log.Error(err, "unable to delete KernelManager")
			return ctrl.Result{}, err
		}
		r.Metrics.KernelManagerCullingCount.WithLabelValues(instance.Namespace, instance.Name).Inc()
		r.Metrics.KernelManagerCullingTimestamp.WithLabelValues(instance.Namespace, instance.Name).Set(float64(t.Unix()))
		return ctrl.Result{}, nil
	}

	// Generate ConfigMap resource for KernelManager if ConnectionConfig is present
	if instance.Spec.ConnectionConfig != (v1.KernelConnectionConfig{}) {
		configMap, err := generateConfigMap(instance)
		if err != nil {
			log.Error(err, "unable to generate configMap")
			return ctrl.Result{}, err
		}
		justCreated := false
		if err := ctrl.SetControllerReference(instance, configMap, r.Scheme); err != nil {
			return ctrl.Result{}, err
		}
		foundConfigMap := &corev1.ConfigMap{}
		err = r.Get(ctx, types.NamespacedName{Name: configMap.Name, Namespace: configMap.Namespace}, foundConfigMap)
		if err != nil && apierrs.IsNotFound(err) {
			log.Info("Creating configMap", "namespace", configMap.Namespace, "name", configMap.Name)
			if err = r.Create(ctx, configMap); err != nil {
				log.Error(err, "unable to create configMap")
				return ctrl.Result{}, err
			}
			justCreated = true
		} else if err != nil {
			log.Error(err, "error getting configMap")
			return ctrl.Result{}, err
		}

		// Update the foundConfigMap object and write the result back if there are any changes
		if !justCreated && reconcilehelper.CopyConfigMapFields(configMap, foundConfigMap) {
			log.Info("Updating ConfigMap", "namespace", instance.Namespace, "name", instance.Name)
			err = r.Update(ctx, foundConfigMap)
			if err != nil {
				log.Error(err, "unable to update Statefulset")
				return ctrl.Result{}, err
			}
		}
	}

	// Generate Pod resource for KernelManager
	pod := generatePod(instance, r.MonitorImage)
	if err := ctrl.SetControllerReference(instance, pod, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}

	foundPod := &corev1.Pod{}
	err := r.Get(ctx, types.NamespacedName{Name: pod.Name, Namespace: pod.Namespace}, foundPod)
	if err != nil && apierrs.IsNotFound(err) {
		log.Info("Creating pod", "namespace", pod.Namespace, "name", pod.Name)
		r.Metrics.KernelManagerCreation.WithLabelValues(pod.Namespace).Inc()
		if err = r.Create(ctx, pod); err != nil {
			log.Error(err, "unable to create pod")
			r.Metrics.KernelManagerFailCreation.WithLabelValues(pod.Namespace).Inc()

			r.EventRecorder.Eventf(instance, corev1.EventTypeWarning,
				"FailedCreate", "Failed to perform operation on %s: %v", instance.Name, err)

			return ctrl.Result{}, err
		}
	} else if err != nil {
		log.Error(err, "error getting KernelManager")
		return ctrl.Result{}, err
	}

	// Update KernelManager CR status
	err = updateKernelManagerStatus(r, instance, foundPod, req)
	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// kmNameFromInvolvedObject returns the name of the KernelManager that corresponds to the given object reference.
func kmNameFromInvolvedObject(c client.Client, object *corev1.ObjectReference) (string, error) {
	name, namespace := object.Name, object.Namespace

	if object.Kind == "Pod" {
		pod := &corev1.Pod{}
		err := c.Get(
			context.TODO(),
			types.NamespacedName{
				Namespace: namespace,
				Name:      name,
			},
			pod,
		)
		if err != nil {
			return "", err
		}
		if kmName, ok := pod.Labels[KernelManagerNameLabel]; ok {
			return kmName, nil
		}
	}
	return "", fmt.Errorf("object isn't related to a KernelManager")
}

// generateConfigMap generates a ConfigMap resource for the given KernelManager instance.
func generateConfigMap(instance *v1.KernelManager) (*corev1.ConfigMap, error) {
	// Marshal the ConnectionConfig to a JSON string
	data, err := reconcilehelper.ToSnakeCaseJSON(instance.Spec.ConnectionConfig)
	if err != nil {
		return nil, err
	}
	// Create the connection file name from the CR name
	connFileName := fmt.Sprintf("%s.json", instance.Name)

	// Create the ConfigMap object
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:        instance.Name,
			Namespace:   instance.Namespace,
			Labels:      instance.Labels,
			Annotations: instance.Annotations,
		},
		Data: map[string]string{
			connFileName: data,
		},
	}

	// Copy all the kernel manager labels to the configmap
	for k, v := range instance.Labels {
		configMap.Labels[k] = v
	}

	return configMap, nil
}

// generatePod generates a pod resource for the given KernelManager instance.
func generatePod(instance *v1.KernelManager, monitorImage string) *corev1.Pod {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        instance.Name,
			Namespace:   instance.Namespace,
			Labels:      instance.Labels,
			Annotations: instance.Annotations,
		},
		Spec: instance.Spec.Template.Spec,
	}

	// Set the kernel manager main container name to the CR name
	pod.Spec.Containers[0].Name = instance.Name

	// Add connection file volume to the pod if `ConnectionConfig` is provided
	if instance.Spec.ConnectionConfig != (v1.KernelConnectionConfig{}) {
		// Set the mount path if not provided, default to `/tmp`
		mountPath := instance.Spec.MountPath
		if mountPath == "" {
			mountPath = "/tmp"
		}

		// Set container env to use the connection file
		sidecarConnectionFilePath := fmt.Sprintf("%s/%s.json", mountPath, instance.Name)
		pod.Spec.Containers[0].Env = append(pod.Spec.Containers[0].Env, corev1.EnvVar{
			Name:  "KERNEL_CONNECTION_FILE_PATH",
			Value: sidecarConnectionFilePath,
		})

		// Set pod volume and volume mount
		pod.Spec.Volumes = append(pod.Spec.Volumes, corev1.Volume{
			Name: "connection-file-volume",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: instance.Name},
				}},
		})
		pod.Spec.Containers[0].VolumeMounts = append(pod.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
			Name:      "connection-file-volume",
			MountPath: mountPath,
		})
	}

	// Copy all the kernel manager labels to the pod including pod default related labels
	l := &pod.ObjectMeta.Labels
	for k, v := range instance.Labels {
		(*l)[k] = v
	}

	// If `IdleTimeoutSeconds` and `ConnectionConfig` is provided
	// Add a sidecar container to monitor the kernel manager's activity
	// NODE: The sidecar container uses the connection file to connect to the kernel and monitor the activity.
	if monitorImage != "" && instance.Spec.IdleTimeoutSeconds != 0 && instance.Spec.ConnectionConfig != (v1.KernelConnectionConfig{}) {
		// Set the culling interval to 60 seconds if not provided
		cullingIntervalSeconds := instance.Spec.CullingIntervalSeconds
		if cullingIntervalSeconds == 0 {
			cullingIntervalSeconds = 60
		}
		// Add the sidecar container envs
		sidecarContainerEnvs := []corev1.EnvVar{
			{
				Name:  "NAME",
				Value: instance.Name,
			},
			{
				Name:  "NAMESPACE",
				Value: instance.Namespace,
			},
			{
				Name:  "IDLE_TIMEOUT",
				Value: fmt.Sprintf("%d", instance.Spec.IdleTimeoutSeconds),
			},
			{
				Name:  "CULLING_INTERVAL",
				Value: fmt.Sprintf("%d", cullingIntervalSeconds),
			},
		}

		sidecarContainer := corev1.Container{
			Name:  "monitor",
			Image: monitorImage,
			Env:   sidecarContainerEnvs,
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      "connection-file-volume",
					MountPath: "/tmp",
				},
			},
		}
		pod.Spec.Containers = append(pod.Spec.Containers, sidecarContainer)

		// Add the default service account to the pod, if not provided
		// NODE: The sidecar container uses the default service account to access the Kubernetes API
		// to label the kernel manager when it's idle.
		// And set the last activity time to the current time.
		if pod.Spec.ServiceAccountName == "" {
			pod.Spec.ServiceAccountName = DefaultServiceAccount
		}
	}
	return pod
}

// updateKernelManagerStatus updates the status of the given KernelManager CR.
func updateKernelManagerStatus(r *KernelManagerReconciler,
	instance *v1.KernelManager, pod *corev1.Pod, req ctrl.Request) error {
	log := r.Log.WithValues("Kernel Manager", req.NamespacedName)

	status := createKernelManagerStatus(r, instance, pod, req)

	log.Info("Updating KernelManager CR Status", "status", status)

	instance.Status = status
	return r.Status().Update(context.Background(), instance)
}

func createKernelManagerStatus(r *KernelManagerReconciler,
	instance *v1.KernelManager, pod *corev1.Pod, req ctrl.Request) v1.KernelManagerStatus {

	log := r.Log.WithValues("kernel manager", req.NamespacedName)

	// Initialize KernelManager CR Status
	log.Info("Initializing KernelManager CR Status")
	status := v1.KernelManagerStatus{
		Conditions:     make([]v1.KernelManagerCondition, 0),
		ContainerState: corev1.ContainerState{},
		Phase:          pod.Status.Phase,
		IP:             pod.Status.PodIP,
	}

	// Update the status based on the Pod's status
	if reflect.DeepEqual(pod.Status, corev1.PodStatus{}) {
		log.Info("No pod.Status found. Won't update kernel manager conditions and containerState")
		return status
	}

	// Update status of the CR using the ContainerState of
	// the container that has the same name as the CR.
	// If no container of same name is found, the state of the CR is not updated.
	kmContainerFound := false
	log.Info("Calculating KernelManager's  containerState")

	// TODO: Handle multiple containers in the pod
	for i := range pod.Status.ContainerStatuses {
		if pod.Status.ContainerStatuses[i].Name != instance.Name {
			continue
		}

		if pod.Status.ContainerStatuses[i].State == instance.Status.ContainerState {
			continue
		}

		// Update KernelManager CR's status.ContainerState
		cs := pod.Status.ContainerStatuses[i].State
		log.Info("Updating KernelManager CR state: ", "state", cs)

		status.ContainerState = cs
		kmContainerFound = true
		break
	}

	if !kmContainerFound {
		log.Error(nil, "Could not find container with the same name as KernelManager "+
			"in containerStates of Pod. Will not update kernelmanager's "+
			"status.containerState ")
	}

	// Mirroring pod condition
	kernelManagerConditions := []v1.KernelManagerCondition{}
	log.Info("Calculating KernelManager's Conditions")
	for i := range pod.Status.Conditions {
		condition := PodCondToKernelManagerCond(pod.Status.Conditions[i])
		kernelManagerConditions = append(kernelManagerConditions, condition)
	}

	status.Conditions = kernelManagerConditions

	return status
}

func PodCondToKernelManagerCond(podc corev1.PodCondition) v1.KernelManagerCondition {

	condition := v1.KernelManagerCondition{}

	if len(podc.Type) > 0 {
		condition.Type = string(podc.Type)
	}

	if len(podc.Status) > 0 {
		condition.Status = string(podc.Status)
	}

	if len(podc.Message) > 0 {
		condition.Message = podc.Message
	}

	if len(podc.Reason) > 0 {
		condition.Reason = podc.Reason
	}

	// check if podc.LastProbeTime is null. If so initialize
	// the field with metav1.Now()
	check := podc.LastProbeTime.Time.Equal(time.Time{})
	if !check {
		condition.LastProbeTime = podc.LastProbeTime
	} else {
		condition.LastProbeTime = metav1.Now()
	}

	// check if podc.LastTransitionTime is null. If so initialize
	// the field with metav1.Now()
	check = podc.LastTransitionTime.Time.Equal(time.Time{})
	if !check {
		condition.LastTransitionTime = podc.LastTransitionTime
	} else {
		condition.LastTransitionTime = metav1.Now()
	}

	return condition
}

// SetupWithManager sets up the controller with the Manager.
func (r *KernelManagerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.KernelManager{}).
		Named("kernelmanager").
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.Pod{}).
		Complete(r)
}
