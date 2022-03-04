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
	"fmt"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"oracle-operator/utils"
	"oracle-operator/utils/constants"
	"oracle-operator/utils/options"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"strings"
	"time"

	oraclev1 "oracle-operator/api/v1"
)

const finalizeBackupCleanup = "backup-cleanup"

// OracleBackupReconciler reconciles a OracleBackup object
type OracleBackupReconciler struct {
	*rest.Config
	*kubernetes.Clientset

	client.Client
	Scheme *runtime.Scheme

	log      logr.Logger
	opt      *options.Options
	recorder record.EventRecorder
}

//+kubebuilder:rbac:groups=oracle.iwhalecloud.com,resources=oraclebackups,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=oracle.iwhalecloud.com,resources=oraclebackups/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=oracle.iwhalecloud.com,resources=oraclebackups/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the OracleBackup object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.0/pkg/reconcile··
func (r *OracleBackupReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.log = log.FromContext(ctx)

	ob := &oraclev1.OracleBackup{}
	err := r.Get(ctx, req.NamespacedName, ob)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	r.log.Info("Start Reconcile")

	if exit, err := r.backupFinalizer(ctx, ob); exit {
		return ctrl.Result{}, err
	}

	if len(ob.Status.BackupStatus) != 0 && ob.Status.BackupStatus != oraclev1.BackupStatusFailed {
		// 已经完成备份或者正在备份
		return ctrl.Result{}, nil
	}

	oc := &oraclev1.OracleCluster{}
	err = r.Client.Get(ctx, types.NamespacedName{Namespace: ob.Namespace, Name: ob.Spec.ClusterName}, oc)
	if err != nil {
		r.log.Error(err, "not found cluster when backup", "cluster", ob.Spec.ClusterName)
		return ctrl.Result{}, nil
	}

	if oc.Status.Ready != corev1.ConditionTrue {
		r.log.Info("oracle status is not ready, wait 5s", "oracle", oc.Name)
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	oraclePod, err := r.getOraclePod(ctx, ob)
	if err != nil {
		return ctrl.Result{}, err
	}

	ob.Status.BackupStatus = oraclev1.BackupStatusRunning
	err = r.Status().Update(ctx, ob)
	if err != nil {
		return ctrl.Result{}, err
	}

	old := ob.DeepCopy()
	defer func() {
		err = r.updateBackup(ctx, old, ob)
		if err != nil {
			r.log.Error(err, "update backup status error")
		}
	}()

	osbwsInstallCmd, err := r.osbwsInstallCmd(ctx, ob, oc)
	if err != nil {
		ob.Status.BackupStatus = oraclev1.BackupStatusFailed
		return ctrl.Result{}, fmt.Errorf("get osbwsInstallCmd error: %v", err)
	}

	err = r.execCommand(req.Namespace, oraclePod.Name, constants.ContainerOracle, osbwsInstallCmd)
	if err != nil {
		ob.Status.BackupStatus = oraclev1.BackupStatusFailed
		return ctrl.Result{}, fmt.Errorf("exec osbwsInstall command error: %v", err)
	}

	backupCmd, backupTag := r.backupCommand(oc)
	err = r.execCommand(req.Namespace, oraclePod.Name, constants.ContainerOracle, backupCmd)
	if err != nil {
		ob.Status.BackupStatus = oraclev1.BackupStatusFailed
		return ctrl.Result{}, fmt.Errorf("exec backup command error: %v", err)
	}

	ob.Status.BackupTag = backupTag
	ob.Status.BackupStatus = oraclev1.BackupStatusCompleted
	return ctrl.Result{}, nil
}

func (r *OracleBackupReconciler) backupFinalizer(ctx context.Context, ob *oraclev1.OracleBackup) (bool, error) {
	if ob.DeletionTimestamp == nil {
		if !utils.AddFinalizer(ob, finalizeBackupCleanup) {
			return false, nil
		}
		err := r.Client.Update(ctx, ob)
		if err != nil {
			r.log.Error(err, "add finalizer error")
		}
		return false, nil
	}
	if !utils.DeleteFinalizer(ob, finalizeBackupCleanup) {
		return true, nil
	}

	if len(ob.Status.BackupTag) == 0 {
		return true, r.Client.Update(ctx, ob)
	}

	// 删除备份
	oraclePod, err := r.getOraclePod(ctx, ob)
	if err != nil {
		return true, fmt.Errorf("handle backup finalizer error: %v", err)
	}
	err = r.execCommand(ob.Namespace, oraclePod.Name, constants.ContainerOracle, r.backupDeleteCommand(ob.Status.BackupTag))
	if err != nil {
		return true, fmt.Errorf("backup delete command error: %v", err)
	}
	return true, r.Client.Update(ctx, ob)
}

func (r *OracleBackupReconciler) osbwsInstallCmd(ctx context.Context, ob *oraclev1.OracleBackup, oc *oraclev1.OracleCluster) (string, error) {
	var command string

	secret := &corev1.Secret{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: ob.Spec.BackupSecretName, Namespace: ob.Namespace}, secret)
	if err != nil {
		return command, err
	}

	oracleSID := oc.Spec.PodSpec.OracleSID
	awsID := string(secret.Data[constants.SecretAWSID])
	awsKey := string(secret.Data[constants.SecretAWSKey])
	if len(awsID) == 0 || len(awsKey) == 0 {
		return command, fmt.Errorf("backup secret is error, awsID: %s, awsKey: %s", awsID, awsKey)
	}

	endPoint := string(secret.Data[constants.SecretEndpoint])
	if index := strings.Index(endPoint, "//"); index >= 0 {
		endPoint = endPoint[index+2:]
	}
	spEndpoint := strings.Split(endPoint, ":")
	if len(spEndpoint) != 2 {
		return command, fmt.Errorf("endpoint split error, endpoint: %s", endPoint)
	}
	awsEndpoint := spEndpoint[0]
	awsPort := spEndpoint[1]

	command = fmt.Sprintf(r.opt.OSBWSInstallCmd, oracleSID, awsID, awsKey, awsEndpoint, awsPort)
	return command, nil
}

func (r *OracleBackupReconciler) backupCommand(oc *oraclev1.OracleCluster) (string, string) {
	bakTAG := fmt.Sprintf("BAK%s", time.Now().Format("20060102T150405"))
	return fmt.Sprintf(r.opt.BackupCmd, oc.Spec.PodSpec.OracleSID, bakTAG), bakTAG
}

func (r *OracleBackupReconciler) backupDeleteCommand(backupTag string) string {
	return fmt.Sprintf(r.opt.BackupDeleteCmd, backupTag)
}

func (r *OracleBackupReconciler) execCommand(namespace, podName, container, command string) error {
	return utils.ExecCommand(r.Clientset, r.Config, namespace, podName, container, []string{"sh", "-c", command})
}

func (r *OracleBackupReconciler) getOraclePod(ctx context.Context, ob *oraclev1.OracleBackup) (*corev1.Pod, error) {
	oraclePodList := corev1.PodList{}
	err := r.Client.List(ctx, &oraclePodList, client.MatchingLabels(map[string]string{"oracle": ob.Spec.ClusterName}))
	if err != nil || len(oraclePodList.Items) == 0 {
		return nil, fmt.Errorf("not found oracle pod: %v", err)
	}
	return &oraclePodList.Items[0], nil
}

func (r *OracleBackupReconciler) updateBackup(ctx context.Context, old, ob *oraclev1.OracleBackup) error {
	// TODO：Currently only the status needs to be updated
	if equality.Semantic.DeepEqual(old.Status, ob.Status) {
		return nil
	}
	r.log.Info("update backup status", "old", old.Status, "new", ob.Status)
	return r.Client.Status().Update(ctx, ob)
}

// SetupWithManager sets up the controller with the Manager.
func (r *OracleBackupReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.recorder = mgr.GetEventRecorderFor("oraclebackup-controller")
	r.opt = options.GetOptions()
	return ctrl.NewControllerManagedBy(mgr).
		For(&oraclev1.OracleBackup{}).
		Complete(r)
}
