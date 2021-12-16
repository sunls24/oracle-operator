package options

import (
	"flag"
	"oracle-operator/utils/constants"
)

type Options struct {
	CLIImage      string
	ExporterImage string
	Namespace     string
}

func (o *Options) AddFlags() {
	flag.StringVar(&o.CLIImage, "cli-image", constants.DefaultCLIImage, "Oracle cli image address")
	flag.StringVar(&o.ExporterImage, "exporter-image", constants.DefaultExporterImage, "Oracle exporter image address")
	flag.StringVar(&o.Namespace, "namespace", "", "Namespace used for LeaderElection and Watch")
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
