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
	xerr "github.com/pkg/errors"
	"github.com/robfig/cron/v3"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	oraclev1 "oracle-operator/api/v1"
	"oracle-operator/utils/options"
	"reflect"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sync"
)

var (
	defaultParser = cron.NewParser(
		cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.DowOptional | cron.Descriptor)
)

type startStopCron struct {
	cron *cron.Cron
}

func (c startStopCron) Start(ctx context.Context) error {
	c.cron.Start()
	<-ctx.Done()
	c.cron.Stop()

	return nil
}

type OracleBackupCronReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	log      logr.Logger
	opt      *options.Options
	recorder record.EventRecorder

	cron        *cron.Cron
	cronJobLock *sync.Mutex
}

func (r *OracleBackupCronReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.log = log.FromContext(ctx)

	oc := &oraclev1.OracleCluster{}
	err := r.Get(ctx, req.NamespacedName, oc)
	if err != nil {
		if errors.IsNotFound(err) {
			r.unregisterCluster(req.NamespacedName)
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	if len(oc.Spec.BackupSchedule) == 0 {
		r.unregisterCluster(req.NamespacedName)
		return reconcile.Result{}, nil
	}

	r.log.Info("Start Reconcile")

	if len(oc.Spec.BackupSecretName) == 0 {
		r.log.Info("backup secret name is not specified, return")
		return ctrl.Result{}, nil
	}

	schedule, err := defaultParser.Parse(oc.Spec.BackupSchedule)
	if err != nil {
		return reconcile.Result{}, xerr.Wrap(err, "failed to parse schedule")
	}

	r.updateClusterSchedule(oc, schedule)
	return ctrl.Result{}, nil
}

func (r *OracleBackupCronReconciler) updateClusterSchedule(oc *oraclev1.OracleCluster, schedule cron.Schedule) {
	r.cronJobLock.Lock()
	defer r.cronJobLock.Unlock()

	for _, entry := range r.cron.Entries() {
		job, ok := entry.Job.(*BackupJob)
		if !ok || job.Cluster.Name != oc.Name || job.Cluster.Namespace != oc.Namespace {
			continue
		}

		// 策略改变, 重新计时
		if !reflect.DeepEqual(entry.Schedule, schedule) {
			r.log.Info("cluster backup schedule is change, update", "schedule", oc.Spec.BackupSchedule)
			job.Cancel()
			r.cron.Remove(entry.ID)
			break
		}

		if oc.Spec.BackupHistoryLimit != job.BackupHistoryLimit {
			r.log.Info("cluster backup history limit is change, update", "limit", oc.Spec.BackupHistoryLimit)
			job.RefreshLimit(oc.Spec.BackupHistoryLimit)
		}
		if oc.Spec.BackupSecretName != job.SecretName {
			r.log.Info("cluster backup secret name is change, update", "secret", oc.Spec.BackupSecretName)
			job.SecretName = oc.Spec.BackupSecretName
		}
		return
	}

	r.cron.Schedule(schedule, &BackupJob{
		Cluster:            types.NamespacedName{Name: oc.Name, Namespace: oc.Namespace},
		SecretName:         oc.Spec.BackupSecretName,
		BackupHistoryLimit: oc.Spec.BackupHistoryLimit,
		Client:             r.Client,
	})
}

func (r *OracleBackupCronReconciler) unregisterCluster(key types.NamespacedName) {
	r.cronJobLock.Lock()
	defer r.cronJobLock.Unlock()

	for _, entry := range r.cron.Entries() {
		j, ok := entry.Job.(*BackupJob)
		if ok && j.Cluster == key {
			j.Cancel()
			r.cron.Remove(entry.ID)
			break
		}
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *OracleBackupCronReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.recorder = mgr.GetEventRecorderFor("oraclebackupcron-controller")
	r.opt = options.GetOptions()
	r.cronJobLock = new(sync.Mutex)
	r.cron = cron.New()

	if err := mgr.Add(startStopCron{r.cron}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		Named("oraclebackupcron").
		For(&oraclev1.OracleCluster{}).
		Complete(r)
}
