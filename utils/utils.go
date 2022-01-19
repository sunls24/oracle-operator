package utils

import (
	corev1 "k8s.io/api/core/v1"
	"strconv"
)

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

/*
InitMemorySize 计算初始化需要的PGA和SGA大小
当限制内存*80%*20%<2GB时(memory<12.5G): SGA=(限制内存-2GB)*80%, PGA_TARGET=SGA/4, PGA_LIMIT=2048M
当限制内存*80%*20%>=2GB时(memory>=12.5G)：SGA=限制内存*80%*80%, PGA_TARGET=限制内存*80%*20%*50%, PGA_LIMIT=限制内存*80%*20%
Return: PGA_TARGET, PGA_LIMIT, SGA
*/
const memoryLimit = 12800 // 12.5*1024
func InitMemorySize(memory float64) (pgaTarget, pgaLimit, sga string) {
	if memory <= 2048 {
		// 内存值小于2G不做处理
		return
	}
	if memory < memoryLimit {
		pgaLimit = "2048"
		sgaValue := (memory - 2048) * 0.8
		sga = strconv.Itoa(int(sgaValue))
		pgaTarget = strconv.Itoa(int(sgaValue / 4))
	} else {
		sga = strconv.Itoa(int(memory * 0.64))
		pgaLimitValue := memory * 0.16
		pgaLimit = strconv.Itoa(int(pgaLimitValue))
		pgaTarget = strconv.Itoa(int(pgaLimitValue * 0.5))
	}
	return
}
