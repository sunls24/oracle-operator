package controllers

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	oraclev1 "oracle-operator/api/v1"
	"oracle-operator/utils"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sort"
	"time"
)

type BackupJob struct {
	Cluster            types.NamespacedName
	SecretName         string
	BackupHistoryLimit int
	Client             client.Client

	log          logr.Logger
	cancelBackup func()
}

func (cj *BackupJob) Cancel() {
	if cj.cancelBackup != nil {
		cj.cancelBackup()
	}
}

func (cj *BackupJob) RefreshLimit(limit int) {
	cj.BackupHistoryLimit = limit
	go cj.backupLimit()
}

func (cj *BackupJob) Run() {
	cj.Cancel()
	cj.log = log.FromContext(nil).WithValues("cluster", cj.Cluster).WithName("oraclebackupcron.backupJob")
	// 如果执行失败则会每分钟重试一次, 总计30次
	cj.cancelBackup = utils.Retry(cj.createBackup, time.Minute, 30, true)
}

func (cj *BackupJob) backupLimit() {
	if cj.BackupHistoryLimit == 0 {
		return
	}
	<-time.After(time.Second)

	list := oraclev1.OracleBackupList{}
	err := cj.Client.List(context.TODO(), &list, client.MatchingFields{"spec.clusterName": cj.Cluster.Name})
	if err != nil {
		cj.log.Error(err, "get cluster backup list error")
		return
	}
	if len(list.Items) == 0 || len(list.Items) <= cj.BackupHistoryLimit {
		return
	}
	sort.Slice(list.Items, func(i, j int) bool {
		return list.Items[i].CreationTimestamp.Before(&list.Items[j].CreationTimestamp)
	})

	count := len(list.Items) - cj.BackupHistoryLimit
	for i := 0; i < count; i++ {
		err = cj.Client.Delete(context.TODO(), &list.Items[i])
		if err != nil {
			cj.log.Error(err, "delete backup error", "backupName", list.Items[i].Name)
		}
	}
}

func (cj *BackupJob) createBackup(i int) bool {
	oc := &oraclev1.OracleCluster{}
	err := cj.Client.Get(context.TODO(), cj.Cluster, oc)
	if err != nil {
		cj.log.Error(err, "not found cluster when automatic backup", "retry", i)
		return false
	}

	if oc.Status.Status != oraclev1.ClusterStatusTrue {
		cj.log.Info("oracle status is not true when automatic backup", "status", oc.Status, "retry", i)
		return false
	}

	runningList, err := BackupRunningList(context.TODO(), cj.Client, cj.Cluster.Name)
	if err != nil {
		cj.log.Error(err, "backup running list", "retry", i)
		return false
	}
	if len(runningList) != 0 {
		cj.log.Info("at least a backup is running", "retry", i)
		return false
	}

	backupLabel := oc.ClusterLabel()
	backupLabel["auto"] = "true"
	backupName := fmt.Sprintf("auto-%s-%s", cj.Cluster.Name, time.Now().Format("20060102t150405"))
	backup := &oraclev1.OracleBackup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      backupName,
			Namespace: cj.Cluster.Namespace,
			Labels:    backupLabel,
		},
		Spec: oraclev1.OracleBackupSpec{
			ClusterName:      cj.Cluster.Name,
			BackupSecretName: cj.SecretName,
		},
	}

	if err = cj.Client.Create(context.TODO(), backup); err != nil {
		cj.log.Error(err, "create backup")
		return false
	}

	go cj.backupLimit()
	return true
}
