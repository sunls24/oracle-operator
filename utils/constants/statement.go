package constants

import "fmt"

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
  RECOVER DATABASE FROM TAG='${BACKUP_TAG}';
}
EXIT;
EOF
sqlplus / as sysdba <<EOF
RECOVER DATABASE UNTIL CANCEL USING BACKUP CONTROLFILE;
CANCEL
ALTER DATABASE OPEN RESETLOGS;
EOF`
)

const (
	setup01Name = "01_createTablespace_19c.sql"
	setup01     = `-- 01 create tablespace
alter session set container=CBPDB;
create tablespace %s datafile '%s/%s/%s/%s_01.dbf' size 2048m autoextend off;`  // TODO: PDB, SIZE

	setup02Name = "02_createProfile_19c.sql"
	setup02     = `-- 02 create profile
alter session set container=CBPDB;
create profile ZSMART limit
 sessions_per_user unlimited
 cpu_per_session unlimited
 cpu_per_call unlimited
 connect_time unlimited
 idle_time unlimited
 logical_reads_per_session unlimited
 logical_reads_per_call unlimited
 composite_limit unlimited
 private_sga unlimited
 failed_login_attempts unlimited
 password_life_time unlimited
 password_reuse_time unlimited
 password_reuse_max unlimited
 password_lock_time 1/24
 password_grace_time 7
 password_verify_function ORA12C_VERIFY_FUNCTION;`

	setup03Name = "03_createUser_19c.sql"
	setup03     = `-- 03 create user
alter session set container=CBPDB;
create user %s identified by "%s" default tablespace %s profile ZSMART;`

	setup04Name = "04_grantPrivilegeToUser_19c.sql"
	setup04     = `-- 04 grant privilege to user
alter session set container=CBPDB;
grant SET CONTAINER to CC;
grant CREATE SESSION to CC;
grant CREATE TRIGGER to CC;
grant CREATE SEQUENCE to CC;
grant CREATE TYPE to CC;
grant CREATE PROCEDURE to CC;
grant CREATE CLUSTER to CC;
grant CREATE OPERATOR to CC;
grant CREATE INDEXTYPE to CC;
grant CREATE TABLE to CC;
grant CREATE SYNONYM to CC;
grant CREATE VIEW to CC;
grant CREATE MATERIALIZED VIEW to CC;
grant CREATE DATABASE LINK to CC;
grant ALTER SESSION to CC;
grant DEBUG ANY PROCEDURE to CC;
grant DEBUG CONNECT SESSION to CC;
grant SELECT ANY TABLE to CC;
grant SELECT ANY TRANSACTION to CC;
grant CREATE JOB to CC;
grant EXP_FULL_DATABASE to CC;
grant IMP_FULL_DATABASE to CC;
grant UNLIMITED TABLESPACE to CC;`
)

func GetSetup(sid, pdb, tablespace, user, password string) map[string]string {
	return map[string]string{
		setup01Name: fmt.Sprintf(setup01, tablespace, OracleMountPath, sid, pdb, tablespace),
		setup02Name: setup02,
		setup03Name: fmt.Sprintf(setup03, user, password, tablespace),
		setup04Name: setup04,
	}
}
