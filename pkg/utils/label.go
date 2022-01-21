package utils

import (
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/alertmanager/template"
	"github.com/prometheus/common/model"
	"k8s.io/apimachinery/pkg/api/meta"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func KvToLabelSet(obj template.KV) model.LabelSet {

	ls := model.LabelSet{}
	for k, v := range obj {
		ls[model.LabelName(k)] = model.LabelValue(v)
	}

	return ls
}

// FilterAlerts filter the alerts with label selector,if the selector is not correct, return all of the alerts.
func FilterAlerts(data template.Data, selector *v1.LabelSelector, logger log.Logger) template.Data {

	if selector == nil {
		return data
	}

	labelSelector, err := v1.LabelSelectorAsSelector(selector)
	if err != nil {
		_ = level.Error(logger).Log("msg", "filter notification error", "error", err)
		return data
	}

	if labelSelector.Empty() {
		return data
	}

	newData := template.Data{
		Receiver:          data.Receiver,
		Status:            data.Status,
		GroupLabels:       data.GroupLabels,
		CommonLabels:      data.CommonLabels,
		CommonAnnotations: data.CommonAnnotations,
		ExternalURL:       data.ExternalURL,
	}

	for _, alert := range data.Alerts {
		if labelSelector.Matches(labels.Set(alert.Labels)) {
			newData.Alerts = append(newData.Alerts, alert)
		}
	}

	return newData
}

func LabelMatchSelector(label map[string]string, selector *v1.LabelSelector) bool {

	if selector == nil {
		return true
	}

	labelSelector, err := v1.LabelSelectorAsSelector(selector)
	if err != nil {
		return false
	}
	if labelSelector.Empty() {
		return true
	}

	return labelSelector.Matches(labels.Set(label))
}

func GetObjectName(obj interface{}) string {
	accessor, err := meta.Accessor(obj)
	if err != nil {
		return ""
	}

	return accessor.GetName()
}

func GetObjectLabels(obj interface{}) map[string]string {
	accessor, err := meta.Accessor(obj)
	if err != nil {
		return nil
	}

	return accessor.GetLabels()
}
