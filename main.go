package main

import (
	"flag"
	"time"

	"github.com/golang/glog"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"k8s.io/sample-controller/pkg/signals"
)

var (
	masterURL  string
	kubeconfig string
)

func main() {
	flag.Parse()

	// set up signals so we handle the first shutdown signal gracefully
	stopCh := signals.SetupSignalHandler()

	cfg, err := clientcmd.BuildConfigFromFlags(masterURL, kubeconfig)
	if err != nil {
		glog.Fatalf("Error building kubeconfig: %s", err.Error())
	}

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

	controller := NewController(
		kubeClient,
		kubeInformerFactory.Core().V1().Services(),
		kubeInformerFactory.Core().V1().Endpoints())

	go kubeInformerFactory.Start(stopCh)
	if err = controller.Run(1, stopCh); err != nil {
		glog.Fatalf("Error running controller: %s", err.Error())
	}
}

func init() {
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&masterURL, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
}
