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
	xerr "github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
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

// Start 清理备份状态, 仅在首次运行执行
// 状态为 Running 的备份和恢复将状态设置为 Failed
// 设置 Oracle 的 Status 为 Ready 的状态
func (r *OracleBackupReconciler) Start(ctx context.Context) error {
	l := log.FromContext(ctx).WithName("BackupStatusClean")
	backupList := oraclev1.OracleBackupList{}
	err := r.List(ctx, &backupList)
	if err != nil {
		l.Error(err, "get oracle backup list error")
	}
	// 在Operator进程启动时状态为Running的备份属于异常备份,
	// 原备份进程已停止, 但状态未被设置, 会导致此备份无法正常进行(协调时会跳过)
	for _, bak := range backupList.Items {
		if bak.Status.BackupStatus != constants.StatusRunning {
			continue
		}
		bak.Status.BackupStatus = constants.StatusFailed
		if err = r.Status().Update(ctx, &bak); err != nil {
			l.Error(err, "update backup status error")
		}
	}

	restoreList := oraclev1.OracleRestoreList{}
	if err = r.List(ctx, &restoreList); err != nil {
		l.Error(err, "get oracle restore list error")
	}

	for _, restore := range restoreList.Items {
		if restore.Status.RestoreStatus != constants.StatusRunning {
			continue
		}
		restore.Status.RestoreStatus = constants.StatusFailed
		if err = r.Status().Update(ctx, &restore); err != nil {
			l.Error(err, "update restore status error")
		}
	}

	oraList := oraclev1.OracleClusterList{}
	if err = r.List(ctx, &oraList); err != nil {
		l.Error(err, "get oracle list error")
	}

	for _, oc := range oraList.Items {
		if oc.Status.Status != oraclev1.ClusterStatusRestoring && oc.Status.Status != oraclev1.ClusterStatusBackingUp {
			continue
		}
		oc.Status.Status = oraclev1.ClusterStatusWaiting
		if err = r.Status().Update(ctx, &oc); err != nil {
			l.Error(err, "update oracle status error")
		}
	}
	return nil
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
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	r.log.Info("Start Reconcile")

	if exit, err := r.backupFinalizer(ctx, ob); exit {
		return ctrl.Result{}, xerr.Wrap(err, "backup finalizer")
	}

	if len(ob.Status.BackupStatus) != 0 && ob.Status.BackupStatus != constants.StatusFailed {
		// 已经完成备份或者正在备份
		return ctrl.Result{}, nil
	}

	oc := &oraclev1.OracleCluster{}
	err = r.Get(ctx, key(ob.Namespace, ob.Spec.ClusterName), oc)
	if err != nil {
		r.log.Error(err, "not found cluster when backup")
		return ctrl.Result{}, nil
	}

	if oc.Status.Status != oraclev1.ClusterStatusTrue && oc.Status.Status != oraclev1.ClusterStatusBackingUp {
		r.log.Info("oracle status is not true, wait 5s", "oracle", oc.Name, "status", oc.Status.Status)
		return wait10s, nil
	}

	runningList, err := BackupRunningList(ctx, r.Client, oc.Name)
	if err != nil {
		return ctrl.Result{}, xerr.Wrap(err, "backup running list")
	}
	if len(runningList) != 0 {
		r.log.Info("at least a backup is running, wait 5s")
		return wait10s, nil
	}

	oraclePod, err := oc.GetPod(ctx, r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}

	oc.Status.Status = oraclev1.ClusterStatusBackingUp
	if err = r.Status().Update(ctx, oc); err != nil {
		return ctrl.Result{}, xerr.Wrap(err, "update cluster status")
	}
	defer func() {
		_ = r.Get(ctx, key(oc.Namespace, oc.Name), oc)
		oc.Status.Status = oraclev1.ClusterStatusWaiting
		if err = r.Status().Update(ctx, oc); err != nil {
			r.log.Error(err, "update cluster status error")
		}
	}()

	ob.Status.BackupStatus = constants.StatusRunning
	if err = r.Status().Update(ctx, ob); err != nil {
		return ctrl.Result{}, xerr.Wrap(err, "update backup status")
	}
	defer func() {
		if err = r.Status().Update(ctx, ob); err != nil {
			r.log.Error(err, "update backup status error")
		}
	}()

	osbwsInstallCmd, err := r.osbwsInstallCmd(ctx, ob, oc)
	if err != nil {
		ob.Status.BackupStatus = constants.StatusFailed
		return ctrl.Result{}, xerr.Wrap(err, "get osbwsInstallCmd")
	}

	err = r.execCommand(req.Namespace, oraclePod.Name, constants.ContainerOracle, osbwsInstallCmd)
	if err != nil {
		ob.Status.BackupStatus = constants.StatusFailed
		return ctrl.Result{}, xerr.Wrap(err, "exec osbwsInstall command")
	}

	backupCmd, backupTag := r.backupCommand(oc)
	err = r.execCommand(req.Namespace, oraclePod.Name, constants.ContainerOracle, backupCmd)
	if err != nil {
		ob.Status.BackupStatus = constants.StatusFailed
		return ctrl.Result{}, xerr.Wrap(err, "exec backup command")
	}

	ob.Status.BackupTag = backupTag
	ob.Status.BackupStatus = constants.StatusCompleted
	if ob.Status.CompletedTime == nil || ob.Status.CompletedTime.IsZero() {
		t := metav1.NewTime(time.Now())
		ob.Status.CompletedTime = &t
	}
	return ctrl.Result{}, nil
}

func (r *OracleBackupReconciler) backupFinalizer(ctx context.Context, ob *oraclev1.OracleBackup) (bool, error) {
	if ob.DeletionTimestamp == nil {
		if !utils.AddFinalizer(ob, finalizeBackupCleanup) {
			return false, nil
		}
		if err := r.Client.Update(ctx, ob); err != nil {
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

	oc := &oraclev1.OracleCluster{}
	err := r.Get(ctx, key(ob.Namespace, ob.Spec.ClusterName), oc)
	if err != nil {
		if errors.IsNotFound(err) {
			return true, r.Client.Update(ctx, ob)
		}
		return true, xerr.Wrap(err, "not found oracle")
	}

	oraclePod, err := oc.GetPod(ctx, r.Client)
	if err != nil {
		return true, err
	}

	err = r.execCommand(ob.Namespace, oraclePod.Name, constants.ContainerOracle, r.backupDeleteCommand(ob.Status.BackupTag))
	if err != nil {
		return true, xerr.Wrap(err, "exec delete backup command")
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

	oracleSID := strings.ToUpper(oc.Spec.PodSpec.OracleSID)
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
	return fmt.Sprintf(r.opt.BackupCmd, strings.ToUpper(oc.Spec.PodSpec.OracleSID), bakTAG), bakTAG
}

func (r *OracleBackupReconciler) backupDeleteCommand(backupTag string) string {
	return fmt.Sprintf(r.opt.BackupDeleteCmd, backupTag)
}

func (r *OracleBackupReconciler) execCommand(namespace, podName, container, command string) error {
	return utils.ExecCommand(r.Clientset, r.Config, namespace, podName, container, []string{"sh", "-c", command})
}

func BackupRunningList(ctx context.Context, c client.Client, clusterName string) ([]oraclev1.OracleBackup, error) {
	list := oraclev1.OracleBackupList{}
	err := c.List(ctx, &list, client.MatchingFields{"status.backupStatus": constants.StatusRunning})
	if err != nil || len(list.Items) == 0 {
		return list.Items, err
	}

	filter := make([]oraclev1.OracleBackup, 0)
	for _, bak := range list.Items {
		if bak.Spec.ClusterName == clusterName {
			filter = append(filter, bak)
		}
	}
	return filter, err
}

// SetupWithManager sets up the controller with the Manager.
func (r *OracleBackupReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.recorder = mgr.GetEventRecorderFor("oraclebackup-controller")
	r.opt = options.GetOptions()

	if err := addBackupFieldIndexers(mgr); err != nil {
		return err
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&oraclev1.OracleBackup{}).
		WithOptions(controller.Options{MaxConcurrentReconciles: 2}).
		Complete(r)
}

func addBackupFieldIndexers(mgr manager.Manager) error {
	err := mgr.GetFieldIndexer().IndexField(context.TODO(), &oraclev1.OracleBackup{}, "status.backupStatus",
		func(obj client.Object) []string {
			return []string{obj.(*oraclev1.OracleBackup).Status.BackupStatus}
		})
	if err != nil {
		return err
	}
	return mgr.GetFieldIndexer().IndexField(context.TODO(), &oraclev1.OracleBackup{}, "spec.clusterName",
		func(obj client.Object) []string {
			return []string{obj.(*oraclev1.OracleBackup).Spec.ClusterName}
		})
}
