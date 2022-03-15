package template

import (
	"sort"
	"time"

	"github.com/kubesphere/notification-manager/pkg/utils"

	"github.com/kubesphere/notification-manager/pkg/constants"
)

type Data struct {
	Alerts Alerts `json:"alerts"`

	GroupLabels       KV `json:"groupLabels"`
	CommonLabels      KV `json:"commonLabels"`
	CommonAnnotations KV `json:"commonAnnotations"`
}

func (d *Data) Format() *Data {

	if len(d.Alerts) == 0 {
		return d
	}

	var (
		commonLabels      = d.Alerts[0].Labels.Clone()
		commonAnnotations = d.Alerts[0].Annotations.Clone()
	)
	for _, a := range d.Alerts[1:] {
		if len(commonLabels) == 0 && len(commonAnnotations) == 0 {
			break
		}
		for ln, lv := range commonLabels {
			if a.Labels[ln] != lv {
				delete(commonLabels, ln)
			}
		}
		for an, av := range commonAnnotations {
			if a.Annotations[an] != av {
				delete(commonAnnotations, an)
			}
		}
	}

	d.CommonLabels = make(map[string]string)
	for k, v := range commonLabels {
		d.CommonLabels[k] = v
	}

	d.CommonAnnotations = make(map[string]string)
	for k, v := range commonAnnotations {
		d.CommonAnnotations[k] = v
	}

	return d
}

func (d *Data) Status() string {
	if len(d.Alerts.Firing()) > 0 {
		return constants.AlertFiring
	}
	return constants.AlertResolved
}

// Pair is a key/value string pair.
type Pair struct {
	Name, Value string
}

// Pairs is a list of key/value string pairs.
type Pairs []Pair

// Names returns a list of names of the pairs.
func (ps Pairs) Names() []string {
	ns := make([]string, 0, len(ps))
	for _, p := range ps {
		ns = append(ns, p.Name)
	}
	return ns
}

// Values returns a list of values of the pairs.
func (ps Pairs) Values() []string {
	vs := make([]string, 0, len(ps))
	for _, p := range ps {
		vs = append(vs, p.Value)
	}
	return vs
}

// KV is a set of key/value string pairs.
type KV map[string]string

// SortedPairs returns a sorted list of key/value pairs.
func (kv KV) SortedPairs() Pairs {
	var (
		pairs     = make([]Pair, 0, len(kv))
		keys      = make([]string, 0, len(kv))
		sortStart = 0
	)
	for k := range kv {
		if k == constants.AlertName {
			keys = append([]string{k}, keys...)
			sortStart = 1
		} else {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys[sortStart:])

	for _, k := range keys {
		pairs = append(pairs, Pair{k, kv[k]})
	}
	return pairs
}

// Remove returns a copy of the key/value set without the given keys.
func (kv KV) Remove(keys []string) KV {
	keySet := make(map[string]struct{}, len(keys))
	for _, k := range keys {
		keySet[k] = struct{}{}
	}

	res := KV{}
	for k, v := range kv {
		if _, ok := keySet[k]; !ok {
			res[k] = v
		}
	}
	return res
}

// Names returns the names of the label names in the LabelSet.
func (kv KV) Names() []string {
	return kv.SortedPairs().Names()
}

// Values returns a list of the values in the LabelSet.
func (kv KV) Values() []string {
	return kv.SortedPairs().Values()
}

func (kv KV) Clone() KV {
	m := make(map[string]string)
	for k, v := range kv {
		m[k] = v
	}
	return m
}

type Alert struct {
	Status      string `json:"status"`
	Labels      KV     `json:"labels"`
	Annotations KV     `json:"annotations"`

	StartsAt time.Time `json:"startsAt,omitempty"`
	EndsAt   time.Time `json:"endsAt,omitempty"`
}

func (a *Alert) Message() string {
	message := a.Annotations[constants.AlertMessage]
	if utils.StringIsNil(message) {
		message = a.Annotations[constants.AlertSummary]
		if utils.StringIsNil(message) {
			message = a.Annotations[constants.AlertSummaryCN]
		}
	}

	return message
}

func (a *Alert) MessageCN() string {
	message := a.Annotations[constants.AlertSummaryCN]
	if utils.StringIsNil(message) {
		message = a.Annotations[constants.AlertMessage]
		if utils.StringIsNil(message) {
			message = a.Annotations[constants.AlertSummary]
		}
	}

	return message
}

// Alerts is a list of Alert objects.
type Alerts []*Alert

// Firing returns the subset of alerts that are firing.
func (as Alerts) Firing() []*Alert {
	var res []*Alert
	for _, a := range as {
		if a.Status == constants.AlertFiring {
			res = append(res, a)
		}
	}
	return res
}

// Resolved returns the subset of alerts that are resolved.
func (as Alerts) Resolved() []*Alert {
	var res []*Alert
	for _, a := range as {
		if a.Status == constants.AlertResolved {
			res = append(res, a)
		}
	}
	return res
}
