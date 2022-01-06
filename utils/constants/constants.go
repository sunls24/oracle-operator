package constants

import corev1 "k8s.io/api/core/v1"

const (
	OraclePWD               = "ORACLE_PASSWORD"
	OracleVolumeName        = "data"
	DefaultLeaderElectionID = "oracle-operator-leader-election"

	InitPGASize      = "INIT_PGA_SIZE"
	InitPGALimitSize = "INIT_PGA_LIMIT_SIZE"
	InitSGASize      = "INIT_SGA_SIZE"
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
	DefaultPullPolicy       = corev1.PullIfNotPresent
	DefaultCLIImage         = "oracle/instantclient:19-gotty-3"
	DefaultReplicas   int32 = 1
	DefaultOracleSID        = "CC"
)

const (
	ReasonSuccessfulCreate = "SuccessfulCreate"
	ReasonReconciling      = "Reconciling"
)
