package constants

import corev1 "k8s.io/api/core/v1"

const (
	OraclePWD               = "ORACLE_PASSWORD"
	OracleVolumeName        = "data"
	DefaultLeaderElectionID = "oracle-operator-leader-election"
)

const (
	SecurityContextRunAsUser = 54321
	SecurityContextFsGroup   = 54321
)

const (
	ContainerOracle    = "oracle"
	ContainerOracleCli = "oracle-cli"
)

const (
	DefaultPullPolicy = corev1.PullIfNotPresent
)

const (
	ReasonSuccessfulCreate = "SuccessfulCreate"
	ReasonReconciling      = "Reconciling"
)
