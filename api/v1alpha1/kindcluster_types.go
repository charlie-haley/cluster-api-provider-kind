/*
Copyright 2023.

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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

const (
	// ClusterFinalizer allows ReconcileKindCluster to clean up Kind resources associated with KindCluster before
	// removing it from the apiserver.
	ClusterFinalizer = "kindcluster.infrastructure.cluster.x-k8s.io"
)

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=kindclusters,scope=Namespaced,categories=cluster-api
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Cluster",type="string",JSONPath=".metadata.labels.cluster\\.x-k8s\\.io/cluster-name",description="Cluster to which this MetalCluster belongs"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.ready",description="Kind cluster is ready"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="The age of this resource"
// +kubebuilder:printcolumn:name="Endpoint",type="string",JSONPath=".spec.controlPlaneEndpoint",description="API Endpoint",priority=1

// KindCluster is the Schema for the kindclusters API
type KindCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KindClusterSpec   `json:"spec,omitempty"`
	Status KindClusterStatus `json:"status,omitempty"`
}

// KindClusterSpec defines the desired state of KindCluster
type KindClusterSpec struct {
	// ControlPlaneEndpoint represents the endpoint used to communicate with the control plane.
	// +optional
	ControlPlaneEndpoint clusterv1.APIEndpoint `json:"controlPlaneEndpoint"`
}

// KindClusterStatus defines the observed state of KindCluster
type KindClusterStatus struct {
	Ready bool `json:"ready"`
}

//+kubebuilder:object:root=true

// KindClusterList contains a list of KindCluster
type KindClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KindCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&KindCluster{}, &KindClusterList{})
}
