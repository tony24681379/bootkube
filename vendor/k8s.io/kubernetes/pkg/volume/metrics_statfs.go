/*
Copyright 2016 The Kubernetes Authors All rights reserved.

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

package volume

import (
	"errors"
	"fmt"

	"k8s.io/kubernetes/pkg/api/resource"
	"k8s.io/kubernetes/pkg/volume/util"
)

var _ MetricsProvider = &metricsStatFS{}

// metricsStatFS represents a MetricsProvider that calculates the used and available
// Volume space by stat'ing and gathering filesystem info for the Volume path.
type metricsStatFS struct {
	// the directory path the volume is mounted to.
	path string
}

// NewMetricsStatfs creates a new metricsStatFS with the Volume path.
func NewMetricsStatFS(path string) MetricsProvider {
	return &metricsStatFS{path}
}

// See MetricsProvider.GetMetrics
// GetMetrics calculates the volume usage and device free space by executing "du"
// and gathering filesystem info for the Volume path.
func (md *metricsStatFS) GetMetrics() (*Metrics, error) {
	metrics := &Metrics{}
	if md.path == "" {
		return metrics, errors.New("no path defined for disk usage metrics.")
	}

	err := md.getFsInfo(metrics)
	if err != nil {
		return metrics, err
	}

	return metrics, nil
}

// getFsInfo writes metrics.Capacity, metrics.Used and metrics.Available from the filesystem info
func (md *metricsStatFS) getFsInfo(metrics *Metrics) error {
	available, capacity, usage, err := util.FsInfo(md.path)
	if err != nil {
		return fmt.Errorf("Failed to get FsInfo due to error %v", err)
	}
	metrics.Available = resource.NewQuantity(available, resource.BinarySI)
	metrics.Capacity = resource.NewQuantity(capacity, resource.BinarySI)
	metrics.Used = resource.NewQuantity(usage, resource.BinarySI)
	return nil
}
