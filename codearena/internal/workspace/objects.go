package workspace

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// resourceName is the shared name for every Kubernetes object of a workspace.
// UUIDs are lowercase/DNS-safe; "ws-<uuid>" stays well under the 63-char limit.
func resourceName(id string) string { return "ws-" + id }

// previewHost builds the public hostname for a workspace's preview URL,
// e.g. "ws-<id>.preview.<ip>.nip.io".
func (m *Manager) previewHost(id string) string {
	return resourceName(id) + "." + m.previewBase
}

// labels returns the identifying labels stamped on every object of a workspace.
func labels(id string, userID int64) map[string]string {
	return map[string]string{
		"app.kubernetes.io/managed-by": "codearena",
		"app.kubernetes.io/component":  "workspace",
		"codearena.dev/workspace":      id,
		"codearena.dev/owner":          itoa(userID),
	}
}

func (m *Manager) pvc(id string, userID int64) *corev1.PersistentVolumeClaim {
	name := resourceName(id)
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: m.namespace, Labels: labels(id, userID)},
		Spec: corev1.PersistentVolumeClaimSpec{
			// RWO is fine: a workspace is single-writer, one pod at a time.
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Gi")},
			},
			// No storageClassName => the cluster default (local-path on this cluster).
		},
	}
}

func (m *Manager) deployment(id string, userID int64, image string, replicas int32) *appsv1.Deployment {
	name := resourceName(id)
	lbls := labels(id, userID)
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: m.namespace, Labels: lbls},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			// One workspace = one pod holding the PVC (RWO). Recreate avoids two
			// pods contending for the same volume during a rollout.
			Strategy: appsv1.DeploymentStrategy{Type: appsv1.RecreateDeploymentStrategyType},
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"codearena.dev/workspace": id}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: lbls},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:       "workspace",
						Image:      image,
						WorkingDir: "/workspace",
						// The image's ENTRYPOINT is the agent (built in P2); it serves
						// fs/term/run on AGENT_PORT and hosts the user's shell + processes.
						Env: []corev1.EnvVar{
							{Name: "AGENT_PORT", Value: itoa(int64(m.agentPort))},
							{Name: "PREVIEW_PORT", Value: itoa(int64(m.previewPort))},
						},
						Ports: []corev1.ContainerPort{
							{Name: "agent", ContainerPort: m.agentPort},
							{Name: "preview", ContainerPort: m.previewPort},
						},
						VolumeMounts: []corev1.VolumeMount{{Name: "workspace", MountPath: "/workspace"}},
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("100m"),
								corev1.ResourceMemory: resource.MustParse("128Mi"),
							},
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("1"),
								corev1.ResourceMemory: resource.MustParse("1Gi"),
							},
						},
					}},
					Volumes: []corev1.Volume{{
						Name: "workspace",
						VolumeSource: corev1.VolumeSource{
							PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: name},
						},
					}},
				},
			},
		},
	}
}

func (m *Manager) service(id string, userID int64) *corev1.Service {
	name := resourceName(id)
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: m.namespace, Labels: labels(id, userID)},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeClusterIP,
			Selector: map[string]string{"codearena.dev/workspace": id},
			Ports: []corev1.ServicePort{
				{Name: "agent", Port: m.agentPort, TargetPort: intstr.FromInt32(m.agentPort)},
				{Name: "preview", Port: m.previewPort, TargetPort: intstr.FromInt32(m.previewPort)},
			},
		},
	}
}

func (m *Manager) ingress(id string, userID int64) *networkingv1.Ingress {
	name := resourceName(id)
	className := "nginx"
	pathType := networkingv1.PathTypePrefix
	return &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: m.namespace,
			Labels:    labels(id, userID),
			Annotations: map[string]string{
				// Preview apps often use websockets/long polling.
				"nginx.ingress.kubernetes.io/proxy-read-timeout": "3600",
				"nginx.ingress.kubernetes.io/proxy-send-timeout": "3600",
			},
		},
		Spec: networkingv1.IngressSpec{
			IngressClassName: &className,
			Rules: []networkingv1.IngressRule{{
				Host: m.previewHost(id),
				IngressRuleValue: networkingv1.IngressRuleValue{
					HTTP: &networkingv1.HTTPIngressRuleValue{
						Paths: []networkingv1.HTTPIngressPath{{
							Path:     "/",
							PathType: &pathType,
							Backend: networkingv1.IngressBackend{
								Service: &networkingv1.IngressServiceBackend{
									Name: name,
									Port: networkingv1.ServiceBackendPort{Number: m.previewPort},
								},
							},
						}},
					},
				},
			}},
		},
	}
}
