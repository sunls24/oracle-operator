package constants

import (
	"fmt"
	v1 "oracle-operator/api/v1"
	"strings"
)

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
	setup02Name = "02_createProfile_19c.sql"
	setup02     = `-- 02 create profile
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

	grantPrivilege = `grant SET CONTAINER to $USER;
grant CREATE SESSION to $USER;
grant CREATE TRIGGER to $USER;
grant CREATE SEQUENCE to $USER;
grant CREATE TYPE to $USER
grant CREATE PROCEDURE to $USER;
grant CREATE CLUSTER to $USER;
grant CREATE OPERATOR to $USER;
grant CREATE INDEXTYPE to $USER;
grant CREATE TABLE to $USER;
grant CREATE SYNONYM to $USER;
grant CREATE VIEW to $USER;
grant CREATE MATERIALIZED VIEW to $USER;
grant CREATE DATABASE LINK to $USER;
grant ALTER SESSION to $USER;
grant DEBUG ANY PROCEDURE to $USER;
grant DEBUG CONNECT SESSION to $USER;
grant SELECT ANY TABLE to $USER;
grant SELECT ANY TRANSACTION to $USER;
grant CREATE JOB to $USER;
grant EXP_FULL_DATABASE to $USER;
grant IMP_FULL_DATABASE to $USER;
grant UNLIMITED TABLESPACE to $USER;`
)

func GetSetupSQL(sid string, tablespaceList []v1.Tablespace, userList []v1.User) map[string]string {
	sid = strings.ToUpper(sid)
	setup01Name, setup01 := createTablespace01(tablespaceList, sid)
	setup03Name, setup03 := createUser03(userList)
	setup04Name, setup04 := grantPrivilege04(userList)
	return map[string]string{
		setup01Name: setup01,
		setup02Name: setup02,
		setup03Name: setup03,
		setup04Name: setup04,
	}
}

func createTablespace01(list []v1.Tablespace, sid string) (string, string) {
	sqlList := make([]string, len(list))
	for _, v := range list {
		sqlList = append(sqlList,
			fmt.Sprintf("create tablespace %s datafile '%s/%s/%s_01.dbf' size %dm autoextend off;", v.Name, OracleMountPath, sid, v.Name, v.Size))
	}
	return "01_createTablespace_19c.sql", fmt.Sprintf("-- 01 create tablespace\n%s", strings.Join(sqlList, "\n"))
}

func createUser03(list []v1.User) (string, string) {
	sqlList := make([]string, len(list))
	for _, u := range list {
		sqlList = append(sqlList,
			fmt.Sprintf(`create user %s identified by "%s" default tablespace %s profile ZSMART;`, u.Name, u.Password, u.Tablespace))
	}

	return "03_createUser_19c.sql", fmt.Sprintf("-- 03 create user\n%s", strings.Join(sqlList, "\n"))
}

func grantPrivilege04(list []v1.User) (string, string) {
	sqlList := make([]string, len(list))
	for _, u := range list {
		sqlList = append(sqlList, strings.ReplaceAll(grantPrivilege, "$USER", u.Name))
	}
	return "04_grantPrivilegeToUser_19c.sql", fmt.Sprintf("-- 04 grant privilege to user\n%s", strings.Join(sqlList, "\n"))
}
