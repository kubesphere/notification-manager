module kubesphere

go 1.13

require (
	github.com/emicklei/go-restful v2.16.0+incompatible
	github.com/spf13/cobra v1.1.1
	github.com/spf13/pflag v1.0.5
	k8s.io/api v0.19.3
	k8s.io/apimachinery v0.19.3
	k8s.io/apiserver v0.19.3
	k8s.io/client-go v12.0.0+incompatible
	k8s.io/klog v1.0.0
	kubesphere.io/kubesphere v0.0.0-20210430091105-1f57ec2e38f2
	sigs.k8s.io/controller-runtime v0.6.4
)

replace (
	github.com/googleapis/gnostic => github.com/googleapis/gnostic v0.4.0
	k8s.io/api => k8s.io/api v0.18.6
	k8s.io/apiserver => k8s.io/apiserver v0.18.6
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.18.6
	k8s.io/client-go => k8s.io/client-go v0.18.6
	k8s.io/kube-openapi => k8s.io/kube-openapi v0.0.0-20200410145947-61e04a5be9a6
	k8s.io/kubectl => k8s.io/kubectl v0.18.6
	kubesphere.io/client-go v0.0.0 => kubesphere.io/client-go v0.3.1
)
