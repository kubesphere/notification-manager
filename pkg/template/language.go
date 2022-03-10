package template

import (
	"strings"

	"github.com/kubesphere/notification-manager/pkg/constants"
	"sigs.k8s.io/yaml"
)

type languagePack struct {
	Name       string            `json:"name,omitempty"`
	Dictionary map[string]string `json:"dictionary,omitempty"`
}

func ParserDictionary(pack []string) (map[string]map[string]string, error) {

	dictionary := make(map[string]map[string]string)
	for _, p := range pack {
		var lps []languagePack
		if err := yaml.Unmarshal([]byte(p), &lps); err != nil {
			return nil, err
		}

		for _, lp := range lps {
			m := dictionary[lp.Name]
			if m == nil {
				m = make(map[string]string)
			}
			for k, v := range lp.Dictionary {
				m[strings.ToLower(k)] = v
			}
			dictionary[lp.Name] = m
		}
	}

	dictionary[DefaultLanguage] = map[string]string{
		constants.AlertFiring:   strings.ToUpper(constants.AlertFiring),
		constants.AlertResolved: strings.ToUpper(constants.AlertResolved),
	}

	return dictionary, nil
}
