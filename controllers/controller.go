package controllers

import "k8s.io/apimachinery/pkg/types"

func key(namespace, name string) types.NamespacedName {
	return types.NamespacedName{Namespace: namespace, Name: name}
}
