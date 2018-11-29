/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"fmt"
	"strings"
	"time"

	"github.com/golang/glog"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"

	"k8s.io/sample-controller/common/utils"
)

const controllerAgentName = "sample-controller"

const (
	// SuccessSynced is used as part of the Event 'reason' when a Foo is synced
	SuccessSynced = "Synced"
	// ErrResourceExists is used as part of the Event 'reason' when a Foo fails
	// to sync due to a Deployment of the same name already existing.
	ErrResourceExists = "ErrResourceExists"

	// MessageResourceExists is the message used for Events when a resource
	// fails to sync due to a Deployment already existing
	MessageResourceExists = "Resource %q already exists and is not managed by Foo"
	// MessageResourceSynced is the message used for an Event fired when a Foo
	// is synced successfully
	MessageResourceSynced = "Foo synced successfully"
)

// Controller is the controller implementation for Foo resources
type Controller struct {
	// kubeclientset is a standard kubernetes clientset
	kubeclientset kubernetes.Interface

	// service lister
	serviceLister corelisters.ServiceLister
	serviceSynced cache.InformerSynced

	// endpoints lister
	endpointsLister corelisters.EndpointsLister
	endpointsSynced cache.InformerSynced

	// workqueue is a rate limited work queue. This is used to queue work to be
	// processed instead of performing it as soon as a change happens. This
	// means we can ensure we only process a fixed amount of resources at a
	// time, and makes it easy to ensure we are never processing the same item
	// simultaneously in two different workers.
	workqueue workqueue.RateLimitingInterface
	// recorder is an event recorder for recording Event resources to the
	// Kubernetes API.
	recorder record.EventRecorder
}

// NewController returns a new sample controller
func NewController(
	kubeclientset kubernetes.Interface,
	serviceInformer coreinformers.ServiceInformer,
	endpointsInformer coreinformers.EndpointsInformer) *Controller {

	// Create event broadcaster
	// Add sample-controller types to the default Kubernetes Scheme so Events can be
	// logged for sample-controller types.
	// samplescheme.AddToScheme(scheme.Scheme)
	glog.V(4).Info("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(glog.Infof)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeclientset.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})

	controller := &Controller{
		kubeclientset:   kubeclientset,
		serviceLister:   serviceInformer.Lister(),
		serviceSynced:   serviceInformer.Informer().HasSynced,
		endpointsLister: endpointsInformer.Lister(),
		endpointsSynced: endpointsInformer.Informer().HasSynced,
		workqueue:       workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "SharedQueue"),
		recorder:        recorder,
	}

	glog.Info("Setting up event handlers")

	// Set up event handler for Service resource change.
	serviceInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueueService,
		UpdateFunc: func(old, new interface{}) {
			// simply filters for changes in resource version before continuing
			// in general, resource versions change on update
			newService := new.(*corev1.Service)
			oldService := old.(*corev1.Service)

			if newService.ResourceVersion != oldService.ResourceVersion {
				controller.enqueueService(new)
			}
		},
	})

	// Set up event handler for Endpoints resource change.
	endpointsInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueueEndpoints,
		UpdateFunc: func(old, new interface{}) {
			// simply filters for changes in resource version before continuing
			// in general, resource versions change on update
			newEndpoints := new.(*corev1.Endpoints)
			oldEndpoints := old.(*corev1.Endpoints)

			if newEndpoints.ResourceVersion != oldEndpoints.ResourceVersion {
				controller.enqueueEndpoints(new)
			}
		},
	})

	return controller
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (c *Controller) Run(threadiness int, stopCh <-chan struct{}) error {
	defer runtime.HandleCrash()
	defer c.workqueue.ShutDown()

	// Start the informer factories to begin populating the informer caches
	glog.Info("=== Starting Foo controller")

	// Wait for the caches to be synced before starting workers
	glog.Info("=== Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.serviceSynced, c.endpointsSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	glog.Info("=== Starting workers")
	// Launch two workers to process Foo resources
	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	glog.Info("=== Started workers")
	<-stopCh
	glog.Info("=== Shutting down workers")

	return nil
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workqueue.
func (c *Controller) runWorker() {
	for c.processNextWorkItem() {
	}
}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it, by calling the syncHandler.
func (c *Controller) processNextWorkItem() bool {
	obj, shutdown := c.workqueue.Get()

	if shutdown {
		return false
	}

	// We wrap this block in a func so we can defer c.workqueue.Done.
	err := func(obj interface{}) error {
		// We call Done here so the workqueue knows we have finished
		// processing this item. We also must remember to call Forget if we
		// do not want this work item being re-queued. For example, we do
		// not call Forget if a transient error occurs, instead the item is
		// put back on the workqueue and attempted again after a back-off
		// period.
		defer c.workqueue.Done(obj)
		var key string
		var ok bool
		// We expect strings to come off the workqueue. These are of the
		// form namespace/name. We do this as the delayed nature of the
		// workqueue means the items in the informer cache may actually be
		// more up to date that when the item was initially put onto the
		// workqueue.
		if key, ok = obj.(string); !ok {
			// As the item in the workqueue is actually invalid, we call
			// Forget here else we'd go into a loop of attempting to
			// process a work item that is invalid.
			c.workqueue.Forget(obj)
			runtime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}
		// Run the syncHandler, passing it the namespace/name string of the
		// Foo resource to be synced.
		if err := c.syncHandler(key); err != nil {
			return fmt.Errorf("error syncing '%s': %s", key, err.Error())
		}
		// Finally, if no error occurs we Forget this item so it does not
		// get queued again until another change happens.
		c.workqueue.Forget(obj)
		glog.Infof("=== Successfully synced '%s'", key)

		return nil
	}(obj)

	if err != nil {
		runtime.HandleError(err)
		return true
	}

	return true
}

// syncHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the Foo resource
// with the current status of the resource.
func (c *Controller) syncHandler(key string) error {
	glog.Infof("=== Sync/Handle for key: %s", key)
	parts := strings.Split(key, "/")

	kind := parts[0]
	namespace := parts[1]
	name := parts[2]

	switch kind {
	case "Service":
		glog.Infof("### Sync: kind=Service, namespace=%s, name=%s", namespace, name)
		return c.syncService(namespace, name)
	case "Endpoints":
		glog.Infof("### Sync: kind=Endpoints, namespace=%s, name=%s", namespace, name)
		return c.syncEndpoints(namespace, name)
	}

	//c.recorder.Event(foo, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	return nil
}

func (c *Controller) enqueueService(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		runtime.HandleError(err)
		return
	}
	keyWithKind := fmt.Sprintf("Service/%s", key)
	c.workqueue.AddRateLimited(keyWithKind)
}

func (c *Controller) enqueueEndpoints(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		runtime.HandleError(err)
		return
	}
	keyWithKind := fmt.Sprintf("Endpoints/%s", key)
	c.workqueue.AddRateLimited(keyWithKind)
}

func (c *Controller) syncService(namespace, name string) error {
	// Get the Service resource with this namespace/name

	svc, err := c.serviceLister.Services(namespace).Get(name)
	if err != nil {
		// The Service resource may no longer exist, in which case we stop
		// processing.
		if errors.IsNotFound(err) {
			runtime.HandleError(fmt.Errorf("service: '%s/%s' in work queue no longer exists", namespace, name))
			return nil
		}

		return err
	}
	glog.Infof("=== Handle resource: Namespace: %s, name: %s, \nService: %+v", namespace, name, utils.PrettyJSON(svc))
	return nil
}

func (c *Controller) syncEndpoints(namespace, name string) error {
	// Get the Endpoints resource with this namespace/name

	endpoints, err := c.endpointsLister.Endpoints(namespace).Get(name)
	if err != nil {
		// The Endpoints resource may no longer exist, in which case we stop
		// processing.
		if errors.IsNotFound(err) {
			runtime.HandleError(fmt.Errorf("Endpoints: '%s/%s' in work queue no longer exists", namespace, name))
			return nil
		}

		return err
	}
	glog.Infof("=== Handle resource: Namespace: %s, name: %s, \nEndpoints: %+v", namespace, name, utils.PrettyJSON(endpoints))
	return nil
}
