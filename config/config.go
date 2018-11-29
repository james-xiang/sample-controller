package config

import (
	// TODO: pflag won't work glog, need to remove glog in order to use pflag
	//flag "github.com/spf13/pflag"
	"flag"
)

const (
	defaultNamespace = "default"
	defaultTimeout   = 30
	defaultQPS       = 100
	defaultBurst     = 200
)

// CmdConfig defines args for this application
type CmdConfig struct {
	Kubeconfig string
	MasterURL  string
	Namespace  string
	APIQPS     float32
	APIBurst   int
}

var globalConfig *CmdConfig

// func init() {
// 	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
// 	flag.StringVar(&masterURL, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
// }

var (
	kubeconfig   = flag.String("kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	masterURL    = flag.String("master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
	k8sNamespace = flag.String("namespace", defaultNamespace, "The name of current namespace")
	k8sAPIQPS    = flag.Float64("qps", defaultQPS, "QPS of k8s API access")
	k8sAPIBurst  = flag.Int("burst", defaultBurst, "the Burst rate of k8s API access")
)

// LoadConfig parses command line args
func loadConfig() *CmdConfig {
	flag.Parse()

	cfg := &CmdConfig{}

	cfg.Kubeconfig = *kubeconfig
	cfg.MasterURL = *masterURL
	cfg.Namespace = *k8sNamespace
	cfg.APIQPS = float32(*k8sAPIQPS)
	cfg.APIBurst = *k8sAPIBurst

	return cfg
}

// GetConfig returns config singleton
func GetConfig() *CmdConfig {
	if globalConfig == nil {
		globalConfig = loadConfig()
	}
	return globalConfig
}
