// This is a generated file. Do not edit directly.
// Run hack/pin-dependency.sh to change pinned dependency versions.
// Run hack/update-vendor.sh to update go.mod files and the vendor directory.

module kubesphere

go 1.16

require (
	github.com/emicklei/go-restful/v3 v3.8.0
	github.com/spf13/cobra v1.4.0
	github.com/spf13/pflag v1.0.5
	k8s.io/api v0.22.5
	k8s.io/apimachinery v0.22.5
	k8s.io/apiserver v0.22.5
	k8s.io/client-go v12.0.0+incompatible
	k8s.io/klog v1.0.0
	kubesphere.io/api v0.0.0
	kubesphere.io/kubesphere v0.0.0-20210924035154-15205cbc4007
	sigs.k8s.io/controller-runtime v0.9.3
)

replace (
	github.com/BurntSushi/toml => github.com/BurntSushi/toml v0.3.1
	github.com/OneOfOne/xxhash => github.com/OneOfOne/xxhash v1.2.7
	github.com/PuerkitoBio/purell => github.com/PuerkitoBio/purell v1.1.1
	github.com/PuerkitoBio/urlesc => github.com/PuerkitoBio/urlesc v0.0.0-20170810143723-de5bf2ad4578
	github.com/asaskevich/govalidator => github.com/asaskevich/govalidator v0.0.0-20200108200545-475eaeb16496
	github.com/beorn7/perks => github.com/beorn7/perks v1.0.1
	github.com/cespare/xxhash/v2 => github.com/cespare/xxhash/v2 v2.1.1
	github.com/davecgh/go-spew => github.com/davecgh/go-spew v1.1.1
	github.com/emicklei/go-restful v2.14.3+incompatible => github.com/emicklei/go-restful/v3 v3.8.0
	github.com/evanphx/json-patch => github.com/evanphx/json-patch v4.9.0+incompatible
	github.com/ghodss/yaml => github.com/ghodss/yaml v1.0.0
	github.com/go-logr/logr => github.com/go-logr/logr v0.4.0
	github.com/go-logr/zapr => github.com/go-logr/zapr v0.4.0
	github.com/go-openapi/jsonpointer => github.com/go-openapi/jsonpointer v0.19.3
	github.com/go-openapi/jsonreference => github.com/go-openapi/jsonreference v0.19.3
	github.com/go-openapi/spec => github.com/go-openapi/spec v0.19.7
	github.com/go-openapi/swag => github.com/go-openapi/swag v0.19.9
	github.com/gobwas/glob => github.com/gobwas/glob v0.2.3
	github.com/gogo/protobuf => github.com/gogo/protobuf v1.3.2
	github.com/golang/glog => github.com/golang/glog v0.0.0-20160126235308-23def4e6c14b
	github.com/golang/groupcache => github.com/golang/groupcache v0.0.0-20200121045136-8c9f03a8e57e
	github.com/golang/protobuf => github.com/golang/protobuf v1.4.2
	github.com/google/go-cmp => github.com/google/go-cmp v0.4.0
	github.com/google/gofuzz => github.com/google/gofuzz v1.1.0
	github.com/google/uuid => github.com/google/uuid v1.1.1
	github.com/googleapis/gnostic => github.com/googleapis/gnostic v0.4.1
	github.com/hashicorp/golang-lru => github.com/hashicorp/golang-lru v0.5.4
	github.com/imdario/mergo => github.com/imdario/mergo v0.3.9
	github.com/inconshreveable/mousetrap => github.com/inconshreveable/mousetrap v1.0.0
	github.com/json-iterator/go => github.com/json-iterator/go v1.1.10
	github.com/kelseyhightower/envconfig => github.com/kelseyhightower/envconfig v1.4.0
	github.com/konsorten/go-windows-terminal-sequences => github.com/konsorten/go-windows-terminal-sequences v1.0.2
	github.com/kr/pretty => github.com/kr/pretty v0.2.0
	github.com/kr/text => github.com/kr/text v0.1.0
	github.com/mailru/easyjson => github.com/mailru/easyjson v0.7.1
	github.com/matttproud/golang_protobuf_extensions => github.com/matttproud/golang_protobuf_extensions v1.0.1
	github.com/modern-go/concurrent => github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd
	github.com/modern-go/reflect2 => github.com/modern-go/reflect2 v1.0.1
	github.com/nxadm/tail => github.com/nxadm/tail v1.4.4
	github.com/onsi/ginkgo => github.com/onsi/ginkgo v1.14.0
	github.com/onsi/gomega => github.com/onsi/gomega v1.10.1
	github.com/open-policy-agent/opa => github.com/open-policy-agent/opa v0.40.0
	github.com/pkg/errors => github.com/pkg/errors v0.9.1
	github.com/pmezard/go-difflib => github.com/pmezard/go-difflib v1.0.0
	github.com/projectcalico/go-json => github.com/projectcalico/go-json v0.0.0-20161128004156-6219dc7339ba
	github.com/projectcalico/go-yaml => github.com/projectcalico/go-yaml v0.0.0-20161201183616-955bc3e451ef
	github.com/projectcalico/go-yaml-wrapper => github.com/projectcalico/go-yaml-wrapper v0.0.0-20161127220527-598e54215bee
	github.com/projectcalico/libcalico-go => github.com/projectcalico/libcalico-go v1.7.2-0.20191014160346-2382c6cdd056
	github.com/prometheus/client_golang => github.com/prometheus/client_golang v1.11.0
	github.com/prometheus/client_model => github.com/prometheus/client_model v0.2.0
	github.com/prometheus/common => github.com/prometheus/common v0.10.0
	github.com/prometheus/procfs => github.com/prometheus/procfs v0.1.3
	github.com/rcrowley/go-metrics => github.com/rcrowley/go-metrics v0.0.0-20181016184325-3113b8401b8a
	github.com/sirupsen/logrus => github.com/sirupsen/logrus v1.4.2
	github.com/spf13/cobra => github.com/spf13/cobra v0.0.5
	github.com/spf13/pflag => github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify => github.com/stretchr/testify v1.4.0
	github.com/yashtewari/glob-intersection => github.com/yashtewari/glob-intersection v0.0.0-20180916065949-5c77d914dd0b
	go.uber.org/atomic => go.uber.org/atomic v1.6.0
	go.uber.org/multierr => go.uber.org/multierr v1.3.0
	go.uber.org/tools => go.uber.org/tools v0.0.0-20190618225709-2cfd321de3ee
	go.uber.org/zap => go.uber.org/zap v1.13.0
	golang.org/x/lint => golang.org/x/lint v0.0.0-20190301231843-5614ed5bae6f
	golang.org/x/net => golang.org/x/net v0.0.0-20211209124913-491a49abca63
	golang.org/x/oauth2 => golang.org/x/oauth2 v0.0.0-20190402181905-9f3314589c9a
	golang.org/x/sys => golang.org/x/sys v0.0.0-20220412211240-33da011f77ad
	golang.org/x/term => golang.org/x/term v0.0.0-20201126162022-7de9c90e9dd1
	golang.org/x/text => golang.org/x/text v0.3.7
	golang.org/x/time => golang.org/x/time v0.0.0-20190308202827-9d24e82272b4
	golang.org/x/tools => golang.org/x/tools v0.0.0-20190710153321-831012c29e42
	golang.org/x/xerrors => golang.org/x/xerrors v0.0.0-20190717185122-a985d3407aa7
	gomodules.xyz/jsonpatch/v2 => gomodules.xyz/jsonpatch/v2 v2.0.1
	google.golang.org/appengine => google.golang.org/appengine v1.6.6
	google.golang.org/genproto => google.golang.org/genproto v0.0.0-20200420144010-e5e8543f8aeb
	google.golang.org/grpc => google.golang.org/grpc v1.27.1
	google.golang.org/protobuf => google.golang.org/protobuf v1.23.0
	gopkg.in/check.v1 => gopkg.in/check.v1 v1.0.0-20190902080502-41f04d3bba15
	gopkg.in/inf.v0 => gopkg.in/inf.v0 v0.9.1
	gopkg.in/tomb.v1 => gopkg.in/tomb.v1 v1.0.0-20141024135613-dd632973f1e7
	gopkg.in/yaml.v2 => gopkg.in/yaml.v2 v2.4.0
	gotest.tools => gotest.tools v2.2.0+incompatible
	honnef.co/go/tools => honnef.co/go/tools v0.0.1-2020.1.3
	istio.io/api => istio.io/api v0.0.0-20201113182140-d4b7e3fc2b44
	istio.io/client-go => istio.io/client-go v0.0.0-20201113183938-0734e976e785

	istio.io/gogo-genproto => istio.io/gogo-genproto v0.0.0-20201113182723-5b8563d8a012
	k8s.io/api => k8s.io/api v0.21.2
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.21.2
	k8s.io/apimachinery => k8s.io/apimachinery v0.21.2
	k8s.io/apiserver => k8s.io/apiserver v0.21.2
	k8s.io/client-go => k8s.io/client-go v0.21.2
	k8s.io/component-base => k8s.io/component-base v0.21.2
	k8s.io/klog => k8s.io/klog v1.0.0
	k8s.io/klog/v2 => k8s.io/klog/v2 v2.8.0
	k8s.io/kube-openapi => k8s.io/kube-openapi v0.0.0-20210305001622-591a79e4bda7
	k8s.io/kubectl => k8s.io/kubectl v0.21.2
	k8s.io/utils => k8s.io/utils v0.0.0-20200603063816-c1c6865ac451
	kubesphere.io/api => kubesphere.io/api v0.0.0-20210917114432-19cb9aacd65f
	kubesphere.io/client-go => kubesphere.io/client-go v0.3.1
	kubesphere.io/monitoring-dashboard => kubesphere.io/monitoring-dashboard v0.2.2
	sigs.k8s.io/application => sigs.k8s.io/application v0.8.4-0.20201016185654-c8e2959e57a0
	sigs.k8s.io/controller-runtime => sigs.k8s.io/controller-runtime v0.9.3
	sigs.k8s.io/structured-merge-diff/v4 => sigs.k8s.io/structured-merge-diff/v4 v4.1.0
	sigs.k8s.io/yaml => sigs.k8s.io/yaml v1.2.0
)
