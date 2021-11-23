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
	"github.com/go-logr/logr"
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
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// OracleClusterReconciler reconciles a OracleCluster object
type OracleClusterReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	log      logr.Logger
	recorder record.EventRecorder
}

//+kubebuilder:rbac:groups=oracle.iwhalecloud.com,resources=oracleclusters,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=oracle.iwhalecloud.com,resources=oracleclusters/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=oracle.iwhalecloud.com,resources=oracleclusters/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=secrets;services;events;persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete

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

	err = r.reconcilePVC(ctx, o)
	if err != nil {
		return ctrl.Result{}, err
	}

	err = r.reconcileSVC(ctx, o)
	if err != nil {
		return ctrl.Result{}, err
	}

	err = r.reconcileDeploy(ctx, o)
	if err != nil {
		return ctrl.Result{}, err
	}

	err = r.updateCluster(ctx, old, o)
	return ctrl.Result{}, err
}

func (r *OracleClusterReconciler) reconcileSecret(ctx context.Context, o *oraclev1.OracleCluster) error {
	r.log.Info("Reconcile Secret")
	secret := &corev1.Secret{}
	err := r.Get(ctx, types.NamespacedName{Name: o.UniqueName(), Namespace: o.Namespace}, secret)
	if err == nil {
		return nil
	} else if !errors.IsNotFound(err) {
		return err
	}

	o.SetObject(&secret.ObjectMeta)
	if err = ctrl.SetControllerReference(o, secret, r.Scheme); err != nil {
		return err
	}
	secret.Data = map[string][]byte{constants.OraclePWD: []byte(o.Spec.Password)}
	err = r.Create(ctx, secret)
	if err == nil {
		r.recorder.Eventf(o, corev1.EventTypeNormal, constants.ReasonSuccessfulCreate, "create Secret %s successful", secret.Name)
	}
	return err
}

func (r *OracleClusterReconciler) reconcilePVC(ctx context.Context, o *oraclev1.OracleCluster) error {
	r.log.Info("Reconcile PVC")
	pvc := &corev1.PersistentVolumeClaim{}
	err := r.Get(ctx, types.NamespacedName{Name: o.UniqueName(), Namespace: o.Namespace}, pvc)
	if err == nil {
		return nil
	} else if !errors.IsNotFound(err) {
		return err
	}

	o.SetObject(&pvc.ObjectMeta)
	if err = ctrl.SetControllerReference(o, pvc, r.Scheme); err != nil {
		return err
	}
	pvc.Spec = *o.Spec.VolumeSpec.PersistentVolumeClaim
	err = r.Create(ctx, pvc)
	if err == nil {
		r.recorder.Eventf(o, corev1.EventTypeNormal, constants.ReasonSuccessfulCreate, "create PersistentVolumeClaim %s successful", pvc.Name)
	}
	return err
}

func (r *OracleClusterReconciler) reconcileSVC(ctx context.Context, o *oraclev1.OracleCluster) error {
	svc := &corev1.Service{}
	o.SetObject(&svc.ObjectMeta)
	operationResult, err := ctrl.CreateOrUpdate(ctx, r.Client, svc, func() error {
		svc.Spec.Type = corev1.ServiceTypeNodePort
		svc.Spec.Ports = []corev1.ServicePort{
			{Name: "listener", Port: 1521, NodePort: o.Spec.NodePort},
			{Name: "xmldb", Port: 5500},
			{Name: "tty", Port: 8080},
		}
		svc.Spec.Selector = o.ClusterLabel()
		return ctrl.SetControllerReference(o, svc, r.Scheme)
	})
	r.log.Info("Reconcile SVC", "OperationResult", operationResult)
	switch operationResult {
	case controllerutil.OperationResultCreated:
		r.recorder.Eventf(o, corev1.EventTypeNormal, constants.ReasonSuccessfulCreate, "create Service %s successful", svc.Name)
	default:
		r.recorder.Eventf(o, corev1.EventTypeNormal, constants.ReasonReconciling, "reconciling Service %s, OperationResult: %s", svc.Name, operationResult)
	}
	return err
}

func (r *OracleClusterReconciler) reconcileDeploy(ctx context.Context, o *oraclev1.OracleCluster) error {
	deploy := &appv1.Deployment{}
	o.SetObject(&deploy.ObjectMeta)
	operationResult, err := ctrl.CreateOrUpdate(ctx, r.Client, deploy, func() error {
		o.SetStatus(deploy)
		deploy.Spec.Replicas = o.Spec.Replicas
		if o.Spec.PodSpec.Strategy != nil {
			deploy.Spec.Strategy = *o.Spec.PodSpec.Strategy
		}
		deploy.Spec.Selector = &metav1.LabelSelector{MatchLabels: o.ClusterLabel()}

		podSpec := corev1.PodTemplateSpec{}
		podSpec.Labels = o.ClusterLabel()
		if o.Spec.PodSpec.TerminationGracePeriodSeconds != nil {
			podSpec.Spec.TerminationGracePeriodSeconds = o.Spec.PodSpec.TerminationGracePeriodSeconds
		}

		securityContext := &corev1.PodSecurityContext{RunAsUser: new(int64), FSGroup: new(int64)}
		*securityContext.RunAsUser = constants.SecurityContextRunAsUser
		*securityContext.FSGroup = constants.SecurityContextFsGroup
		podSpec.Spec.SecurityContext = securityContext

		baseEnv := []corev1.EnvVar{
			{Name: "SVC_HOST", Value: o.Name},
			{Name: "SVC_PORT", Value: "1521"},
			{Name: "ORACLE_SID", Value: "CC"},
			{Name: "ORACLE_PDB", Value: "CCPDB1"},
			{Name: "ORACLE_PWD", ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: o.UniqueName()}, Key: constants.OraclePWD}}},
		}

		var oracleEnv = make([]corev1.EnvVar, len(baseEnv))
		copy(oracleEnv, baseEnv)
		oracleEnv = append(oracleEnv, []corev1.EnvVar{
			{Name: "ORACLE_CHARACTERSET", Value: "AL32UTF8"},
			{Name: "ORACLE_EDITION", Value: "enterprise"},
			{Name: "ENABLE_ARCHIVELOG", Value: "false"},
			{Name: "INIT_SGA_SIZE", Value: "4096"},
			{Name: "INIT_PGA_SIZE", Value: "1024"},
		}...)
		oracleEnv = utils.MergeEnv(oracleEnv, o.Spec.PodSpec.OracleEnv)
		oracleCLIEnv := utils.MergeEnv(baseEnv, o.Spec.PodSpec.OracleCLIEnv)

		podSpec.Spec.Containers = []corev1.Container{
			{
				Name:            constants.ContainerOracle,
				Image:           o.Spec.Image,
				ImagePullPolicy: o.Spec.PodSpec.ImagePullPolicy,
				Ports:           []corev1.ContainerPort{{ContainerPort: 1521}, {ContainerPort: 5500}},
				ReadinessProbe: &corev1.Probe{
					Handler: corev1.Handler{
						Exec: &corev1.ExecAction{
							Command: []string{"/bin/sh", "-c", "$ORACLE_BASE/checkDBLockStatus.sh"},
						},
					},
					InitialDelaySeconds: 20,
					PeriodSeconds:       40,
					TimeoutSeconds:      20,
				},
				VolumeMounts: []corev1.VolumeMount{{MountPath: "/opt/oracle/oradata", Name: constants.OracleVolumeName}},
				Resources:    o.Spec.PodSpec.Resources,
				Env:          oracleEnv,
			},
			{
				Name:            constants.ContainerOracleCli,
				Image:           o.Spec.CLIImage,
				ImagePullPolicy: o.Spec.PodSpec.ImagePullPolicy,
				Ports:           []corev1.ContainerPort{{ContainerPort: 8080}},
				Env:             oracleCLIEnv,
			},
		}

		podSpec.Spec.Volumes = []corev1.Volume{{Name: constants.OracleVolumeName, VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: o.UniqueName()}}}}
		deploy.Spec.Template = podSpec
		return ctrl.SetControllerReference(o, deploy, r.Scheme)
	})
	r.log.Info("Reconcile Deployment", "OperationResult", operationResult)
	switch operationResult {
	case controllerutil.OperationResultCreated:
		r.recorder.Eventf(o, corev1.EventTypeNormal, constants.ReasonSuccessfulCreate, "create Deployment %s successful", deploy.Name)
	default:
		r.recorder.Eventf(o, corev1.EventTypeNormal, constants.ReasonReconciling, "reconciling Deployment %s, OperationResult: %s", deploy.Name, operationResult)
	}
	return err
}

func (r *OracleClusterReconciler) updateCluster(ctx context.Context, old, o *oraclev1.OracleCluster) error {
	if equality.Semantic.DeepEqual(old.Status, o.Status) {
		return nil
	}
	r.log.Info("update status", "old", old.Status, "new", o.Status)
	return r.Status().Update(ctx, o)
}

// SetupWithManager sets up the controller with the Manager.
func (r *OracleClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.recorder = mgr.GetEventRecorderFor("oraclecluster-controller")
	return ctrl.NewControllerManagedBy(mgr).
		For(&oraclev1.OracleCluster{}).
		Complete(r)
}
