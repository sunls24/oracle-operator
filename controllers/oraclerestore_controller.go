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
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"oracle-operator/utils"
	"oracle-operator/utils/constants"
	"oracle-operator/utils/options"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	oraclev1 "oracle-operator/api/v1"
)

// OracleRestoreReconciler reconciles a OracleRestore object
type OracleRestoreReconciler struct {
	*rest.Config
	*kubernetes.Clientset

	client.Client
	Scheme *runtime.Scheme

	log      logr.Logger
	opt      *options.Options
	recorder record.EventRecorder
}

//+kubebuilder:rbac:groups=oracle.iwhalecloud.com,resources=oraclerestores,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=oracle.iwhalecloud.com,resources=oraclerestores/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=oracle.iwhalecloud.com,resources=oraclerestores/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the OracleRestore object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.0/pkg/reconcile
func (r *OracleRestoreReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.log = log.FromContext(ctx)

	or := &oraclev1.OracleRestore{}
	err := r.Get(ctx, req.NamespacedName, or)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	r.log.Info("Start Reconcile")

	if len(or.Status.RestoreStatus) != 0 && or.Status.RestoreStatus != constants.StatusFailed {
		// 已经完成恢复或者正在恢复
		return ctrl.Result{}, nil
	}

	oc := &oraclev1.OracleCluster{}
	err = r.Get(ctx, key(or.Namespace, or.Spec.ClusterName), oc)
	if err != nil {
		r.log.Error(err, "not found cluster when restore")
		return ctrl.Result{}, nil
	}

	if oc.Status.Status != oraclev1.ClusterStatusTrue && oc.Status.Status != oraclev1.ClusterStatusRestoring {
		r.log.Info("oracle status is not true, wait 5s", "oracle", oc.Name, "status", oc.Status.Status)
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	ob := &oraclev1.OracleBackup{}
	err = r.Get(ctx, types.NamespacedName{Name: or.Spec.BackupName, Namespace: or.Namespace}, ob)
	if err != nil {
		r.log.Error(err, "not found backup when restore")
		return ctrl.Result{}, nil
	}

	if ob.Status.BackupStatus == constants.StatusRunning {
		r.log.Info("backup is running, wait 5s", "backup", ob.Name)
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	if ob.Status.BackupStatus != constants.StatusCompleted {
		r.log.Info("exit restore, backup status not completed")
		return ctrl.Result{}, nil
	}

	oraclePod, err := oc.GetPod(ctx, r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}

	oc.Status.Status = oraclev1.ClusterStatusRestoring
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

	or.Status.RestoreStatus = constants.StatusRunning
	if err = r.Status().Update(ctx, or); err != nil {
		return ctrl.Result{}, xerr.Wrap(err, "update restore error")
	}
	defer func() {
		if err = r.Status().Update(ctx, or); err != nil {
			r.log.Error(err, "update restore status error")
		}
	}()

	restoreCommand := r.restoreCommand(ob.Status.BackupTag)
	err = r.execCommand(or.Namespace, oraclePod.Name, constants.ContainerOracle, restoreCommand)
	if err != nil {
		return ctrl.Result{}, xerr.Wrap(err, "exec restore command")
	}

	or.Status.RestoreStatus = constants.StatusCompleted
	return ctrl.Result{}, nil
}

func (r *OracleRestoreReconciler) restoreCommand(backupTag string) string {
	return fmt.Sprintf(r.opt.RestoreCmd, backupTag)
}

func (r *OracleRestoreReconciler) execCommand(namespace, podName, container, command string) error {
	return utils.ExecCommand(r.Clientset, r.Config, namespace, podName, container, []string{"sh", "-c", command})
}

// SetupWithManager sets up the controller with the Manager.
func (r *OracleRestoreReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.recorder = mgr.GetEventRecorderFor("oraclerestore-controller")
	r.opt = options.GetOptions()
	return ctrl.NewControllerManagedBy(mgr).
		For(&oraclev1.OracleRestore{}).
		Complete(r)
}
