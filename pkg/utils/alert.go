package utils

import (
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

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
