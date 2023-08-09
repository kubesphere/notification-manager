package utils

import (
	"github.com/kubesphere/notification-manager/pkg/apis/v2beta2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func LabelSelectorDeal(ls *v2beta2.LabelSelector) (labels.Selector, error) {
	var selector *metav1.LabelSelector
	var lsr metav1.LabelSelectorRequirement
	var lsrs []metav1.LabelSelectorRequirement
	if ls.MatchExpressions == nil || ls.MatchLabels == nil {
		return nil,nil
	}
	selector.MatchLabels = ls.MatchLabels

	for _, requirement := range ls.MatchExpressions {
		if requirement.Operator != v2beta2.LabelSelectorOpMatch {
			lsr.Key = requirement.Key
			lsr.Operator = metav1.LabelSelectorOperator(requirement.Operator)
			lsr.Values = requirement.Values
			lsrs = append(lsrs, lsr)
		} else {
			lsr.Key = requirement.Key
			lsr.Operator = metav1.LabelSelectorOperator(v2beta2.LabelSelectorOpMatch)
			lsr.Values = requirement.Values
			lsrs = append(lsrs, lsr)
		}
	}
	selector.MatchExpressions = lsrs
	sl, err := metav1.LabelSelectorAsSelector(selector)
	if err !=nil {
		return nil,err
	}
	return sl,nil
}
