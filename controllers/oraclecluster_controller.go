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

package controllers

import (
	"context"
	"encoding/base64"
	"fmt"
	"github.com/go-logr/logr"
	"github.com/imdario/mergo"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	oraclev1 "oracle-operator/api/v1"
	"oracle-operator/utils"
	"oracle-operator/utils/constants"
	"oracle-operator/utils/options"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// OracleClusterReconciler reconciles a OracleCluster object
type OracleClusterReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	opt    *options.Options

	log      logr.Logger
	recorder record.EventRecorder
}

//+kubebuilder:rbac:groups=oracle.iwhalecloud.com,resources=oracleclusters,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=oracle.iwhalecloud.com,resources=oracleclusters/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=oracle.iwhalecloud.com,resources=oracleclusters/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=secrets;services;events,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// the OracleCluster object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.10.0/pkg/reconcile
func (r *OracleClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.log = log.FromContext(ctx)

	o := &oraclev1.OracleCluster{}
	err := r.Get(ctx, req.NamespacedName, o)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if o.DeletionTimestamp != nil {
		return ctrl.Result{}, nil
	}

	r.log.Info("Start Reconcile")
	old := o.DeepCopy()
	o.SetDefault()

	err = r.reconcileSecret(ctx, o)
	if err != nil {
		return ctrl.Result{}, err
	}

	err = r.reconcileSVC(ctx, o)
	if err != nil {
		return ctrl.Result{}, err
	}

	err = r.reconcileStatefulSet(ctx, o)
	if err != nil {
		return ctrl.Result{}, err
	}

	err = r.updateCluster(ctx, old, o)
	return ctrl.Result{}, err
}

func decodePWD(in string) ([]byte, error) {
	pwd, err := base64.StdEncoding.DecodeString(in)
	if err != nil {
		return nil, fmt.Errorf(".spec.password must be base64: %v", err)
	}
	return pwd, nil
}

func (r *OracleClusterReconciler) reconcileSecret(ctx context.Context, o *oraclev1.OracleCluster) error {
	r.log.Info("Reconcile Secret")
	secret := &corev1.Secret{}
	err := r.Get(ctx, types.NamespacedName{Name: o.UniteName(), Namespace: o.Namespace}, secret)
	if err == nil {
		return nil
	} else if !errors.IsNotFound(err) {
		return err
	}

	o.SetObject(secret)
	pwd, err := decodePWD(o.Spec.Password)
	if err != nil {
		return err
	}
	if err = ctrl.SetControllerReference(o, secret, r.Scheme); err != nil {
		return err
	}
	secret.Data = map[string][]byte{constants.OraclePWD: pwd}
	return r.eventCreated(r.Create(ctx, secret), o, "Secret", secret.Name)
}

func (r *OracleClusterReconciler) reconcileSVC(ctx context.Context, o *oraclev1.OracleCluster) error {
	svc := &corev1.Service{}
	o.SetObject(svc)
	operationResult, err := ctrl.CreateOrUpdate(ctx, r.Client, svc, func() error {
		svc.Spec.Type = corev1.ServiceTypeNodePort

		if len(svc.Spec.Ports) != 3 {
			svc.Spec.Ports = make([]corev1.ServicePort, 3)
		}
		if o.Spec.NodePort != 0 {
			svc.Spec.Ports[0].NodePort = o.Spec.NodePort
		}
		svc.Spec.Ports[0].Name = "listener"
		svc.Spec.Ports[0].Port = 1521

		svc.Spec.Ports[1].Name = "xmldb"
		svc.Spec.Ports[1].Port = 5500

		svc.Spec.Ports[2].Name = "tty"
		svc.Spec.Ports[2].Port = 8080

		svc.Spec.Selector = o.ClusterLabel()
		return ctrl.SetControllerReference(o, svc, r.Scheme)
	})
	r.log.Info("Reconcile SVC", "OperationResult", operationResult)
	return r.eventOperation(err, operationResult, o, "Service", svc.Name)
}

func (r *OracleClusterReconciler) reconcileStatefulSet(ctx context.Context, o *oraclev1.OracleCluster) error {
	statefulSet := &appv1.StatefulSet{}
	o.SetObject(statefulSet)
	operationResult, err := ctrl.CreateOrUpdate(ctx, r.Client, statefulSet, func() error {
		o.SetStatus(statefulSet)
		statefulSet.Spec.Selector = &metav1.LabelSelector{MatchLabels: o.ClusterLabel()}
		statefulSet.Spec.Template.Labels = o.ClusterLabel()
		statefulSet.Spec.ServiceName = o.UniteName()
		statefulSet.Spec.Replicas = new(int32)
		*statefulSet.Spec.Replicas = constants.DefaultReplicas

		podSpec := statefulSet.Spec.Template.Spec
		securityContext := &corev1.PodSecurityContext{RunAsUser: new(int64), FSGroup: new(int64)}
		*securityContext.RunAsUser = constants.SecurityContextRunAsUser
		*securityContext.FSGroup = constants.SecurityContextFsGroup
		podSpec.SecurityContext = securityContext
		if len(o.Spec.PodSpec.NodeSelector) != 0 {
			podSpec.NodeSelector = o.Spec.PodSpec.NodeSelector
		}

		var memory = o.MemoryValue()
		baseEnv := []corev1.EnvVar{
			{Name: "SVC_HOST", Value: o.Name},
			{Name: "SVC_PORT", Value: "1521"},
			{Name: "ORACLE_SID", Value: o.Spec.PodSpec.OracleSID},
			{Name: "ORACLE_PDB", Value: fmt.Sprintf("%sPDB", o.Spec.PodSpec.OracleSID)},
			{Name: "ORACLE_PWD", ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: o.UniteName()}, Key: constants.OraclePWD}}},
		}
		var oracleEnv = make([]corev1.EnvVar, len(baseEnv))
		copy(oracleEnv, baseEnv)
		oracleEnv = append(oracleEnv, []corev1.EnvVar{
			{Name: "ORACLE_CHARACTERSET", Value: "AL32UTF8"},
			{Name: "ORACLE_EDITION", Value: "enterprise"},
			{Name: "ENABLE_ARCHIVELOG", Value: "false"},
			{Name: "INIT_SGA_SIZE", Value: o.InitSGASize(memory)},
			{Name: "INIT_PGA_SIZE", Value: o.InitPGASize(memory)},
		}...)
		oracleEnv = utils.MergeEnv(oracleEnv, o.Spec.PodSpec.OracleEnv)
		oracleCLIEnv := utils.MergeEnv(baseEnv, o.Spec.PodSpec.OracleCLIEnv)

		if len(podSpec.Containers) != 3 {
			podSpec.Containers = make([]corev1.Container, 3)
		}
		// container oracle
		podSpec.Containers[0].Name = constants.ContainerOracle
		podSpec.Containers[0].Image = o.Spec.Image
		podSpec.Containers[0].ImagePullPolicy = o.Spec.PodSpec.ImagePullPolicy
		if len(podSpec.Containers[0].Ports) != 2 {
			podSpec.Containers[0].Ports = make([]corev1.ContainerPort, 2)
		}
		podSpec.Containers[0].Ports[0].ContainerPort = 1521
		podSpec.Containers[0].Ports[1].ContainerPort = 5500
		if podSpec.Containers[0].ReadinessProbe == nil {
			podSpec.Containers[0].ReadinessProbe = &corev1.Probe{}
		}
		podSpec.Containers[0].ReadinessProbe.Handler = corev1.Handler{
			Exec: &corev1.ExecAction{
				Command: []string{"/bin/sh", "-c", "$ORACLE_BASE/checkDBLockStatus.sh"},
			},
		}
		podSpec.Containers[0].ReadinessProbe.InitialDelaySeconds = 20
		podSpec.Containers[0].ReadinessProbe.PeriodSeconds = 40
		podSpec.Containers[0].ReadinessProbe.TimeoutSeconds = 20
		if len(podSpec.Containers[0].VolumeMounts) != 1 {
			podSpec.Containers[0].VolumeMounts = make([]corev1.VolumeMount, 1)
		}
		podSpec.Containers[0].VolumeMounts[0].MountPath = "/opt/oracle/oradata"
		podSpec.Containers[0].VolumeMounts[0].Name = constants.OracleVolumeName
		podSpec.Containers[0].Resources = o.Spec.PodSpec.Resources
		podSpec.Containers[0].Env = oracleEnv

		// container cli
		podSpec.Containers[1].Name = constants.ContainerOracleCli
		podSpec.Containers[1].Image = r.opt.CLIImage
		podSpec.Containers[1].ImagePullPolicy = o.Spec.PodSpec.ImagePullPolicy
		if len(podSpec.Containers[1].Ports) != 1 {
			podSpec.Containers[1].Ports = make([]corev1.ContainerPort, 1)
		}
		podSpec.Containers[1].Ports[0].ContainerPort = 8080
		podSpec.Containers[1].Env = oracleCLIEnv

		// container exporter
		podSpec.Containers[2].Name = constants.ContainerExporter
		podSpec.Containers[2].Image = r.opt.ExporterImage
		podSpec.Containers[2].ImagePullPolicy = o.Spec.PodSpec.ImagePullPolicy
		if len(podSpec.Containers[2].Ports) != 1 {
			podSpec.Containers[2].Ports = make([]corev1.ContainerPort, 1)
		}
		podSpec.Containers[2].Ports[0].ContainerPort = 9161

		pwd, err := decodePWD(o.Spec.Password)
		if err != nil {
			return err
		}
		//user/password@myhost:1521/service
		source := fmt.Sprintf("%s/%s@localhost:1521/%s", constants.DefaultExporterUser, pwd, o.Spec.PodSpec.OracleSID)
		podSpec.Containers[2].Env = []corev1.EnvVar{{Name: "DATA_SOURCE_NAME", Value: source}}
		statefulSet.Spec.Template.Spec = podSpec

		if len(statefulSet.Spec.VolumeClaimTemplates) != 1 {
			statefulSet.Spec.VolumeClaimTemplates = make([]corev1.PersistentVolumeClaim, 1)
			o.SetObject(&statefulSet.Spec.VolumeClaimTemplates[0])
			err = ctrl.SetControllerReference(o, &statefulSet.Spec.VolumeClaimTemplates[0], r.Scheme)
			if err != nil {
				return err
			}
		}
		statefulSet.Spec.VolumeClaimTemplates[0].Name = constants.OracleVolumeName
		err = mergo.Merge(&statefulSet.Spec.VolumeClaimTemplates[0].Spec, o.Spec.VolumeSpec.PersistentVolumeClaim)
		if err != nil {
			return err
		}
		return ctrl.SetControllerReference(o, statefulSet, r.Scheme)
	})
	r.log.Info("Reconcile StatefulSet", "OperationResult", operationResult)
	return r.eventOperation(err, operationResult, o, "StatefulSet", statefulSet.Name)
}

func (r *OracleClusterReconciler) updateCluster(ctx context.Context, old, o *oraclev1.OracleCluster) error {
	// TODO：Currently only the status needs to be updated
	if equality.Semantic.DeepEqual(old.Status, o.Status) {
		return nil
	}
	r.log.Info("update status", "old", old.Status, "new", o.Status)
	return r.Status().Update(ctx, o)
}

func (r *OracleClusterReconciler) eventCreated(err error, obj runtime.Object, kind, name string) error {
	if err != nil {
		return err
	}
	r.recorder.Eventf(obj, corev1.EventTypeNormal, constants.ReasonSuccessfulCreate, "create %s %s successful", kind, name)
	return nil
}

func (r *OracleClusterReconciler) eventOperation(err error, operationResult controllerutil.OperationResult, obj runtime.Object, kind, name string) error {
	if err != nil {
		return err
	}
	switch operationResult {
	case controllerutil.OperationResultCreated:
		return r.eventCreated(nil, obj, kind, name)
	case controllerutil.OperationResultNone:
		return nil
	default:
		r.recorder.Eventf(obj, corev1.EventTypeNormal, constants.ReasonReconciling, "reconciling %s %s, OperationResult: %s", kind, name, operationResult)
		return nil
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *OracleClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.recorder = mgr.GetEventRecorderFor("oraclecluster-controller")
	r.opt = options.GetOptions()
	return ctrl.NewControllerManagedBy(mgr).
		For(&oraclev1.OracleCluster{}).
		Owns(&corev1.Service{}).
		Owns(&appv1.StatefulSet{}).
		Complete(r)
}
