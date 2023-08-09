package utils

import (
	"github.com/kubesphere/notification-manager/pkg/apis/v2beta2"
	"k8s.io/apimachinery/pkg/labels"

)

func LabelMatchSelector(label map[string]string, selector *v2beta2.LabelSelector) bool {

	if selector == nil {
		return true
	}

	labelSelector, err := LabelSelectorDeal(selector)
	if err != nil {
		return false
	}
	if labelSelector.Empty() {
		return true
	}

	return labelSelector.Matches(labels.Set(label))
}
