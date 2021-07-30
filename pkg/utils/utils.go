package utils

import (
	"crypto/md5"
	"fmt"
	"strings"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	json "github.com/json-iterator/go"
	"github.com/prometheus/alertmanager/template"
	"github.com/prometheus/common/model"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func StringIsNil(s string) bool {
	return s == ""
}

func ArrayToString(array []string, sep string) string {

	if array == nil || len(array) == 0 {
		return ""
	}

	s := ""
	for _, elem := range array {
		s = s + elem + sep
	}

	return strings.TrimSuffix(s, sep)
}

func Md5key(val interface{}) (string, error) {

	bs, err := json.Marshal(val)
	if err != nil {
		return "", err
	}

	data := md5.Sum(bs)
	return fmt.Sprintf("%x", data), nil
}

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

// SplitAlerts splits input data into a slice of data,
// and each of them only contains one Alert (data.Alerts only contains 1 element).
// This function serves to break a long message into several short messages.
func SplitAlerts(data template.Data) []template.Data {

	splitData := make([]template.Data, len(data.Alerts))
	for i := 0; i < len(splitData); i++ {
		splitData[i] = template.Data{
			Alerts:            template.Alerts{data.Alerts[i]},
			Receiver:          data.Receiver,
			Status:            data.Status,
			GroupLabels:       data.GroupLabels,
			CommonLabels:      data.CommonLabels,
			CommonAnnotations: data.CommonAnnotations,
			ExternalURL:       data.ExternalURL,
		}
	}
	return splitData
}
