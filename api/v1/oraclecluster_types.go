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
	"context"
	"fmt"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"oracle-operator/utils/constants"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// OracleClusterSpec defines the desired state of OracleCluster
type OracleClusterSpec struct {
	Image       string `json:"image,omitempty"`
	Password    string `json:"password,omitempty"`
	NodePort    int32  `json:"nodePort,omitempty"`
	ArchiveMode bool   `json:"archiveMode,omitempty"`
	StartupMode string `json:"startupMode,omitempty"`

	BackupSchedule     string `json:"backupSchedule,omitempty"`
	BackupHistoryLimit int    `json:"backupHistoryLimit,omitempty"`
	BackupSecretName   string `json:"backupSecretName,omitempty"`

	PodSpec    PodSpec    `json:"podSpec,omitempty"`
	VolumeSpec VolumeSpec `json:"volumeSpec,omitempty"`
}

const (
	ClusterStatusTrue      = string(corev1.ConditionTrue)
	ClusterStatusWaiting   = "Waiting"
	ClusterStatusBackingUp = "BackingUp"
	ClusterStatusRestoring = "Restoring"
)

// OracleClusterStatus defines the observed state of OracleCluster
type OracleClusterStatus struct {
	Ready  corev1.ConditionStatus `json:"ready,omitempty"`
	Status string                 `json:"status,omitempty"`
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

func (in *OracleCluster) SetStatus(statefulSet *appv1.StatefulSet) {
	in.Status.Ready = corev1.ConditionFalse
	if statefulSet.Status.ReadyReplicas == constants.DefaultReplicas {
		in.Status.Ready = corev1.ConditionTrue
	}

	if in.Status.Status != ClusterStatusRestoring && in.Status.Status != ClusterStatusBackingUp {
		in.Status.Status = string(in.Status.Ready)
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

func (in *OracleCluster) GetPod(ctx context.Context, c client.Client) (*corev1.Pod, error) {
	podList := corev1.PodList{}
	err := c.List(ctx, &podList, client.MatchingLabels(map[string]string{"oracle": in.Name}))
	if err != nil || len(podList.Items) == 0 {
		return nil, fmt.Errorf("not found oracle pod: %v, oracle: %s", err, in.Name)
	}
	return &podList.Items[0], nil
}

func init() {
	SchemeBuilder.Register(&OracleCluster{}, &OracleClusterList{})
}
