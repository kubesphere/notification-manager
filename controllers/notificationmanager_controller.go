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
	"reflect"

	"github.com/go-logr/logr"
	nmv1alpha1 "github.com/kubesphere/notification-manager/api/v1alpha1"
	operator "github.com/kubesphere/notification-manager/pkg/operator"
	commonerrors "github.com/pkg/errors"
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

var (
	ownerKey = ".metadata.controller"
	apiGVStr = nmv1alpha1.GroupVersion.String()
	log      logr.Logger
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
		// Check if the NotificationManager Deployment already exists
		old := &appsv1.Deployment{}
		oldName := types.NamespacedName{Namespace: nm.Namespace, Name: nm.Name + "-deployment"}
		err := r.Get(ctx, oldName, old)

		// Check if the reconcile is triggered by a NotificationManager CR
		// or config CRs like Receivers, EmailConfigs etc.
		newName := types.NamespacedName{Namespace: nm.Namespace, Name: nm.Name}
		configChange := !reflect.DeepEqual(newName, req.NamespacedName)

		switch {
		// Found deployment for NotificationManager CR
		// Update the deployment if the reconcile is not triggered by config CRs
		case err == nil:
			if err := r.update(ctx, &nm, old, configChange); err != nil {
				log.Error(err, "Failed to update Notification Manager")
				return ctrl.Result{}, err
			}
		// Cannot found deployment for NotificationManager CR
		// Create one if the reconcile is not triggered by config CRs
		case errors.IsNotFound(err) && !configChange:
			if err := r.create(ctx, &nm); err != nil {
				log.Error(err, "Failed to create Notification Manager")
				return ctrl.Result{}, err
			}
		// Cannot found deployment for NotificationManager CR
		// Ignore config CRs changes
		case errors.IsNotFound(err) && configChange:
			return ctrl.Result{}, nil
		default:
			log.Error(err, "Failed to get Notification Manager deployment:"+oldName.Namespace+"/"+oldName.Name)
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *NotificationManagerReconciler) create(ctx context.Context, nm *nmv1alpha1.NotificationManager) error {
	err := r.createConfigMap(ctx, nm)
	if err != nil {
		log.Error(err, "Unable to create ConfigMap")
	}

	deploy, err := operator.MakeDeployment(*nm, nil)
	if err != nil {
		log.Error(err, "Make deployment failed")
		return commonerrors.Wrap(err, "Make deployment failed")
	}
	if err := ctrl.SetControllerReference(nm, deploy, r.Scheme); err != nil {
		log.Error(err, "SetControllerReference failed for deployment")
		return err
	}
	if err := r.Create(ctx, deploy); err != nil && !errors.IsAlreadyExists(err) {
		log.Error(err, "Create deployment failed")
		return err
	}

	svc := operator.MakeDeploymentService(*nm)
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

func (r *NotificationManagerReconciler) update(ctx context.Context, nm *nmv1alpha1.NotificationManager, old *appsv1.Deployment, configChanged bool) error {
	if configChanged {
		err := r.updateConfigMap(ctx, nm)
		if err != nil {
			log.Error(err, "Unable to update ConfigMap")
		}
		return err
	}

	deploy, err := operator.MakeDeployment(*nm, old)
	if err != nil {
		log.Error(err, "Make deployment failed")
		return commonerrors.Wrap(err, "Make deployment failed")
	}

	if err := ctrl.SetControllerReference(nm, deploy, r.Scheme); err != nil {
		log.Error(err, "SetControllerReference failed for deployment")
		return err
	}

	// Update the found object and write the result back if there are any changes
	if !reflect.DeepEqual(deploy.Spec, old.Spec) {
		log.V(10).Info("Updating deployment", "New Spec:", deploy.Spec, "Old Spec:", old.Spec)
		err = r.Update(ctx, deploy)
		if err != nil {
			log.Error(err, "Update deployment failed")
			return err
		}
	}

	return nil
}

func (r *NotificationManagerReconciler) createConfigMap(ctx context.Context, nm *nmv1alpha1.NotificationManager) error {
	cm := operator.MakeConfigMap(*nm)
	if err := ctrl.SetControllerReference(nm, cm, r.Scheme); err != nil {
		log.Error(err, "SetControllerReference failed for ConfigMap")
		return err
	}
	if err := r.Create(ctx, cm); err != nil && !errors.IsAlreadyExists(err) {
		log.Error(err, "Create ConfigMap failed")
		return err
	}
	return nil
}

func (r *NotificationManagerReconciler) updateConfigMap(ctx context.Context, nm *nmv1alpha1.NotificationManager) error {
	cm := operator.MakeConfigMap(*nm)
	if err := ctrl.SetControllerReference(nm, cm, r.Scheme); err != nil {
		log.Error(err, "SetControllerReference failed for ConfigMap")
		return err
	}
	// Update the found object and write the result back if there are any changes
	err := r.Update(ctx, cm)
	if err != nil {
		log.Error(err, "Update ConfigMap failed")
		return err
	}
	return nil
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
		// Owns(&corev1.Service{}).
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
