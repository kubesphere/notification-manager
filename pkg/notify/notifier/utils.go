package notifier

import (
	"context"
	"crypto/md5"
	"fmt"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	json "github.com/json-iterator/go"
	"github.com/prometheus/alertmanager/template"
	"github.com/prometheus/common/model"
	"io"
	"io/ioutil"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"net/http"
	"net/url"
)

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

func UrlWithPath(u, path string) (string, error) {

	postMessageURL, err := url.Parse(u)
	if err != nil {
		return "", err
	}

	postMessageURL.Path += path
	return postMessageURL.String(), nil
}

func UrlWithParameters(u string, parameters map[string]string) (string, error) {

	postMessageURL, err := url.Parse(u)
	if err != nil {
		return "", err
	}
	postMessageURL.Query()
	values := postMessageURL.Query()
	for k, v := range parameters {
		values.Set(k, v)
	}

	postMessageURL.RawQuery = values.Encode()
	return postMessageURL.String(), nil
}

func DoHttpRequest(ctx context.Context, client *http.Client, request *http.Request) ([]byte, error) {

	if client == nil {
		client = &http.Client{}
	}

	resp, err := client.Do(request.WithContext(ctx))
	if err != nil {
		return nil, err
	}

	defer func() {
		_, _ = io.Copy(ioutil.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		msg := ""
		if body != nil && len(body) > 0 {
			msg = string(body)
		}
		return nil, fmt.Errorf("http error, code: %d, message: %s", resp.StatusCode, msg)
	}

	return body, nil
}

// Filter the notifications with label selector,if the selector is not correct, return all of the notifications.
func Filter(data template.Data, selector *v1.LabelSelector, logger log.Logger) template.Data {

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
