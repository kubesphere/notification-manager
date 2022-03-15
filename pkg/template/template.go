package template

import (
	"bytes"
	"fmt"
	tmplhtml "html/template"
	"io/ioutil"
	"regexp"
	"strings"
	tmpltext "text/template"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/kubesphere/notification-manager/pkg/utils"
)

const (
	DefaultLanguage = "English"
)

type FuncMap map[string]interface{}

var DefaultFuncs = FuncMap{
	"toUpper": strings.ToUpper,
	"toLower": strings.ToLower,
	"title":   strings.Title,
	// join is equal to strings.Join but inverts the argument order
	// for easier pipelining in templates.
	"join": func(sep string, s []string) string {
		return strings.Join(s, sep)
	},
	"match": regexp.MatchString,
	"safeHtml": func(text string) tmplhtml.HTML {
		return tmplhtml.HTML(text)
	},
	"reReplaceAll": func(pattern, repl, text string) string {
		re := regexp.MustCompile(pattern)
		return re.ReplaceAllString(text, repl)
	},
	"stringSlice": func(s ...string) []string {
		return s
	},
}

type Template struct {
	text       *tmpltext.Template
	html       *tmplhtml.Template
	createTime time.Time
	language   string
	dictionary map[string]map[string]string
}

func New(language string, languagePack []string) (*Template, error) {

	t := &Template{
		text:     tmpltext.New("").Option("missingkey=zero"),
		html:     tmplhtml.New("").Option("missingkey=zero"),
		language: language,
	}

	if utils.StringIsNil(t.language) {
		t.language = DefaultLanguage
	}

	var err error
	t.dictionary, err = ParserDictionary(languagePack)
	if err != nil {
		return nil, err
	}

	funcMap := make(map[string]interface{})
	for k, v := range DefaultFuncs {
		funcMap[k] = v
	}

	funcMap["translate"] = func(key string) string {
		return t.translate(key)
	}

	t.text = t.text.Funcs(funcMap)
	t.html = t.html.Funcs(funcMap)

	return t, nil
}

func (t *Template) translate(key string) string {
	m := t.dictionary[t.language]
	if m == nil {
		return key
	}

	val, ok := m[strings.ToLower(key)]
	if ok {
		return val
	}

	return key
}

func (t *Template) ParserText(text ...string) (*Template, error) {

	var err error
	for _, f := range text {
		if t.text, err = t.text.Parse(f); err != nil {
			return nil, err
		}
		if t.html, err = t.html.Parse(f); err != nil {
			return nil, err
		}
	}

	return t, nil
}

func (t *Template) ParserFile(paths ...string) (*Template, error) {
	var files []string
	for _, path := range paths {

		b, err := ioutil.ReadFile(path)
		if err != nil {
			return nil, err
		}

		files = append(files, string(b))
	}

	return t.ParserText(files...)
}

func (t *Template) Clone() *Template {

	textTmpl, _ := t.text.Clone()
	htmlTmpl, _ := t.html.Clone()

	return &Template{
		text:       textTmpl,
		html:       htmlTmpl,
		language:   t.language,
		dictionary: t.dictionary,
	}
}

func (t *Template) Text(name string, data *Data) (string, error) {

	if name == "" {
		return "", nil
	}

	tmpl, err := t.text.Clone()
	if err != nil {
		return "", err
	}
	tmpl, err = tmpl.New("").Option("missingkey=zero").Parse(name)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err = tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return cleanSuffix(buf.String()), nil
}

func (t *Template) Html(name string, data *Data) (string, error) {

	if name == "" {
		return "", nil
	}

	tmpl, err := t.html.Clone()
	if err != nil {
		return "", err
	}
	tmpl, err = tmpl.New("").Option("missingkey=zero").Parse(name)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err = tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return cleanSuffix(buf.String()), nil
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

func (t *Template) Split(data *Data, maxSize int, templateName string, subjectTemplateName string, l log.Logger) ([]string, []string, error) {

	var messages []string
	var subjects []string
	lastMsg := ""
	lastSubject := ""
	d := &Data{
		GroupLabels: data.GroupLabels,
	}
	for i := 0; i < len(data.Alerts); i++ {

		d.Alerts = append(d.Alerts, data.Alerts[i])
		msg, err := t.Text(templateName, d.Format())
		if err != nil {
			return nil, nil, err
		}

		subject := ""
		if subjectTemplateName != "" {
			subject, err = t.Text(templateName, d)
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

func (t *Template) Expired(expiredAt time.Duration) bool {
	if time.Now().Sub(t.createTime) >= expiredAt {
		return true
	}

	return false
}
