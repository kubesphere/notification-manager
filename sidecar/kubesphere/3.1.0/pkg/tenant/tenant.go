package tenant

import (
	"kubesphere/pkg/ks"

	"k8s.io/klog"
)

var (
	tenants map[string]map[string]string
)

func FromNamespace(ns string) []string {

	m, ok := tenants[ns]
	if !ok {
		return nil
	}

	array := make([]string, 0)
	for k := range m {
		array = append(array, k)
	}
	return array
}

func Reload(r *ks.Runtime) error {

	m := make(map[string]map[string]string)

	users, err := r.ListUser()
	if err != nil {
		klog.Errorf("list users error, %s", err.Error())
		return err
	}

	for _, u := range users {

		workspaces, err := r.ListWorkspaces(u)
		if err != nil {
			klog.Errorf("list workspaces error, %s", err.Error())
			return err
		}
		workspaces = append(workspaces, "")
		for _, workspace := range workspaces {
			namespaces, err := r.ListNamespaces(u, workspace)
			if err != nil {
				klog.Errorf("list namespaces error, %s", err.Error())
				return err
			}

			for _, namespace := range namespaces {
				array, ok := m[namespace]
				if !ok {
					array = make(map[string]string)
				}

				array[u.Name] = ""
				m[namespace] = array
			}
		}
	}

	tenants = m

	klog.Info("reload tenant")
	return nil
}
