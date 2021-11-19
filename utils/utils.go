package utils

import corev1 "k8s.io/api/core/v1"

func MergeEnv(src, ex []corev1.EnvVar) []corev1.EnvVar {
	if len(ex) == 0 {
		return src
	}
	var exEnv []corev1.EnvVar
	for _, env := range ex {
		var flag bool
		for i := range src {
			if src[i].Name == env.Name {
				flag = true
				src[i].ValueFrom = env.ValueFrom
				src[i].Value = env.Value
				break
			}
		}
		if !flag {
			exEnv = append(exEnv, env)
		}
	}
	if len(exEnv) != 0 {
		src = append(src, exEnv...)
	}
	return src
}
