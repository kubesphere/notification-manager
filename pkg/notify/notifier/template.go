package notifier

import (
	"context"
	"fmt"
	"github.com/go-kit/kit/log"
	"github.com/prometheus/alertmanager/notify"
	"github.com/prometheus/alertmanager/template"
	"github.com/prometheus/alertmanager/types"
	"github.com/prometheus/common/model"
	"net/url"
	"reflect"
	"regexp"
	"strings"
	"sync"
)

type Template struct {
	Tmpl *template.Template
	path []string
}

var notifierTemplate *Template
var templatePaths []string
var mutex sync.Mutex

func NewTemplate(paths []string) (*Template, error) {

	mutex.Lock()
	defer mutex.Unlock()

	if !reflect.DeepEqual(templatePaths, paths) {
		templatePaths = paths
		notifierTemplate = nil
	}

	if notifierTemplate != nil {
		return notifierTemplate, nil
	}

	t := &Template{}

	tmpl, err := template.FromGlobs(paths...)
	if err != nil {
		return nil, err
	}
	tmpl.ExternalURL, _ = url.Parse("http://kubesphere.io")

	t.Tmpl = tmpl
	notifierTemplate = t

	return notifierTemplate, nil
}

func (t *Template) TemlText(name string, l log.Logger, data template.Data) (string, error) {

	name = t.transform(name)

	ctx := context.Background()
	ctx = notify.WithGroupLabels(ctx, KvToLabelSet(data.GroupLabels))
	ctx = notify.WithReceiverName(ctx, data.Receiver)

	var as []*types.Alert
	for _, a := range data.Alerts {
		as = append(as, &types.Alert{
			Alert: model.Alert{
				Labels:       KvToLabelSet(a.Labels),
				Annotations:  KvToLabelSet(a.Annotations),
				StartsAt:     a.StartsAt,
				EndsAt:       a.EndsAt,
				GeneratorURL: a.GeneratorURL,
			},
		})
	}

	d := notify.GetTemplateData(ctx, t.Tmpl, as, l)

	var e error
	text := notify.TmplText(t.Tmpl, d, &e)
	if e != nil {
		return "", e
	}

	return strings.TrimRight(text(name), "\n"), nil
}

func (t *Template) transform(name string) string {

	n := strings.ReplaceAll(name, " ", "")

	b, _ := regexp.MatchString("{{template\"(.*?)\".}}", n)
	if b {
		return name
	}

	return fmt.Sprintf("{{ template \"%s\" . }}", name)
}
