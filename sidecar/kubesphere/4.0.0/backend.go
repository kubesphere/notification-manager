package main

import (
	"context"
	"fmt"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/klog"
	iamv1beta1 "kubesphere.io/api/iam/v1beta1"
	"kubesphere.io/client-go/rest"
)

type Backend struct {
	ksClient *rest.RESTClient
	// map[cluster]map[namespace][]{users}
	tenants map[string]map[string][]string

	interval  time.Duration
	batchSize int
}

func NewBackend(host, username, password string, interval time.Duration, batchSize int) (*Backend, error) {
	var config *rest.Config
	if username != "" && password != "" {
		config = &rest.Config{
			Host:     host,
			Username: username,
			Password: password,
		}
	} else {
		config = &rest.Config{
			Host:            host,
			BearerTokenFile: "/var/run/secrets/kubesphere.io/serviceaccount/token",
		}
	}

	c, err := rest.UnversionedRESTClientFor(config)
	if err != nil {
		return nil, err
	}

	return &Backend{
		ksClient:  c,
		tenants:   make(map[string]map[string][]string),
		interval:  interval,
		batchSize: batchSize,
	}, err
}

func (b *Backend) FromNamespace(cluster, ns string) []string {
	cm, ok := b.tenants[cluster]
	if !ok || cm == nil {
		return nil
	}

	return cm[ns]
}

func (b *Backend) Run() {
	b.reload()

	ticker := time.NewTicker(b.interval)
	go func() {
		for {
			select {
			case <-ticker.C:
				b.reload()
			}
		}
	}()
}

func (b *Backend) reload() {
	klog.Info("start reload tenant")
	defer func() {
		klog.Info("end reload tenant")
	}()

	clusters, err := b.listClusters()
	if err != nil {
		klog.Errorf("list clusters error, %s", err.Error())
	}

	users, err := b.listUsers()
	if err != nil {
		klog.Errorf("list users error, %s", err.Error())
		return
	}

	tenants := make(map[string]map[string][]string)
	for _, cluster := range clusters {
		m, err := getTenantInfoFromCluster(cluster, users)
		if err != nil {
			klog.Errorf("get tenant info from %s error, %s", cluster, err.Error())
			tenants[cluster] = b.tenants[cluster]
			continue
		}

		tenants[cluster] = m
	}

	b.tenants = tenants
}

func getTenantInfoFromCluster(cluster string, users []string) (map[string][]string, error) {
	namespaces, err := b.listNamespaces(cluster)
	if err != nil {
		klog.Errorf("list namespaces error, %s", err.Error())
		return nil, err
	}

	var items []iamv1beta1.SubjectAccessReview
	m := make(map[string][]string)
	for _, namespace := range namespaces {
		for _, user := range users {
			items = append(items, iamv1beta1.SubjectAccessReview{
				Spec: iamv1beta1.SubjectAccessReviewSpec{
					ResourceAttributes: &iamv1beta1.ResourceAttributes{
						Namespace: namespace,
						Verb:      "get",
						Group:     "",
						Version:   "v1",
						Resource:  "pods",
					},
					NonResourceAttributes: nil,
					User:                  user,       // "X-Remote-User" request header
					Groups:                []string{}, // "X-Remote-Group" request header
				},
			})

			if len(items) >= batchSize {
				if err := b.batchRequest(cluster, items, m); err != nil {
					return nil, err
				}
				items = items[:0]
				continue
			}
		}
	}

	if err := b.batchRequest(cluster, items, m); err != nil {
		return nil, err
	}

	return m, err
}

func (b *Backend) listClusters() ([]string, error) {
	res := b.ksClient.Get().AbsPath("/kapis/cluster.kubesphere.io/v1alpha1/clusters").Do(context.Background())
	if err := res.Error(); err != nil {
		return nil, err
	}
	clusterList := &iamv1beta1.UserList{}
	err := res.Into(clusterList)
	if err != nil {
		return nil, err
	}

	var clusters []string
	for _, cluster := range clusterList.Items {
		clusters = append(clusters, cluster.Name)
	}

	return clusters, nil
}

func (b *Backend) listUsers() ([]string, error) {
	res := b.ksClient.Get().AbsPath("/kapis/iam.kubesphere.io/v1beta1/users").Do(context.Background())
	if err := res.Error(); err != nil {
		return nil, err
	}
	userList := &iamv1beta1.UserList{}
	err := res.Into(userList)
	if err != nil {
		return nil, err
	}

	var users []string
	for _, user := range userList.Items {
		users = append(users, user.Name)
	}

	return users, nil
}

func (b *Backend) listNamespaces(cluster string) ([]string, error) {
	res := b.ksClient.Get().AbsPath(fmt.Sprintf("/clusters/%s/api/v1/namespaces", cluster)).Do(context.Background())
	if err := res.Error(); err != nil {
		return nil, err
	}
	namespacesList := &v1.NamespaceList{}
	err := res.Into(namespacesList)
	if err != nil {
		return nil, err
	}

	var namespaces []string
	for _, n := range namespacesList.Items {
		namespaces = append(namespaces, n.Name)
	}

	return namespaces, nil
}

func (b *Backend) batchRequest(cluster string, items []iamv1beta1.SubjectAccessReview, m map[string][]string) error {
	list := &iamv1beta1.SubjectAccessReviewList{
		Items: items,
	}
	if err := b.ksClient.Post().AbsPath(fmt.Sprintf("/clusters/%s/kapis/iam.kubesphere.io/v1beta1/subjectaccessreviews", cluster)).
		Body(list).
		Do(context.Background()).
		Into(list); err != nil {
		klog.Errorf("get access view error: %s", err.Error())
		return err
	}
	for _, item := range items {
		if item.Status.Allowed {
			ns := item.Spec.ResourceAttributes.Namespace
			user := item.Spec.User
			m[ns] = append(m[ns], user)
		}
	}

	return nil
}
