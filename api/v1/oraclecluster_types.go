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
	"fmt"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// OracleClusterSpec defines the desired state of OracleCluster
type OracleClusterSpec struct {
	Image    string `json:"image,omitempty"`
	CLIImage string `json:"cliImage,omitempty"`

	Replicas *int32 `json:"replicas,omitempty"`
	Password string `json:"password,omitempty"`

	PodSpec    PodSpec    `json:"podSpec,omitempty"`
	VolumeSpec VolumeSpec `json:"volumeSpec,omitempty"`
}

// OracleClusterStatus defines the observed state of OracleCluster
type OracleClusterStatus struct {
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// OracleCluster is the Schema for the oracleclusters API
type OracleCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OracleClusterSpec   `json:"spec,omitempty"`
	Status OracleClusterStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// OracleClusterList contains a list of OracleCluster
type OracleClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OracleCluster `json:"items"`
}

type PodSpec struct {
	ImagePullPolicy corev1.PullPolicy           `json:"imagePullPolicy,omitempty"`
	Resources       corev1.ResourceRequirements `json:"resources,omitempty"`

	OracleEnv    []corev1.EnvVar `json:"oracleEnv,omitempty"`
	OracleCLIEnv []corev1.EnvVar `json:"oracleCliEnv,omitempty"`
}

type VolumeSpec struct {
	//EmptyDir *corev1.EmptyDirVolumeSource `json:"emptyDir,omitempty"`
	//HostPath *corev1.HostPathVolumeSource `json:"hostPath,omitempty"`
	PersistentVolumeClaim *corev1.PersistentVolumeClaimSpec `json:"persistentVolumeClaim,omitempty"`
}

func (in *OracleCluster) UniqueName() string {
	return fmt.Sprintf("oc-%s", in.Name)
}

func (in *OracleCluster) ClusterLabel() map[string]string {
	return map[string]string{"cluster": in.Name}
}

func (in *OracleCluster) SetObject(obj *metav1.ObjectMeta) {
	obj.Namespace = in.Namespace
	obj.Name = in.UniqueName()
	obj.Labels = in.ClusterLabel()
}

func (in *OracleCluster) AddFinalizer(finalizer string) {
	for _, f := range in.Finalizers {
		if f == finalizer {
			return
		}
	}
	in.Finalizers = append(in.Finalizers, finalizer)
}

func (in *OracleCluster) DeleteFinalizer(finalizer string) {
	var newList []string
	for _, f := range in.Finalizers {
		if f == finalizer {
			continue
		}
		newList = append(newList, f)
	}
	in.Finalizers = newList
}

func init() {
	SchemeBuilder.Register(&OracleCluster{}, &OracleClusterList{})
}
