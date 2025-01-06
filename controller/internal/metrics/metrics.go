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

package metrics

import (
	"context"

	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

// Metrics includes metrics used in km controller
type Metrics struct {
	cli                           client.Client
	runningKernelManagers         *prometheus.GaugeVec
	KernelManagerCreation         *prometheus.CounterVec
	KernelManagerFailCreation     *prometheus.CounterVec
	KernelManagerCullingCount     *prometheus.CounterVec
	KernelManagerCullingTimestamp *prometheus.GaugeVec
}

func NewMetrics(cli client.Client) *Metrics {
	m := &Metrics{
		cli: cli,
		runningKernelManagers: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "km_running",
				Help: "Current running kms in the cluster",
			},
			[]string{"namespace"},
		),
		KernelManagerCreation: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "km_create_total",
				Help: "Total times of creating kms",
			},
			[]string{"namespace"},
		),
		KernelManagerFailCreation: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "km_create_failed_total",
				Help: "Total failure times of creating kms",
			},
			[]string{"namespace"},
		),
		KernelManagerCullingCount: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "notebook_culling_total",
				Help: "Total times of culling notebooks",
			},
			[]string{"namespace", "name"},
		),
		KernelManagerCullingTimestamp: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "last_notebook_culling_timestamp_seconds",
				Help: "Timestamp of the last notebook culling in seconds",
			},
			[]string{"namespace", "name"},
		),
	}

	metrics.Registry.MustRegister(m)
	return m
}

// Describe implements the prometheus.Collector interface.
func (m *Metrics) Describe(ch chan<- *prometheus.Desc) {
	m.runningKernelManagers.Describe(ch)
	m.KernelManagerCreation.Describe(ch)
	m.KernelManagerFailCreation.Describe(ch)
}

// Collect implements the prometheus.Collector interface.
func (m *Metrics) Collect(ch chan<- prometheus.Metric) {
	m.scrape()
	m.runningKernelManagers.Collect(ch)
	m.KernelManagerCreation.Collect(ch)
	m.KernelManagerFailCreation.Collect(ch)
}

// scrape gets current running km statefulsets.
func (m *Metrics) scrape() {
	podList := &corev1.PodList{}
	err := m.cli.List(context.TODO(), podList)
	if err != nil {
		return
	}
	stsCache := make(map[string]float64)
	for _, v := range podList.Items {
		name, ok := v.ObjectMeta.GetLabels()["jupyrator.org/kernelmanager-name"]
		if ok && name == v.Name {
			stsCache[v.Namespace] += 1
		}
	}

	for ns, v := range stsCache {
		m.runningKernelManagers.WithLabelValues(ns).Set(v)
	}
}
