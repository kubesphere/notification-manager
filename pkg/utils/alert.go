package utils

import (
	"github.com/prometheus/alertmanager/template"
	"github.com/prometheus/common/model"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func AlertConvert(alert *template.Alert) *model.Alert {

	return &model.Alert{
		Labels:       KvToLabelSet(alert.Labels),
		Annotations:  KvToLabelSet(alert.Annotations),
		StartsAt:     alert.StartsAt,
		EndsAt:       alert.EndsAt,
		GeneratorURL: "",
	}
}

func KvToLabelSet(obj template.KV) model.LabelSet {

	ls := model.LabelSet{}
	for k, v := range obj {
		ls[model.LabelName(k)] = model.LabelValue(v)
	}

	return ls
}

func LabelSetToKV(ls model.LabelSet) map[string]string {

	m := make(map[string]string)
	for k, v := range ls {
		m[string(k)] = string(v)
	}

	return m
}

// FilterAlerts filter the alerts with label selector,if the selector is not correct, return all of the alerts.
func FilterAlerts(alerts []*model.Alert, selector *v1.LabelSelector) ([]*model.Alert, error) {

	if selector == nil {
		return alerts, nil
	}

	labelSelector, err := v1.LabelSelectorAsSelector(selector)
	if err != nil {
		return alerts, err
	}

	if labelSelector.Empty() {
		return alerts, nil
	}

	var as []*model.Alert
	for _, alert := range alerts {
		if labelSelector.Matches(labels.Set(LabelSetToKV(alert.Labels))) {
			as = append(as, alert)
		}
	}

	return as, nil
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
