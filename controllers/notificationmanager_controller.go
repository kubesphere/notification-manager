/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/kubesphere/notification-manager/pkg/apis/v2beta2"
	"github.com/kubesphere/notification-manager/pkg/utils"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	notificationManager           = "notification-manager"
	defaultPortName               = "webhook"
	defaultServiceAccountName     = "default"
	kubesphereSidecar             = "kubesphere"
	defaultkubesphereSidecarImage = "kubesphere/kubesphere-sidecar:v3.1.0"
)

var (
	ownerKey               = ".metadata.controller"
	apiGVStr               = v2beta2.GroupVersion.String()
	log                    logr.Logger
	minReplicas            int32 = 1
	defaultImage                 = "kubesphere/notification-manager:v1.0.0"
	defaultImagePullPolicy       = corev1.PullIfNotPresent
)

// NotificationManagerReconciler reconciles a NotificationManager object
type NotificationManagerReconciler struct {
	client.Client
	Namespace string
	Log       logr.Logger
	Scheme    *runtime.Scheme
}

// Reconcile reads that state of NotificationManager objects and makes changes based on the state read
// and what is in the NotificationManagerSpec
// +kubebuilder:rbac:groups=notification.kubesphere.io,resources=notificationmanagers;receivers;configs,routers,silences,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=notification.kubesphere.io,resources=notificationmanagers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=get;list;watch;

func (r *NotificationManagerReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log = r.Log.WithValues("NotificationManager Operator", req.NamespacedName)

	var nm v2beta2.NotificationManager
	if err := r.Get(ctx, req.NamespacedName, &nm); err != nil {
		log.Error(err, "Unable to get NotificationManager", "Req", req.NamespacedName.String())
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	var err error
	result := controllerutil.OperationResultNone

	// Create deployment service
	if err = r.createDeploymentSvc(ctx, &nm); err != nil {
		log.Error(err, "Failed to create svc")
		return ctrl.Result{}, err
	}

	// Create or update deployment
	result = controllerutil.OperationResultNone
	deploy := &appsv1.Deployment{}
	deploy.ObjectMeta.Name = nm.Name + "-deployment"
	deploy.ObjectMeta.Namespace = r.Namespace
	if result, err = controllerutil.CreateOrUpdate(ctx, r.Client, deploy, r.mutateDeployment(deploy, &nm)); err != nil {
		log.Error(err, "Failed to CreateOrUpdate deployment", "result", result)
		return ctrl.Result{}, err
	}
	log.V(10).Info("CreateOrUpdate deployment returns", "result", result)

	return ctrl.Result{}, nil
}

func (r *NotificationManagerReconciler) createDeploymentSvc(ctx context.Context, nm *v2beta2.NotificationManager) error {
	nm = nm.DeepCopy()
	if utils.StringIsNil(nm.Spec.PortName) {
		nm.Spec.PortName = defaultPortName
	}

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nm.Name + "-svc",
			Namespace: r.Namespace,
			Labels:    *r.makeCommonLabels(nm),
		},
		Spec: corev1.ServiceSpec{
			Type: "ClusterIP",
			Ports: []corev1.ServicePort{
				{
					Name:       nm.Spec.PortName,
					Port:       19093,
					TargetPort: intstr.FromString(nm.Spec.PortName),
				},
			},
			Selector: *r.makeCommonLabels(nm),
		},
	}

	if err := ctrl.SetControllerReference(nm, svc, r.Scheme); err != nil {
		log.Error(err, "SetControllerReference failed for service")
		return err
	}
	if err := r.Create(ctx, svc); err != nil && !errors.IsAlreadyExists(err) {
		log.Error(err, "Create service failed")
		return err
	}

	return nil
}

func (r *NotificationManagerReconciler) mutateDeployment(deploy *appsv1.Deployment, nm *v2beta2.NotificationManager) controllerutil.MutateFn {
	return func() error {
		nm = nm.DeepCopy()

		if nm.Spec.Image == nil || nm.Spec.Image != nil && utils.StringIsNil(*nm.Spec.Image) {
			nm.Spec.Image = &defaultImage
		}

		if nm.Spec.ImagePullPolicy == nil || nm.Spec.ImagePullPolicy != nil && *nm.Spec.ImagePullPolicy == "" {
			nm.Spec.ImagePullPolicy = &defaultImagePullPolicy
		}

		if nm.Spec.Replicas == nil || nm.Spec.Replicas != nil && *nm.Spec.Replicas <= int32(0) {
			nm.Spec.Replicas = &minReplicas
		}

		if utils.StringIsNil(nm.Spec.PortName) {
			nm.Spec.PortName = defaultPortName
		}

		if utils.StringIsNil(nm.Spec.ServiceAccountName) {
			nm.Spec.ServiceAccountName = defaultServiceAccountName
		}

		deploy.ObjectMeta.Labels = *r.makeCommonLabels(nm)
		deploy.Spec.Replicas = nm.Spec.Replicas
		podLabels := deploy.ObjectMeta.Labels
		deploy.Spec.Selector = &metav1.LabelSelector{
			MatchLabels: podLabels,
		}
		deploy.Spec.Template.ObjectMeta = metav1.ObjectMeta{
			Labels: podLabels,
		}
		deploy.Spec.Template.Spec.NodeSelector = nm.Spec.NodeSelector
		deploy.Spec.Template.Spec.Affinity = nm.Spec.Affinity
		deploy.Spec.Template.Spec.Tolerations = nm.Spec.Tolerations
		deploy.Spec.Template.Spec.ServiceAccountName = nm.Spec.ServiceAccountName

		// Define expected container
		newC := corev1.Container{
			Name:            notificationManager,
			Resources:       nm.Spec.Resources,
			Image:           *nm.Spec.Image,
			ImagePullPolicy: *nm.Spec.ImagePullPolicy,
			Ports: []corev1.ContainerPort{
				{
					Name:          nm.Spec.PortName,
					ContainerPort: 19093,
					Protocol:      corev1.ProtocolTCP,
				},
			},
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      "host-time",
					MountPath: "/etc/localtime",
					ReadOnly:  true,
				},
			},
		}

		if utils.StringIsNil(nm.Spec.DefaultSecretNamespace) {
			newC.Env = []corev1.EnvVar{
				{
					Name: "NAMESPACE",
					ValueFrom: &corev1.EnvVarSource{
						FieldRef: &corev1.ObjectFieldSelector{
							FieldPath: "metadata.namespace",
						},
					},
				},
			}
		} else {
			newC.Env = []corev1.EnvVar{
				{
					Name:  "NAMESPACE",
					Value: nm.Spec.DefaultSecretNamespace,
				},
			}
		}

		if nm.Spec.VolumeMounts != nil {
			newC.VolumeMounts = append(newC.VolumeMounts, nm.Spec.VolumeMounts...)
		}

		if nm.Spec.Args != nil {
			newC.Args = append(newC.Args, nm.Spec.Args...)
		}

		deploy.Spec.Template.Spec.Containers = []corev1.Container{newC}

		if sidecar := r.mutateTenantSidecar(nm); sidecar != nil {
			deploy.Spec.Template.Spec.Containers = append(deploy.Spec.Template.Spec.Containers, *sidecar)
		}

		deploy.Spec.Template.Spec.Volumes = []corev1.Volume{
			{
				Name: "host-time",
				VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{
						Path: "/etc/localtime",
					},
				},
			},
		}
		deploy.Spec.Template.Spec.Volumes = append(deploy.Spec.Template.Spec.Volumes, nm.Spec.Volumes...)

		deploy.SetOwnerReferences(nil)
		return ctrl.SetControllerReference(nm, deploy, r.Scheme)
	}
}

func (r *NotificationManagerReconciler) mutateTenantSidecar(nm *v2beta2.NotificationManager) *corev1.Container {

	if nm.Spec.Sidecars == nil {
		return nil
	}

	sidecar, ok := nm.Spec.Sidecars[v2beta2.Tenant]
	if !ok || sidecar == nil {
		return nil
	}

	if sidecar.Type == kubesphereSidecar {
		return r.generateKubesphereSidecar(sidecar)
	}

	return sidecar.Container
}

func (r *NotificationManagerReconciler) generateKubesphereSidecar(sidecar *v2beta2.Sidecar) *corev1.Container {

	container := sidecar.Container
	if container == nil {
		container = &corev1.Container{
			Name:            "tenant-sidecar",
			ImagePullPolicy: "IfNotPresent",
		}
	}

	if utils.StringIsNil(container.Image) {
		container.Image = defaultkubesphereSidecarImage
	}

	if container.Ports == nil || len(container.Ports) == 0 {
		container.Ports = []corev1.ContainerPort{
			{
				Name:          "tenant",
				ContainerPort: 19094,
				Protocol:      corev1.ProtocolTCP,
			},
		}
	}

	container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{
		Name:      "host-time",
		MountPath: "/etc/localtime",
		ReadOnly:  true,
	})
	return container
}

func (r *NotificationManagerReconciler) makeCommonLabels(nm *v2beta2.NotificationManager) *map[string]string {
	return &map[string]string{"app": notificationManager, notificationManager: nm.Name}
}

func (r *NotificationManagerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := mgr.GetFieldIndexer().IndexField(&corev1.Service{}, ownerKey, func(rawObj runtime.Object) []string {
		// grab the service object, extract the owner.
		svc := rawObj.(*corev1.Service)
		owner := metav1.GetControllerOf(svc)
		if owner == nil {
			return nil
		}
		// Make sure it's a NotificationManager. If so, return it.
		if owner.APIVersion != apiGVStr || owner.Kind != "NotificationManager" {
			return nil
		}
		return []string{owner.Name}
	}); err != nil {
		return err
	}

	if err := mgr.GetFieldIndexer().IndexField(&appsv1.Deployment{}, ownerKey, func(rawObj runtime.Object) []string {
		// grab the deployment object, extract the owner.
		deploy := rawObj.(*appsv1.Deployment)
		owner := metav1.GetControllerOf(deploy)
		if owner == nil {
			return nil
		}
		// Make sure it's a NotificationManager. If so, return it.
		if owner.APIVersion != apiGVStr || owner.Kind != "NotificationManager" {
			return nil
		}
		return []string{owner.Name}
	}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v2beta2.NotificationManager{}).
		Owns(&appsv1.Deployment{}).
		Complete(r)
}
