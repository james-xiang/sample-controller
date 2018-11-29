package main

import (
	"time"

	"github.com/golang/glog"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"k8s.io/sample-controller/common/signals"
	"k8s.io/sample-controller/common/utils"
	config "k8s.io/sample-controller/config"
	ctrl "k8s.io/sample-controller/controller"
)

func main() {
	// flag.Parse()
	cmdConf := config.GetConfig()
	glog.Infof("Cmd args: %s", utils.PrettyJSON(cmdConf))

	// set up signals so we handle the first shutdown signal gracefully
	stopCh := signals.SetupSignalHandler()

	cfg, err := clientcmd.BuildConfigFromFlags(cmdConf.MasterURL, cmdConf.Kubeconfig)
	if err != nil {
		glog.Fatalf("Error building kubeconfig: %s", err.Error())
	}

	cfg.QPS = cmdConf.APIQPS
	cfg.Burst = cmdConf.APIBurst

	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		glog.Fatalf("Error building kubernetes clientset: %s", err.Error())
	}

	// kubeInformerFactory := kubeinformers.NewSharedInformerFactory(kubeClient, time.Second*30)
	kubeInformerFactory := kubeinformers.NewSharedInformerFactoryWithOptions(
		kubeClient,
		time.Second*30,
		kubeinformers.WithNamespace("default"))
	// exampleInformerFactory := informers.NewSharedInformerFactory(exampleClient, time.Second*60)

	controller := ctrl.NewController(
		kubeClient,
		kubeInformerFactory.Core().V1().Services(),
		kubeInformerFactory.Core().V1().Endpoints())

	go kubeInformerFactory.Start(stopCh)
	if err = controller.Run(1, stopCh); err != nil {
		glog.Fatalf("Error running controller: %s", err.Error())
	}
}
