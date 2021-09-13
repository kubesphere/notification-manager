package notifier

import (
	"context"
	"fmt"
	"net/url"
	"reflect"
	"regexp"
	"strings"
	"sync"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/kubesphere/notification-manager/pkg/utils"
	"github.com/prometheus/alertmanager/notify"
	"github.com/prometheus/alertmanager/template"
	"github.com/prometheus/alertmanager/types"
	"github.com/prometheus/common/model"
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
	tmpl.ExternalURL, _ = url.Parse("https://kubesphere.io")

	t.Tmpl = tmpl
	notifierTemplate = t

	return notifierTemplate, nil
}

func (t *Template) TempleText(name string, data template.Data, l log.Logger) (string, error) {

	name = t.transform(name)

	ctx := context.Background()
	ctx = notify.WithGroupLabels(ctx, utils.KvToLabelSet(data.GroupLabels))
	ctx = notify.WithReceiverName(ctx, data.Receiver)

	var as []*types.Alert
	for _, a := range data.Alerts {
		as = append(as, &types.Alert{
			Alert: model.Alert{
				Labels:       utils.KvToLabelSet(a.Labels),
				Annotations:  utils.KvToLabelSet(a.Annotations),
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

	s := text(name)
	if utils.StringIsNil(s) {
		return s, utils.Errorf("template '%s' error or not exist", name)
	}

	return cleanSuffix(s), nil
}

// Delete all `space`, `LF`, `CR` at the end of string.
func cleanSuffix(s string) string {
	for {
		l := len(s)
		if byte(s[l-1]) == 10 || byte(s[l-1]) == 13 || byte(s[l-1]) == 32 {
			s = s[:l-1]
		} else {
			break
		}
	}

	return s
}

func (t *Template) transform(name string) string {

	n := strings.ReplaceAll(name, " ", "")

	b, _ := regexp.MatchString("{{template\"(.*?)\".}}", n)
	if b {
		return name
	}

	return fmt.Sprintf("{{ template \"%s\" . }}", name)
}

func (t *Template) Split(data template.Data, maxSize int, templateName string, subjectTemplateName string, l log.Logger) ([]string, []string, error) {
	d := template.Data{
		Receiver:    data.Receiver,
		GroupLabels: data.GroupLabels,
	}
	var messages []string
	var subjects []string
	lastMsg := ""
	lastSubject := ""
	for i := 0; i < len(data.Alerts); i++ {

		d.Alerts = append(d.Alerts, data.Alerts[i])
		msg, err := t.TempleText(templateName, d, l)
		if err != nil {
			return nil, nil, err
		}

		subject := ""
		if subjectTemplateName != "" {
			subject, err = t.TempleText(templateName, d, l)
			if err != nil {
				return nil, nil, err
			}
		}

		if Len(msg) < maxSize {
			lastMsg = msg
			lastSubject = subject
			continue
		}

		// If there is only alert, and the message length is greater than MaxMessageSize, drop this alert.
		if len(d.Alerts) == 1 {
			_ = level.Error(l).Log("msg", "alert is too large, drop it")
			d.Alerts = nil
			lastMsg = ""
			lastSubject = ""
			continue
		}

		messages = append(messages, lastMsg)
		subjects = append(subjects, subject)

		d.Alerts = nil
		i = i - 1
		lastMsg = ""
		lastSubject = ""
	}

	if len(lastMsg) > 0 {
		messages = append(messages, lastMsg)
		subjects = append(subjects, lastSubject)
	}

	return messages, subjects, nil
}

// Len return the length of string after serialized.
// When a string is serialized, the escape character in the string will occupy two bytes because of `\`.
func Len(s string) int {

	bs, err := utils.JsonMarshal(s)
	if err != nil {
		return len(s)
	}

	// Remove the '"' at the begin and end.
	return len(string(bs)) - 2
}
