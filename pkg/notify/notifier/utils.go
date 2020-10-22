package notifier

import (
	"context"
	"crypto/md5"
	"fmt"
	jsoniter "github.com/json-iterator/go"
	"github.com/prometheus/alertmanager/template"
	"github.com/prometheus/common/model"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
)

func Md5key(val interface{}) (string, error) {

	bs, err := jsoniter.Marshal(val)
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

func JsonOut(v interface{}) {
	bs, _ := jsoniter.Marshal(v)
	fmt.Println(string(bs))
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
