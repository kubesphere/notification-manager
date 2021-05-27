package controller

import (
	"kubesphere/pkg/ks"
	"kubesphere/pkg/tenant"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	toolscache "k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"
	runtimecache "sigs.k8s.io/controller-runtime/pkg/cache"
)

type Controller struct {
	*ks.Runtime
	informers      []runtimecache.Informer
	informerSynced []toolscache.InformerSynced
	// workqueue is a rate limited work queue. This is used to queue work to be
	// processed instead of performing it as soon as a change happens. This
	// means we can ensure we only process a fixed amount of resources at a
	// time, and makes it easy to ensure we are never processing the same item
	// simultaneously in two different workers.
	workqueue workqueue.RateLimitingInterface
}

func NewController(r *ks.Runtime) *Controller {

	c := &Controller{
		Runtime:   r,
		workqueue: workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
	}

	c.informers = make([]runtimecache.Informer, 0)
	c.informerSynced = make([]toolscache.InformerSynced, 0)

	c.informers = append(c.informers, r.InformerFactory.KubernetesSharedInformerFactory().Core().V1().Namespaces().Informer())

	c.informers = append(c.informers, r.InformerFactory.KubernetesSharedInformerFactory().Rbac().V1().Roles().Informer())
	c.informers = append(c.informers, r.InformerFactory.KubernetesSharedInformerFactory().Rbac().V1().RoleBindings().Informer())
	c.informers = append(c.informers, r.InformerFactory.KubernetesSharedInformerFactory().Rbac().V1().ClusterRoles().Informer())
	c.informers = append(c.informers, r.InformerFactory.KubernetesSharedInformerFactory().Rbac().V1().ClusterRoleBindings().Informer())

	c.informers = append(c.informers, r.InformerFactory.KubeSphereSharedInformerFactory().Iam().V1alpha2().Users().Informer())
	c.informers = append(c.informers, r.InformerFactory.KubeSphereSharedInformerFactory().Iam().V1alpha2().GlobalRoles().Informer())
	c.informers = append(c.informers, r.InformerFactory.KubeSphereSharedInformerFactory().Iam().V1alpha2().GlobalRoleBindings().Informer())
	c.informers = append(c.informers, r.InformerFactory.KubeSphereSharedInformerFactory().Iam().V1alpha2().Groups().Informer())
	c.informers = append(c.informers, r.InformerFactory.KubeSphereSharedInformerFactory().Iam().V1alpha2().GroupBindings().Informer())
	c.informers = append(c.informers, r.InformerFactory.KubeSphereSharedInformerFactory().Iam().V1alpha2().WorkspaceRoles().Informer())
	c.informers = append(c.informers, r.InformerFactory.KubeSphereSharedInformerFactory().Iam().V1alpha2().WorkspaceRoleBindings().Informer())

	for _, informer := range c.informers {
		informer.AddEventHandler(toolscache.ResourceEventHandlerFuncs{
			AddFunc: c.enqueue,
			UpdateFunc: func(old, new interface{}) {
				c.enqueue(new)
			},
			DeleteFunc: c.enqueue,
		})
		c.informerSynced = append(c.informerSynced, informer.HasSynced)
	}

	return c
}

func (c *Controller) Run(stopCh <-chan struct{}) {

	defer c.workqueue.ShutDown()

	// Start the informer factories to begin populating the informer caches
	klog.Info("Starting controller")

	// Wait for the caches to be synced before starting workers
	klog.Info("Waiting for informer caches to sync")

	if ok := toolscache.WaitForCacheSync(stopCh, c.informerSynced...); !ok {
		klog.Fatal("failed to wait for caches to sync")
	}

	klog.Info("Starting workers")
	// Launch two workers to process Foo resources
	go wait.Until(c.runWorker, time.Second, stopCh)

	klog.Info("Started workers")
	<-stopCh
	klog.Info("Shutting down workers")
}

func (c *Controller) enqueue(obj interface{}) {
	c.workqueue.Add(obj)
}

func (c *Controller) runWorker() {
	for c.processNextWorkItem() {
	}
}

func (c *Controller) processNextWorkItem() bool {
	obj, shutdown := c.workqueue.Get()

	if shutdown {
		return false
	}

	// We wrap this block in a func so we can defer c.workqueue.Done.
	_ = func(obj interface{}) error {
		// We call Done here so the workqueue knows we have finished
		// processing this item. We also must remember to call Forget if we
		// do not want this work item being re-queued. For example, we do
		// not call Forget if a transient error occurs, instead the item is
		// put back on the workqueue and attempted again after a back-off
		// period.
		defer c.workqueue.Done(obj)

		// Run the reconcile, passing it the namespace/name string of the
		// Foo resource to be synced.
		if err := c.reconcile(obj); err != nil {
			// Put the item back on the workqueue to handle any transient errors.
			c.workqueue.AddRateLimited(obj)
		}
		// Finally, if no error occurs we Forget this item so it does not
		// get queued again until another change happens.
		c.workqueue.Forget(obj)
		return nil
	}(obj)

	return true
}

func (c *Controller) reconcile(_ interface{}) error {

	if err := tenant.Reload(c.Runtime); err != nil {
		klog.Errorf("reload tenant error, %s", err)
		return err
	}

	return nil
}
