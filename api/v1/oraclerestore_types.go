/*
Copyright 2021.

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// OracleRestoreSpec defines the desired state of OracleRestore
type OracleRestoreSpec struct {
	ClusterName string `json:"clusterName,omitempty"`
	BackupTag   string `json:"backupTag,omitempty"`
}

// OracleRestoreStatus defines the observed state of OracleRestore
type OracleRestoreStatus struct {
	RestoreStatus string `json:"restoreStatus,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.restoreStatus"
//+kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
//+kubebuilder:resource:shortName=oraclerestore

// OracleRestore is the Schema for the oraclerestores API
type OracleRestore struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OracleRestoreSpec   `json:"spec,omitempty"`
	Status OracleRestoreStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// OracleRestoreList contains a list of OracleRestore
type OracleRestoreList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OracleRestore `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OracleRestore{}, &OracleRestoreList{})
}