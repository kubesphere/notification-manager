module github.com/kubesphere/notification-manager

go 1.17

require (
	github.com/alibabacloud-go/darabonba-openapi v0.1.5
	github.com/alibabacloud-go/dysmsapi-20170525/v2 v2.0.1
	github.com/aws/aws-sdk-go-v2 v1.16.4
	github.com/aws/aws-sdk-go-v2/config v1.15.7
	github.com/aws/aws-sdk-go-v2/credentials v1.12.2
	github.com/aws/aws-sdk-go-v2/service/sns v1.17.6
	github.com/emicklei/go-restful v2.9.5+incompatible
	github.com/go-chi/chi v4.0.3+incompatible
	github.com/go-kit/kit v0.9.0
	github.com/go-logr/logr v1.2.0
	github.com/golang/glog v0.0.0-20160126235308-23def4e6c14b
	github.com/json-iterator/go v1.1.12
	github.com/mitchellh/hashstructure v1.1.0
	github.com/modern-go/reflect2 v1.0.2
	github.com/mwitkow/go-conntrack v0.0.0-20190716064945-2f068394615f
	github.com/onsi/ginkgo v1.16.4
	github.com/onsi/gomega v1.15.0
	github.com/robfig/cron/v3 v3.0.1
	github.com/spf13/cobra v1.1.1
	github.com/spf13/pflag v1.0.5
	github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common v1.0.194
	github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/sms v1.0.194
	gopkg.in/alecthomas/kingpin.v2 v2.2.6
	gopkg.in/yaml.v2 v2.4.0
	k8s.io/api v0.21.4
	k8s.io/apimachinery v0.21.4
	k8s.io/client-go v0.21.4
	sigs.k8s.io/controller-runtime v0.9.7
	sigs.k8s.io/yaml v1.2.0
)

require (
	cloud.google.com/go v0.54.0 // indirect
	github.com/alecthomas/template v0.0.0-20190718012654-fb15b899a751 // indirect
	github.com/alecthomas/units v0.0.0-20190924025748-f65c72e2690d // indirect
	github.com/alibabacloud-go/debug v0.0.0-20190504072949-9472017b5c68 // indirect
	github.com/alibabacloud-go/endpoint-util v1.1.0 // indirect
	github.com/alibabacloud-go/openapi-util v0.0.7 // indirect
	github.com/alibabacloud-go/tea v1.1.15 // indirect
	github.com/alibabacloud-go/tea-utils v1.3.9 // indirect
	github.com/aliyun/credentials-go v1.1.2 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.12.5 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.1.11 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.4.5 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.3.12 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.9.5 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.11.5 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.16.6 // indirect
	github.com/aws/smithy-go v1.11.2 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.1.1 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/evanphx/json-patch v4.12.0+incompatible // indirect
	github.com/fsnotify/fsnotify v1.4.9 // indirect
	github.com/go-logfmt/logfmt v0.5.0 // indirect
	github.com/go-logr/zapr v0.4.0 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/groupcache v0.0.0-20200121045136-8c9f03a8e57e // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/google/go-cmp v0.5.8 // indirect
	github.com/google/gofuzz v1.1.0 // indirect
	github.com/google/uuid v1.1.2 // indirect
	github.com/googleapis/gnostic v0.5.5 // indirect
	github.com/hashicorp/golang-lru v0.5.4 // indirect
	github.com/imdario/mergo v0.3.12 // indirect
	github.com/inconshreveable/mousetrap v1.0.0 // indirect
	github.com/jpillora/backoff v1.0.0 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.2-0.20181231171920-c182affec369 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/nxadm/tail v1.4.8 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/prometheus/client_golang v1.11.0 // indirect
	github.com/prometheus/client_model v0.2.0 // indirect
	github.com/prometheus/common v0.26.0 // indirect
	github.com/prometheus/procfs v0.6.0 // indirect
	github.com/tjfoc/gmsm v1.3.2 // indirect
	go.uber.org/atomic v1.7.0 // indirect
	go.uber.org/multierr v1.6.0 // indirect
	go.uber.org/zap v1.19.0 // indirect
	golang.org/x/net v0.0.0-20220127200216-cd36cc0744dd // indirect
	golang.org/x/oauth2 v0.0.0-20200107190931-bf48bf16ab8d // indirect
	golang.org/x/sys v0.0.0-20220209214540-3681064d5158 // indirect
	golang.org/x/term v0.0.0-20210927222741-03fcf44c2211 // indirect
	golang.org/x/text v0.3.7 // indirect
	golang.org/x/time v0.0.0-20210723032227-1f47c861a9ac // indirect
	gomodules.xyz/jsonpatch/v2 v2.2.0 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/protobuf v1.27.1 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/ini.v1 v1.56.0 // indirect
	gopkg.in/tomb.v1 v1.0.0-20141024135613-dd632973f1e7 // indirect
	k8s.io/apiextensions-apiserver v0.21.4 // indirect
	k8s.io/component-base v0.21.4 // indirect
	k8s.io/klog/v2 v2.60.1 // indirect
	k8s.io/kube-openapi v0.0.0-20220328201542-3ee0da9b0b42 // indirect
	k8s.io/utils v0.0.0-20220210201930-3a6ce19ff2f9 // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.2.1 // indirect
)

replace (
	github.com/go-logr/logr => github.com/go-logr/logr v0.4.0
	github.com/googleapis/gnostic v0.5.5 => github.com/googleapis/gnostic v0.4.1
	github.com/miekg/dns v1.0.14 => github.com/miekg/dns v1.1.47
	google.golang.org/protobuf => google.golang.org/protobuf v1.23.0
	k8s.io/klog/v2 => k8s.io/klog/v2 v2.9.0
	k8s.io/kube-openapi => k8s.io/kube-openapi v0.0.0-20210305001622-591a79e4bda7
	sigs.k8s.io/controller-runtime => sigs.k8s.io/controller-runtime v0.9.7
)
