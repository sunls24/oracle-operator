package constants

const (
	DefaultOSBWSInstallCmd = `
export ORACLE_HOME=/opt/oracle/oradata/orclhome
export ORACLE_SID=%s
mkdir -p ${ORACLE_HOME}/lib
mkdir -p ${ORACLE_HOME}/dbs/osbws_wallet
java -jar ${ORACLE_BASE}/osbws_install.jar -walletDir ${ORACLE_HOME}/dbs/osbws_wallet -AWSID %s -AWSKey %s -awsEndpoint %s -awsPort %s -location default -no-import-certificate -debug -libDir ${ORACLE_HOME}/lib -useSigV2`

	DefaultBackupCmd = `
export BACKUP_HOME=/opt/oracle/oradata/orclhome
export ORACLE_SID=%s
export BACKUP_TAG=%s
rman target / <<EOF
CONFIGURE DEVICE TYPE 'SBT_TAPE' PARALLELISM 4 BACKUP TYPE TO BACKUPSET;
CONFIGURE CHANNEL DEVICE TYPE SBT parms='SBT_LIBRARY=${BACKUP_HOME}/lib/libosbws.so,SBT_PARMS=(OSB_WS_PFILE=${BACKUP_HOME}/dbs/osbws${ORACLE_SID}.ora)';
CONFIGURE DEFAULT DEVICE TYPE TO SBT;
CONFIGURE COMPRESSION ALGORITHM clear;
CONFIGURE CONTROLFILE AUTOBACKUP OFF;
SHOW ALL;
RUN {
  BACKUP DATABASE SECTION SIZE=4G TAG='${BACKUP_TAG}' PLUS ARCHIVELOG DELETE INPUT TAG='${BACKUP_TAG}';
}
EXIT;
EOF`

	DefaultBackupDeleteCmd = `
export ORACLE_SID=${ORACLE_SID^^}
rman target / <<EOF
RUN {
  DELETE NOPROMPT BACKUP TAG='%s';
}
EXIT;
EOF`

	DefaultRestoreCmd = `
export ORACLE_SID=${ORACLE_SID^^}
export BACKUP_TAG=%s
sqlplus / as sysdba <<EOF
SHUTDOWN IMMEDIATE;
STARTUP MOUNT;
EXIT;
EOF
rman target / <<EOF
RUN {
  RESTORE DATABASE FROM TAG='${BACKUP_TAG}';
  RECOVER DATABASE;
}
EXIT;
EOF
sqlplus / as sysdba <<EOF
RECOVER DATABASE UNTIL CANCEL USING BACKUP CONTROLFILE;
CANCEL
ALTER DATABASE OPEN RESETLOGS;
EOF`
)
