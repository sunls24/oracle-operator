package constants

import (
	corev1 "k8s.io/api/core/v1"
	"strconv"
)

const (
	OraclePWD        = "ORACLE_PASSWORD"
	OracleVolumeName = "data"

	InitPGASize      = "INIT_PGA_SIZE"
	InitPGALimitSize = "INIT_PGA_LIMIT_SIZE"
	InitSGASize      = "INIT_SGA_SIZE"
	EnableArchiveLog = "ENABLE_ARCHIVELOG"

	SecretAWSID    = "AWS_ACCESS_KEY_ID"
	SecretAWSKey   = "AWS_SECRET_KEY"
	SecretEndpoint = "S3_ENDPOINT"
)

const (
	SecurityContextRunAsUser = 54321
	SecurityContextFsGroup   = 54321
)

const (
	ContainerOracle    = "oracle"
	ContainerOracleCli = "oracle-cli"
	ContainerExporter  = "exporter"
)

const (
	DefaultPullPolicy         = corev1.PullIfNotPresent
	DefaultReplicas     int32 = 1
	DefaultOracleSID          = "CC"
	DefaultExporterUser       = "system"
	DefaultExportPort   int32 = 9161

	DefaultCLIImage         = "oracle/instantclient:19-gotty-3"
	DefaultExporterImage    = "iamseth/oracledb_exporter"
	DefaultLeaderElectionID = "oracle-operator-leader-election"
	DefaultOSBWSInstallCmd  = `export ORACLE_HOME=/opt/oracle/oradata/orclhome
export ORACLE_SID=%s
mkdir -p $ORACLE_HOME/lib
mkdir -p $ORACLE_HOME/dbs/osbws_wallet
java -jar $ORACLE_BASE/osbws_install.jar -walletDir $ORACLE_HOME/dbs/osbws_wallet -AWSID %s -AWSKey %s -awsEndpoint %s -awsPort %s -location default -no-import-certificate -debug -libDir $ORACLE_HOME/lib -useSigV2`
)

const (
	ReasonSuccessfulCreate = "SuccessfulCreate"
	ReasonReconciling      = "Reconciling"
)

var ExportAnnotations = map[string]string{"prometheus.io/port": strconv.Itoa(int(DefaultExportPort)), "prometheus.io/scrape": "true"}
