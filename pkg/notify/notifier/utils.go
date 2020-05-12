package notifier

import (
	"crypto/md5"
	"fmt"
	jsoniter "github.com/json-iterator/go"
	"github.com/prometheus/alertmanager/template"
	"github.com/prometheus/common/model"
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
