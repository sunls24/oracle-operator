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

// OracleBackupSpec defines the desired state of OracleBackup
type OracleBackupSpec struct {
	ClusterName      string `json:"clusterName"`
	BackupSecretName string `json:"backupSecretName,omitempty"`
}

// OracleBackupStatus defines the observed state of OracleBackup
type OracleBackupStatus struct {
	BackupStatus string `json:"backupStatus,omitempty"`
	BackupTag    string `json:"backupTag,omitempty"`
}

const (
	BackupStatusRunning   = "Running"
	BackupStatusCompleted = "Completed"
	BackupStatusFailed    = "Failed"
)

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.backupStatus"
//+kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
//+kubebuilder:resource:shortName=oraclebackup

// OracleBackup is the Schema for the oraclebackups API
type OracleBackup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OracleBackupSpec   `json:"spec,omitempty"`
	Status OracleBackupStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// OracleBackupList contains a list of OracleBackup
type OracleBackupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OracleBackup `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OracleBackup{}, &OracleBackupList{})
}
