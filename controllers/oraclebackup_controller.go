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
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"oracle-operator/utils"
	"oracle-operator/utils/constants"
	"oracle-operator/utils/options"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	oraclev1 "oracle-operator/api/v1"
)

// OracleBackupReconciler reconciles a OracleBackup object
type OracleBackupReconciler struct {
	*rest.Config
	client.Client
	Scheme *runtime.Scheme
	opt    *options.Options

	log       logr.Logger
	recorder  record.EventRecorder
	clientSet *kubernetes.Clientset
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

	if ob.DeletionTimestamp != nil {
		// TODO: 执行Finalizers
		return ctrl.Result{}, nil
	}

	if ob.Status.Completed {
		// 已经备份完成
		return ctrl.Result{}, nil
	}

	// TODO: 执行备份命令
	oraclePod, err := r.clientSet.CoreV1().Pods(req.Namespace).
		List(ctx, metav1.ListOptions{LabelSelector: fmt.Sprintf("oracle=%s", ob.Spec.ClusterName)})
	if err != nil || oraclePod == nil || len(oraclePod.Items) == 0 {
		return ctrl.Result{}, fmt.Errorf("get oracle pod error: %v, oracle pod: %v", err, oraclePod)
	}

	err = utils.ExecCommand(r.clientSet, r.Config, req.Namespace, oraclePod.Items[0].Name, constants.ContainerOracle, []string{"echo", "TEST TEST !!! EXEC COMMAND!"})
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("exec command error: %v", err)
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *OracleBackupReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.recorder = mgr.GetEventRecorderFor("oraclecluster-controller")
	r.opt = options.GetOptions()
	var err error
	r.clientSet, err = kubernetes.NewForConfig(r.Config)
	if err != nil {
		return err
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&oraclev1.OracleBackup{}).
		Complete(r)
}
