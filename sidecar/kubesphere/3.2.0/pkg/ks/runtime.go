package ks

import (
	"context"
	"fmt"

	"k8s.io/klog"

	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/authentication/user"
	iamv1alpha2 "kubesphere.io/api/iam/v1alpha2"
	tenantv1alpha1 "kubesphere.io/api/tenant/v1alpha1"
	tenantv1alpha2 "kubesphere.io/api/tenant/v1alpha2"
	"kubesphere.io/kubesphere/pkg/apis"
	"kubesphere.io/kubesphere/pkg/apiserver/authorization/authorizer"
	"kubesphere.io/kubesphere/pkg/apiserver/authorization/rbac"
	"kubesphere.io/kubesphere/pkg/apiserver/query"
	"kubesphere.io/kubesphere/pkg/apiserver/request"
	"kubesphere.io/kubesphere/pkg/client/clientset/versioned/scheme"
	"kubesphere.io/kubesphere/pkg/informers"
	"kubesphere.io/kubesphere/pkg/models/iam/am"
	"kubesphere.io/kubesphere/pkg/models/resources/v1alpha3/resource"
	"kubesphere.io/kubesphere/pkg/simple/client/k8s"
	"kubesphere.io/kubesphere/pkg/utils/sliceutil"
	runtimecache "sigs.k8s.io/controller-runtime/pkg/cache"
)

type Runtime struct {
	k8s.Client
	informers.InformerFactory
	*rbac.RBACAuthorizer
	*resource.ResourceGetter
	stopCh <-chan struct{}
}

func NewRuntime(kubeConfig string, stopCh <-chan struct{}) (*Runtime, error) {

	kubernetesOptions := k8s.NewKubernetesOptions()
	kubernetesOptions.KubeConfig = kubeConfig

	kubernetesClient, err := k8s.NewKubernetesClient(kubernetesOptions)
	if err != nil {
		klog.Errorf("create kubernetes client error, %s", err.Error())
		return nil, err
	}

	informerFactory := informers.NewInformerFactories(kubernetesClient.Kubernetes(), kubernetesClient.KubeSphere(),
		nil, nil, nil, nil)

	r := &Runtime{
		kubernetesClient,
		informerFactory,
		rbac.NewRBACAuthorizer(am.NewOperator(kubernetesClient.KubeSphere(), kubernetesClient.Kubernetes(), informerFactory, nil)),
		resource.NewResourceGetter(informerFactory, nil),
		stopCh,
	}

	err = r.waitForResourceSync()
	if err != nil {
		klog.Errorf("wait for resource sync error, %s", err.Error())
		return nil, err
	}

	return r, nil
}

func (r *Runtime) waitForResourceSync() error {

	discoveryClient := r.Kubernetes().Discovery()
	_, apiResourcesList, err := discoveryClient.ServerGroupsAndResources()
	if err != nil {
		return err
	}

	isResourceExists := func(resource schema.GroupVersionResource) bool {
		for _, apiResource := range apiResourcesList {
			if apiResource.GroupVersion == resource.GroupVersion().String() {
				for _, rsc := range apiResource.APIResources {
					if rsc.Name == resource.Resource {
						return true
					}
				}
			}
		}
		return false
	}

	// resources we have to create informer first
	k8sGVRs := []schema.GroupVersionResource{
		{Group: "", Version: "v1", Resource: "namespaces"},
		{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "roles"},
		{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "rolebindings"},
		{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "clusterroles"},
		{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "clusterrolebindings"},
	}

	for _, gvr := range k8sGVRs {
		if isResourceExists(gvr) {
			_, err := r.KubernetesSharedInformerFactory().ForResource(gvr)
			if err != nil {
				return err
			}
		}
	}

	r.KubernetesSharedInformerFactory().Start(r.stopCh)
	r.KubernetesSharedInformerFactory().WaitForCacheSync(r.stopCh)

	ksInformerFactory := r.KubeSphereSharedInformerFactory()

	ksGVRs := []schema.GroupVersionResource{
		{Group: "tenant.kubesphere.io", Version: "v1alpha1", Resource: "workspaces"},
		{Group: "tenant.kubesphere.io", Version: "v1alpha2", Resource: "workspacetemplates"},
		{Group: "iam.kubesphere.io", Version: "v1alpha2", Resource: "users"},
		{Group: "iam.kubesphere.io", Version: "v1alpha2", Resource: "globalroles"},
		{Group: "iam.kubesphere.io", Version: "v1alpha2", Resource: "globalrolebindings"},
		{Group: "iam.kubesphere.io", Version: "v1alpha2", Resource: "groups"},
		{Group: "iam.kubesphere.io", Version: "v1alpha2", Resource: "groupbindings"},
		{Group: "iam.kubesphere.io", Version: "v1alpha2", Resource: "workspaceroles"},
		{Group: "iam.kubesphere.io", Version: "v1alpha2", Resource: "workspacerolebindings"},
	}

	for _, gvr := range ksGVRs {
		if isResourceExists(gvr) {
			_, err = ksInformerFactory.ForResource(gvr)
			if err != nil {
				return err
			}
		}
	}

	ksInformerFactory.Start(r.stopCh)
	ksInformerFactory.WaitForCacheSync(r.stopCh)

	sch := scheme.Scheme
	if err := apis.AddToScheme(sch); err != nil {
		return err
	}

	runtimeCache, err := runtimecache.New(r.Config(), runtimecache.Options{Scheme: sch})
	if err != nil {
		klog.Errorf("create runttime cache error, %s", err.Error())
		return err
	}

	// controller runtime cache for resources
	go func() {
		_ = runtimeCache.Start(context.Background())
	}()
	runtimeCache.WaitForCacheSync(context.Background())

	return nil
}

func (r *Runtime) ListUser() ([]*iamv1alpha2.User, error) {

	res, err := r.List("users", "", query.New())
	if err != nil {
		return nil, err
	}

	users := make([]*iamv1alpha2.User, 0)
	for _, tmp := range res.Items {
		users = append(users, tmp.(*iamv1alpha2.User))
	}

	return users, nil
}

func (r *Runtime) ListWorkspaces(u *iamv1alpha2.User) ([]string, error) {

	listWS := authorizer.AttributesRecord{
		User: &user.DefaultInfo{
			Name:   u.Name,
			Groups: u.Spec.Groups,
		},
		Verb:            "list",
		APIGroup:        "*",
		Resource:        "workspaces",
		ResourceRequest: true,
		ResourceScope:   request.GlobalScope,
	}

	decision, _, err := r.Authorize(listWS)
	if err != nil {
		klog.Errorf("authorize error, %s", err.Error())
		return nil, err
	}

	workspaces := make([]string, 0)

	// allowed to list all workspaces
	if decision == authorizer.DecisionAllow {
		result, err := r.List(tenantv1alpha2.ResourcePluralWorkspaceTemplate, "", query.New())
		if err != nil {
			klog.Errorf("list workspace template error, %s", err.Error())
			return nil, err
		}

		for _, tmp := range result.Items {
			workspaces = append(workspaces, tmp.(*tenantv1alpha2.WorkspaceTemplate).Name)
		}

		return workspaces, nil
	}

	// retrieving associated resources through role binding
	workspaceRoleBindings, err := r.ListWorkspaceRoleBindings(u.Name, u.Spec.Groups, "")
	if err != nil {
		klog.Errorf("list workspace rolebinding error, %s", err.Error())
		return nil, err
	}

	for _, roleBinding := range workspaceRoleBindings {
		workspaceName := roleBinding.Labels[tenantv1alpha1.WorkspaceLabel]
		obj, err := r.Get(tenantv1alpha2.ResourcePluralWorkspaceTemplate, "", workspaceName)
		if errors.IsNotFound(err) {
			klog.V(4).Infof("workspace role binding: %+v found but workspace not exist", roleBinding.Name)
			continue
		}
		if err != nil {
			klog.Errorf("get workspace rolebinding '%s' error, %s", roleBinding.Name, err.Error())
			return nil, err
		}

		workspaces = append(workspaces, obj.(*tenantv1alpha2.WorkspaceTemplate).Name)
	}

	return workspaces, nil
}

func (r *Runtime) ListNamespaces(u *iamv1alpha2.User, workspace string) ([]string, error) {
	nsScope := request.ClusterScope
	queryParam := query.New()
	if workspace != "" {
		nsScope = request.WorkspaceScope
		// filter by workspace
		queryParam.Filters[query.FieldLabel] = query.Value(fmt.Sprintf("%s=%s", tenantv1alpha1.WorkspaceLabel, workspace))
	}

	listNS := authorizer.AttributesRecord{
		User: &user.DefaultInfo{
			Name:   u.Name,
			Groups: u.Spec.Groups,
		},
		Verb:            "list",
		Workspace:       workspace,
		Resource:        "namespaces",
		ResourceRequest: true,
		ResourceScope:   nsScope,
	}

	decision, _, err := r.Authorize(listNS)
	if err != nil {
		klog.Errorf("authorize error, %s", err.Error())
		return nil, err
	}

	namespaces := make([]string, 0)

	// allowed to list all namespaces in the specified scope
	if decision == authorizer.DecisionAllow {
		result, err := r.List("namespaces", "", queryParam)
		if err != nil {
			klog.Errorf("list namespace error, %s", err.Error())
			return nil, err
		}

		for _, tmp := range result.Items {
			namespaces = append(namespaces, tmp.(*v1.Namespace).Name)
		}

		return namespaces, nil
	}

	// retrieving associated resources through role binding
	roleBindings, err := r.ListRoleBindings(u.Name, u.Spec.Groups, "")
	if err != nil {
		klog.Errorf("list rolebinding error, %s", err.Error())
		return nil, err
	}

	for _, roleBinding := range roleBindings {
		obj, err := r.Get("namespaces", "", roleBinding.Namespace)
		if err != nil {
			klog.Errorf("get rolebinding '%s' error, %s", roleBinding.Name, err.Error())
			return nil, err
		}
		namespace := obj.(*v1.Namespace)
		// label matching selector, remove duplicate entity
		if queryParam.Selector().Matches(labels.Set(namespace.Labels)) {
			namespaces = append(namespaces, obj.(*v1.Namespace).Name)
		}
	}

	return namespaces, nil
}

func (r *Runtime) ListWorkspaceRoleBindings(username string, groups []string, workspace string) ([]*iamv1alpha2.WorkspaceRoleBinding, error) {

	roleBindings, err := r.List(iamv1alpha2.ResourcesPluralWorkspaceRoleBinding, "", query.New())
	if err != nil {
		return nil, err
	}

	result := make([]*iamv1alpha2.WorkspaceRoleBinding, 0)

	for _, obj := range roleBindings.Items {
		roleBinding := obj.(*iamv1alpha2.WorkspaceRoleBinding)
		inSpecifiedWorkspace := workspace == "" || roleBinding.Labels[tenantv1alpha1.WorkspaceLabel] == workspace
		if contains(roleBinding.Subjects, username, groups) && inSpecifiedWorkspace {
			result = append(result, roleBinding)
		}
	}

	return result, nil
}

func (r *Runtime) ListRoleBindings(username string, groups []string, namespace string) ([]*rbacv1.RoleBinding, error) {

	roleBindings, err := r.List(iamv1alpha2.ResourcesPluralRoleBinding, namespace, query.New())
	if err != nil {
		return nil, err
	}

	result := make([]*rbacv1.RoleBinding, 0)
	for _, obj := range roleBindings.Items {
		roleBinding := obj.(*rbacv1.RoleBinding)
		if contains(roleBinding.Subjects, username, groups) {
			result = append(result, roleBinding)
		}
	}
	return result, nil
}

func contains(subjects []rbacv1.Subject, username string, groups []string) bool {
	// if username is nil means list all role bindings
	if username == "" {
		return true
	}
	for _, subject := range subjects {
		if subject.Kind == rbacv1.UserKind && subject.Name == username {
			return true
		}
		if subject.Kind == rbacv1.GroupKind && sliceutil.HasString(groups, subject.Name) {
			return true
		}
	}
	return false
}
