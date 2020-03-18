package operator

import (
	nmv1alpha1 "github.com/kubesphere/notification-manager/api/v1alpha1"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"time"
)

const (
	defaultPortName           = "webhook"
	defaultServiceAccountName = "default"
)

var (
	minReplicas int32  = 1
	image       string = "kubesphere/notification-manager:v0.1.0"
)

func MakeDeployment(nm nmv1alpha1.NotificationManager) (*appsv1.Deployment, error) {
	nm = *nm.DeepCopy()

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

	spec, err := makeDeploymentSpec(nm)
	if err != nil {
		return nil, errors.Wrap(err, "make Deployment spec")
	}

	// Define the desired NotificationManager Deployment object
	deploy := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nm.Name + "-deployment",
			Namespace: nm.Namespace,
			Labels:    nm.ObjectMeta.Labels,
		},
		Spec: *spec,
	}

	return &deploy, nil
}

func makeDeploymentSpec(nm nmv1alpha1.NotificationManager) (*appsv1.DeploymentSpec, error) {
	nm = *nm.DeepCopy()

	podLabels := map[string]string{}
	podLabels["app"] = "notification-manager"
	podLabels["notification-manager"] = nm.Name

	// define volume for ConfigMap
	volumes := []corev1.Volume{
		{
			Name: "notification-manager-config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: nm.Name + "-config",
					},
				},
			},
		},
	}

	volumeMounts := []corev1.VolumeMount{
		{
			Name:      "notification-manager-config",
			ReadOnly:  true,
			MountPath: "/etc/notification-manager/config",
		},
	}

	// Define the desired NotificationManager Deployment object
	deploySpec := &appsv1.DeploymentSpec{
		Replicas: nm.Spec.Replicas,
		Selector: &metav1.LabelSelector{
			MatchLabels: podLabels,
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: podLabels,
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
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
					},
				},
				ServiceAccountName: nm.Spec.ServiceAccountName,
				Volumes:            volumes,
			},
		},
	}
	return deploySpec, nil
}

func MakeDeploymentService(nm nmv1alpha1.NotificationManager) *corev1.Service {
	nm = *nm.DeepCopy()

	if nm.Spec.PortName == "" {
		nm.Spec.PortName = defaultPortName
	}

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nm.Name + "-svc",
			Namespace: nm.Namespace,
			Labels:    map[string]string{"app": "notification-manager"},
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
			Selector: map[string]string{
				"app":                  "notification-manager",
				"notification-manager": nm.Name,
			},
		},
	}
	return svc
}

func MakeConfigMap(nm nmv1alpha1.NotificationManager) *corev1.ConfigMap {
	nm = *nm.DeepCopy()
	data := map[string]string{"UpdateTime": time.Now().String()}
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nm.Name + "-config",
			Namespace: nm.Namespace,
			Labels:    map[string]string{"app": "notification-manager"},
		},
		Data: data,
	}
}
