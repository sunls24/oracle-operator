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
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"oracle-operator/utils/constants"
	"strconv"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// OracleClusterSpec defines the desired state of OracleCluster
type OracleClusterSpec struct {
	Image    string `json:"image,omitempty"`
	Password string `json:"password,omitempty"`
	NodePort int32  `json:"nodePort,omitempty"`

	PodSpec    PodSpec    `json:"podSpec,omitempty"`
	VolumeSpec VolumeSpec `json:"volumeSpec,omitempty"`
}

// OracleClusterStatus defines the observed state of OracleCluster
type OracleClusterStatus struct {
	Ready corev1.ConditionStatus `json:"ready,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.ready",description="The cluster status"
//+kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
//+kubebuilder:resource:shortName=oracle

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
	NodeSelector    map[string]string           `json:"nodeSelector,omitempty"`

	OracleSID    string          `json:"oracleSID,omitempty"`
	OracleEnv    []corev1.EnvVar `json:"oracleEnv,omitempty"`
	OracleCLIEnv []corev1.EnvVar `json:"oracleCliEnv,omitempty"`
}

type VolumeSpec struct {
	//EmptyDir *corev1.EmptyDirVolumeSource `json:"emptyDir,omitempty"`
	//HostPath *corev1.HostPathVolumeSource `json:"hostPath,omitempty"`
	PersistentVolumeClaim *corev1.PersistentVolumeClaimSpec `json:"persistentVolumeClaim,omitempty"`
}

func (in *OracleCluster) UniteName() string {
	return fmt.Sprintf("%s-oracle", in.Name)
}

func (in *OracleCluster) ClusterLabel() map[string]string {
	return map[string]string{"oracle": in.Name}
}

func (in *OracleCluster) SetObject(obj metav1.Object) {
	obj.SetNamespace(in.Namespace)
	obj.SetName(in.UniteName())
	obj.SetLabels(in.ClusterLabel())
}

func (in *OracleCluster) MemoryValue() int64 {
	var res = in.Spec.PodSpec.Resources
	var mem = res.Limits.Memory().Value()
	if mem == 0 {
		mem = res.Requests.Memory().Value()
	}
	if mem != 0 {
		mem /= 1048576 // 1024 * 1024
	}
	return mem
}

func (in *OracleCluster) InitSGASize(memory int64) string {
	if memory == 0 {
		return constants.DefaultInitSGASize
	}
	// 内存*80%*80%
	return strconv.Itoa(int(float64(memory) * 0.64))
}

func (in *OracleCluster) InitPGASize(memory int64) string {
	if memory == 0 {
		return constants.DefaultInitPGASize
	}
	// 内存*80%*20%
	return strconv.Itoa(int(float64(memory) * 0.16))
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

func (in *OracleCluster) SetStatus(statefulSet *appv1.StatefulSet) {
	in.Status.Ready = corev1.ConditionFalse
	if statefulSet.Status.ReadyReplicas == constants.DefaultReplicas {
		in.Status.Ready = corev1.ConditionTrue
	}
}

func (in *OracleCluster) SetDefault() {
	if len(in.Spec.PodSpec.ImagePullPolicy) == 0 {
		in.Spec.PodSpec.ImagePullPolicy = constants.DefaultPullPolicy
	}
	if len(in.Spec.PodSpec.OracleSID) == 0 {
		in.Spec.PodSpec.OracleSID = constants.DefaultOracleSID
	}
}

func init() {
	SchemeBuilder.Register(&OracleCluster{}, &OracleClusterList{})
}
