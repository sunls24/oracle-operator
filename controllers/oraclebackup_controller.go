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
	"sigs.k8s.io/controller-runtime/pkg/log"
	"strings"

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

	if len(ob.Status.BackupStatus) != 0 {
		// 已经备份完成或者正在备份
		return ctrl.Result{}, nil
	}

	// TODO: 执行备份命令
	oraclePod, err := r.clientSet.CoreV1().Pods(req.Namespace).
		List(ctx, metav1.ListOptions{LabelSelector: fmt.Sprintf("oracle=%s", ob.Spec.ClusterName)})
	if err != nil || oraclePod == nil || len(oraclePod.Items) == 0 {
		return ctrl.Result{}, fmt.Errorf("get oracle pod error: %v, oracle pod: %v", err, oraclePod)
	}

	osbwsInstallCmd, err := r.osbwsInstallCmd(ctx, ob)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("get osbwsInstallCmd error: %v", err)
	}

	err = utils.ExecCommand(r.clientSet, r.Config, req.Namespace, oraclePod.Items[0].Name, constants.ContainerOracle,
		[]string{"sh", "-c", osbwsInstallCmd})
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("exec command error: %v", err)
	}

	return ctrl.Result{}, nil
}

func (r *OracleBackupReconciler) osbwsInstallCmd(ctx context.Context, ob *oraclev1.OracleBackup) (string, error) {
	var command string
	oc := &oraclev1.OracleCluster{}
	err := r.Client.Get(ctx, types.NamespacedName{Namespace: ob.Namespace, Name: ob.Spec.ClusterName}, oc)
	if err != nil {
		return command, err
	}

	secret := &corev1.Secret{}
	err = r.Client.Get(ctx, types.NamespacedName{Name: ob.Spec.BackupSecretName, Namespace: ob.Namespace}, secret)
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

	command = fmt.Sprintf(constants.DefaultOSBWSInstallCmd, oracleSID, awsID, awsKey, awsEndpoint, awsPort)
	return command, nil
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
