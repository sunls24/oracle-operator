package controllers

import (
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"time"
)

func key(namespace, name string) types.NamespacedName {
	return types.NamespacedName{Namespace: namespace, Name: name}
}

var (
	wait10s = ctrl.Result{RequeueAfter: 10 * time.Second}
)
