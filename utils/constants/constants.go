package constants

import corev1 "k8s.io/api/core/v1"

const (
	OraclePWD        = "ORACLE_PASSWORD"
	OracleVolumeName = "data"
)

const (
	TerminationGracePeriodSeconds = 30
	SecurityContextRunAsUser      = 54321
	SecurityContextFsGroup        = 54321
)

const (
	ContainerOracle    = "oracle"
	ContainerOracleCli = "oracle-cli"
)

const (
	DefaultPullPolicy = corev1.PullIfNotPresent
)
