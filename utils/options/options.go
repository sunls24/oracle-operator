package options

import (
	"flag"
	"oracle-operator/utils/constants"
)

type Options struct {
	CLIImage      string
	ExporterImage string
	Namespace     string

	//osbws_install.jar 的安装命令模版
	OSBWSInstallCmd string
	BackupCmd       string
	BackupDeleteCmd string
	RestoreCmd      string
}

func (o *Options) AddFlags() {
	flag.StringVar(&o.CLIImage, "cli-image", constants.DefaultCLIImage, "Oracle cli image address")
	flag.StringVar(&o.ExporterImage, "exporter-image", constants.DefaultExporterImage, "Oracle exporter image address")
	flag.StringVar(&o.Namespace, "namespace", "", "Namespace used for LeaderElection and Watch")

	flag.StringVar(&o.OSBWSInstallCmd, "osbws-install-cmd", constants.DefaultOSBWSInstallCmd, "osbws_install.jar command install template")
	flag.StringVar(&o.BackupCmd, "backup-cmd", constants.DefaultBackupCmd, "backup command template")
	flag.StringVar(&o.BackupDeleteCmd, "backup-delete-cmd", constants.DefaultBackupDeleteCmd, "backup delete command template")
	flag.StringVar(&o.RestoreCmd, "restore-cmd", constants.DefaultRestoreCmd, "restore backup command template")
}

var instance *Options

func init() {
	instance = &Options{
		CLIImage:      constants.DefaultCLIImage,
		ExporterImage: constants.DefaultExporterImage,
	}
}

func GetOptions() *Options {
	return instance
}
