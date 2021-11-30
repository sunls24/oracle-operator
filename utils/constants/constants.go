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
	DefaultPullPolicy  = corev1.PullIfNotPresent
	DefaultInitSGASize = "4096"
	DefaultInitPGASize = "1024"
	DefaultCLIImage    = "oracle/instantclient:19-gotty-3"
)

const (
	ReasonSuccessfulCreate = "SuccessfulCreate"
	ReasonReconciling      = "Reconciling"
)
