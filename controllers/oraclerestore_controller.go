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
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"oracle-operator/utils"
	"oracle-operator/utils/options"

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

	ob := &oraclev1.OracleRestore{}
	err := r.Get(ctx, req.NamespacedName, ob)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	r.log.Info("Start Reconcile")

	return ctrl.Result{}, nil
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
