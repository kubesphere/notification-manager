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
	"k8s.io/apimachinery/pkg/util/intstr"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"time"

	"github.com/go-logr/logr"
	nmv1alpha1 "github.com/kubesphere/notification-manager/pkg/apis/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	notificationManager                = "notification-manager"
	notificationManagerConfig          = "notification-manager-config"
	notificationManagerConfigMountPath = "/etc/notification-manager/config"
	defaultPortName                    = "webhook"
	defaultServiceAccountName          = "default"
)

var (
	ownerKey    = ".metadata.controller"
	apiGVStr    = nmv1alpha1.GroupVersion.String()
	log         logr.Logger
	minReplicas int32  = 1
	image       string = "kubesphere/notification-manager:v0.1.0"
)

// NotificationManagerReconciler reconciles a NotificationManager object
type NotificationManagerReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// Reconcile reads that state of NotificationManager objects and makes changes based on the state read
// and what is in the NotificationManagerSpec
// +kubebuilder:rbac:groups=notification.kubesphere.io,resources=notificationmanagers;receivers;emailconfigs;emailreceivers;webhookconfigs;webhookreceivers;wechatconfigs;wechatreceivers;slackconfigs;slackreceivers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=notification.kubesphere.io,resources=notificationmanagers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=services;configmaps,verbs=get;list;watch;create;update;patch;delete

func (r *NotificationManagerReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log = r.Log.WithValues("NotificationManager Operator", req.NamespacedName)

	var nms nmv1alpha1.NotificationManagerList
	if err := r.List(ctx, &nms); err != nil {
		log.Error(err, "Unable to list NotificationManager")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	for _, nm := range nms.Items {
		var err error
		result := controllerutil.OperationResultNone
		// Check if the reconcile is triggered by a NotificationManager CR
		// or config CRs like Receivers, EmailConfigs etc.
		newName := types.NamespacedName{Namespace: nm.Namespace, Name: nm.Name}
		configChanged := !reflect.DeepEqual(newName, req.NamespacedName)
		// Create or update configmap
		cm := &corev1.ConfigMap{}
		cm.ObjectMeta.Name = nm.Name + "-config"
		cm.ObjectMeta.Namespace = nm.Namespace
		if result, err = controllerutil.CreateOrUpdate(ctx, r.Client, cm, r.mutateConfigMap(cm, configChanged, &nm)); err != nil {
			log.Error(err, "Failed to CreateOrUpdate configmap", "result", result)
			return ctrl.Result{}, err
		}
		log.V(10).Info("CreateOrUpdate configmap returns", "result", result)

		if configChanged {
			continue
		}

		// Create deployment service
		if err = r.createDeploymentSvc(ctx, &nm); err != nil {
			log.Error(err, "Failed to create svc")
			return ctrl.Result{}, err
		}

		// Create or update deployment
		result = controllerutil.OperationResultNone
		deploy := &appsv1.Deployment{}
		deploy.ObjectMeta.Name = nm.Name + "-deployment"
		deploy.ObjectMeta.Namespace = nm.Namespace
		if result, err = controllerutil.CreateOrUpdate(ctx, r.Client, deploy, r.mutateDeployment(deploy, cm, &nm)); err != nil {
			log.Error(err, "Failed to CreateOrUpdate deployment", "result", result)
			return ctrl.Result{}, err
		}
		log.V(10).Info("CreateOrUpdate deployment returns", "result", result)
	}

	return ctrl.Result{}, nil
}

func (r *NotificationManagerReconciler) createDeploymentSvc(ctx context.Context, nm *nmv1alpha1.NotificationManager) error {
	nm = nm.DeepCopy()
	if nm.Spec.PortName == "" {
		nm.Spec.PortName = defaultPortName
	}

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nm.Name + "-svc",
			Namespace: nm.Namespace,
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

func (r *NotificationManagerReconciler) mutateDeployment(deploy *appsv1.Deployment, cm *corev1.ConfigMap, nm *nmv1alpha1.NotificationManager) controllerutil.MutateFn {
	return func() error {
		nm = nm.DeepCopy()

		if (nm.Spec.Image == nil) || (nm.Spec.Image != nil && *nm.Spec.Image == "") {
			nm.Spec.Image = &image
		}

		if (nm.Spec.Replicas == nil) || (nm.Spec.Replicas != nil && *nm.Spec.Replicas <= int32(0)) {
			nm.Spec.Replicas = &minReplicas
		}

		if nm.Spec.PortName == "" {
			nm.Spec.PortName = defaultPortName
		}

		if nm.Spec.ServiceAccountName == "" {
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

		deploy.Spec.Template.Spec.ServiceAccountName = nm.Spec.ServiceAccountName

		// Define configmap volume mounts
		volumeMounts := []corev1.VolumeMount{
			{
				Name:      notificationManagerConfig,
				ReadOnly:  true,
				MountPath: notificationManagerConfigMountPath,
			},
		}

		// Define expected container
		newC := corev1.Container{
			Name:            "notification-manager",
			Image:           *nm.Spec.Image,
			ImagePullPolicy: "Always",
			Ports: []corev1.ContainerPort{
				{
					Name:          nm.Spec.PortName,
					ContainerPort: 19093,
					Protocol:      corev1.ProtocolTCP,
				},
			},
			VolumeMounts: volumeMounts,
		}

		// Make sure existing Containers match expected Containers
		for i, c := range deploy.Spec.Template.Spec.Containers {
			if c.Name == newC.Name {
				deploy.Spec.Template.Spec.Containers[i].Image = newC.Image
				deploy.Spec.Template.Spec.Containers[i].ImagePullPolicy = newC.ImagePullPolicy
				deploy.Spec.Template.Spec.Containers[i].Ports = newC.Ports
				deploy.Spec.Template.Spec.Containers[i].VolumeMounts = newC.VolumeMounts
				break
			}
		}

		// Create new Containers if no existing Containers exist
		if len(deploy.Spec.Template.Spec.Containers) == 0 {
			deploy.Spec.Template.Spec.Containers = []corev1.Container{newC}
		}

		// Define volume for ConfigMap
		newVol := corev1.Volume{
			Name: notificationManagerConfig,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: cm.ObjectMeta.Name,
					},
				},
			},
		}

		// Make sure existing volumes match expected volumes
		for i, v := range deploy.Spec.Template.Spec.Volumes {
			if v.Name == newVol.Name {
				if v.ConfigMap != nil {
					deploy.Spec.Template.Spec.Volumes[i].ConfigMap.LocalObjectReference =
						newVol.ConfigMap.LocalObjectReference
				}
				break
			}
		}

		// Create new volumes if no existing volumes exist
		if len(deploy.Spec.Template.Spec.Volumes) == 0 {
			deploy.Spec.Template.Spec.Volumes = []corev1.Volume{newVol}
		}

		deploy.SetOwnerReferences(nil)
		return ctrl.SetControllerReference(nm, deploy, r.Scheme)
	}
}

func (r *NotificationManagerReconciler) mutateConfigMap(cm *corev1.ConfigMap, configChanged bool, nm *nmv1alpha1.NotificationManager) controllerutil.MutateFn {
	return func() error {
		cm.ObjectMeta.Labels = *r.makeCommonLabels(nm)
		if configChanged {
			cm.Data = map[string]string{"UpdateTime": time.Now().String()}
		}
		cm.SetOwnerReferences(nil)
		return ctrl.SetControllerReference(nm, cm, r.Scheme)
	}
}

func (r *NotificationManagerReconciler) makeCommonLabels(nm *nmv1alpha1.NotificationManager) *map[string]string {
	return &map[string]string{"app": notificationManager, notificationManager: nm.Name}
}

func (r *NotificationManagerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := mgr.GetFieldIndexer().IndexField(&corev1.Service{}, ownerKey, func(rawObj runtime.Object) []string {
		// grab the job object, extract the owner.
		svc := rawObj.(*corev1.Service)
		owner := metav1.GetControllerOf(svc)
		if owner == nil {
			return nil
		}
		// Make sure it's a FluentBit. If so, return it.
		if owner.APIVersion != apiGVStr || owner.Kind != "NotificationManager" {
			return nil
		}
		return []string{owner.Name}
	}); err != nil {
		return err
	}

	if err := mgr.GetFieldIndexer().IndexField(&appsv1.Deployment{}, ownerKey, func(rawObj runtime.Object) []string {
		// grab the job object, extract the owner.
		deploy := rawObj.(*appsv1.Deployment)
		owner := metav1.GetControllerOf(deploy)
		if owner == nil {
			return nil
		}
		// Make sure it's a FluentBit. If so, return it.
		if owner.APIVersion != apiGVStr || owner.Kind != "NotificationManager" {
			return nil
		}
		return []string{owner.Name}
	}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&nmv1alpha1.NotificationManager{}).
		Owns(&appsv1.Deployment{}).
		Watches(&source.Kind{Type: &nmv1alpha1.Receiver{}}, &handler.EnqueueRequestForObject{}).
		Watches(&source.Kind{Type: &nmv1alpha1.EmailConfig{}}, &handler.EnqueueRequestForObject{}).
		Watches(&source.Kind{Type: &nmv1alpha1.EmailReceiver{}}, &handler.EnqueueRequestForObject{}).
		Watches(&source.Kind{Type: &nmv1alpha1.WebhookConfig{}}, &handler.EnqueueRequestForObject{}).
		Watches(&source.Kind{Type: &nmv1alpha1.WebhookReceiver{}}, &handler.EnqueueRequestForObject{}).
		Watches(&source.Kind{Type: &nmv1alpha1.WechatConfig{}}, &handler.EnqueueRequestForObject{}).
		Watches(&source.Kind{Type: &nmv1alpha1.WechatReceiver{}}, &handler.EnqueueRequestForObject{}).
		Watches(&source.Kind{Type: &nmv1alpha1.SlackConfig{}}, &handler.EnqueueRequestForObject{}).
		Watches(&source.Kind{Type: &nmv1alpha1.SlackReceiver{}}, &handler.EnqueueRequestForObject{}).
		Complete(r)
}
