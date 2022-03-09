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
	wait5s = ctrl.Result{RequeueAfter: 5 * time.Second}
)
