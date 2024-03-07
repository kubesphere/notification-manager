package main

import (
	"context"
	"time"

	"k8s.io/api/authorization/v1beta1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/klog"
	iamv1beta1 "kubesphere.io/api/iam/v1beta1"
	"kubesphere.io/client-go/rest"
)

type Backend struct {
	ksClient *rest.RESTClient
	tenants  map[string]map[string]string

	interval time.Duration
}

func NewBackend(host, username, password string, interval time.Duration) (*Backend, error) {
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
		ksClient: c,
		tenants:  make(map[string]map[string]string),
		interval: interval,
	}, err
}

func (b *Backend) FromNamespace(ns string) []string {

	m, ok := b.tenants[ns]
	if !ok {
		return nil
	}

	array := make([]string, 0)
	for k := range m {
		array = append(array, k)
	}
	return array
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

	users, err := b.listUsers()
	if err != nil {
		klog.Errorf("list users error, %s", err.Error())
	}

	namespaces, err := b.listNamespaces()
	if err != nil {
		klog.Errorf("list namespaces error, %s", err.Error())
	}

	var items []iamv1beta1.SubjectAccessReview

	m := make(map[string]map[string]string)
	for _, namespace := range namespaces {
		for _, user := range users {
			sar := iamv1beta1.SubjectAccessReview{
				Spec: iamv1beta1.SubjectAccessReviewSpec{
					ResourceAttributes: &iamv1beta1.ResourceAttributes{
						Namespace: namespace,
						Verb:      "get",
						Group:     "notification.kubesphere.io",
						Version:   "v2beta2",
						Resource:  "receivenotification",
					},
					NonResourceAttributes: nil,
					User:                  user,       // "X-Remote-User" request header
					Groups:                []string{}, // "X-Remote-Group" request header
				},
			}
			items = append(items, sar)
		}
	}
	b.batchRequest(items, m)
	b.tenants = m
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

func (b *Backend) listNamespaces() ([]string, error) {
	res := b.ksClient.Get().AbsPath("/api/v1/namespaces").Do(context.Background())
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

func (b *Backend) canAccess(user, namespace string) (bool, error) {
	subjectAccessReview := &v1beta1.SubjectAccessReview{
		Spec: v1beta1.SubjectAccessReviewSpec{
			ResourceAttributes: &v1beta1.ResourceAttributes{
				Namespace: namespace,
				Verb:      "get",
				Group:     "notification.kubesphere.io",
				Version:   "v2beta2",
				Resource:  "receivenotification",
			},
			NonResourceAttributes: nil,
			User:                  user,       // "X-Remote-User" request header
			Groups:                []string{}, // "X-Remote-Group" request header
		},
	}

	if err := b.ksClient.Post().AbsPath("/kapis/iam.kubesphere.io/v1beta1/subjectaccessreviews").
		Body(subjectAccessReview).
		Do(context.Background()).
		Into(subjectAccessReview); err != nil {
		return false, err
	}

	return subjectAccessReview.Status.Allowed, nil
}

func (b *Backend) batchRequest(subjectAccessReviews []iamv1beta1.SubjectAccessReview, m map[string]map[string]string) {

	batchSize := 500
	for i := 0; i < len(subjectAccessReviews); i += batchSize {
		items := subjectAccessReviews[i:minimum(i+batchSize, len(subjectAccessReviews))]
		list := &iamv1beta1.SubjectAccessReviewList{
			Items: items,
		}
		if err := b.ksClient.Post().AbsPath("/kapis/iam.kubesphere.io/v1beta1/subjectaccessreviews").
			Body(list).
			Do(context.Background()).
			Into(list); err != nil {
			klog.Errorf("get access view error: %s", err.Error())
			return
		}
		for _, item := range items {
			if item.Status.Allowed {
				ns := item.Spec.ResourceAttributes.Namespace
				user := item.Spec.User
				array, ok := m[ns]
				if !ok {
					array = make(map[string]string)
				}
				array[user] = ""
				m[ns] = array
			}
		}
	}
}

func minimum(a, b int) int {
	if a < b {
		return a
	}
	return b
}
