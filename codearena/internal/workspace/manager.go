// Package workspace manages the Kubernetes objects that back a persistent
// user workspace (the "Repl"): a Deployment + PVC + Service + Ingress. It is the
// control-plane reconciler — analogous to Replit's conman/controlplane, but the
// Pod + kubelet do the actual container lifecycle.
package workspace

import (
	"context"
	"fmt"
	"strconv"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/amura/codearena/internal/config"
	"github.com/amura/codearena/internal/models"
)

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

// Manager creates and controls workspace Kubernetes objects.
type Manager struct {
	client         kubernetes.Interface
	namespace      string
	image          string
	previewBase    string
	agentPort      int32
	previewPort    int32
	previewURLPort int // external port appended to preview URLs (ingress NodePort); 0 = none
}

// NewManager builds a Manager from in-cluster config, falling back to the local
// kubeconfig for out-of-cluster development (same pattern as executor.K8s).
func NewManager(cfg config.Config) (*Manager, error) {
	rc, err := rest.InClusterConfig()
	if err != nil {
		rules := clientcmd.NewDefaultClientConfigLoadingRules()
		rc, err = clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
			rules, &clientcmd.ConfigOverrides{}).ClientConfig()
		if err != nil {
			return nil, fmt.Errorf("no in-cluster config and no kubeconfig: %w", err)
		}
	}
	client, err := kubernetes.NewForConfig(rc)
	if err != nil {
		return nil, fmt.Errorf("build k8s client: %w", err)
	}
	return &Manager{
		client:         client,
		namespace:      cfg.WorkspaceNamespace,
		image:          cfg.WorkspaceImage,
		previewBase:    cfg.PreviewBaseDomain,
		agentPort:      int32(cfg.WorkspaceAgentPort),
		previewPort:    int32(cfg.WorkspacePreviewPort),
		previewURLPort: cfg.PreviewURLPort,
	}, nil
}

// PreviewURL returns the public URL a workspace's server is reachable at. When
// the ingress is exposed via a NodePort (not port 80), that port is appended.
func (m *Manager) PreviewURL(id string) string {
	host := m.previewHost(id)
	if m.previewURLPort > 0 {
		return fmt.Sprintf("http://%s:%d", host, m.previewURLPort)
	}
	return "http://" + host
}

// AgentEndpoint returns the in-cluster address of a workspace's agent, used by
// the WS proxy (P3) to reach the pod. Resolves via the workspace Service DNS.
func (m *Manager) AgentEndpoint(id string) string {
	return fmt.Sprintf("%s.%s.svc.cluster.local:%d", resourceName(id), m.namespace, m.agentPort)
}

// Create provisions all objects for a new workspace (started, replicas=1).
// It is idempotent: AlreadyExists is treated as success so a retried create or
// a create-after-partial-failure converges.
func (m *Manager) Create(ctx context.Context, ws models.Workspace) error {
	image := ws.Image
	if image == "" {
		image = m.image
	}
	if err := m.createOrIgnore(ctx, "pvc", func() error {
		_, err := m.client.CoreV1().PersistentVolumeClaims(m.namespace).
			Create(ctx, m.pvc(ws.ID, ws.UserID), metav1.CreateOptions{})
		return err
	}); err != nil {
		return err
	}
	if err := m.createOrIgnore(ctx, "service", func() error {
		_, err := m.client.CoreV1().Services(m.namespace).
			Create(ctx, m.service(ws.ID, ws.UserID), metav1.CreateOptions{})
		return err
	}); err != nil {
		return err
	}
	if err := m.createOrIgnore(ctx, "ingress", func() error {
		_, err := m.client.NetworkingV1().Ingresses(m.namespace).
			Create(ctx, m.ingress(ws.ID, ws.UserID), metav1.CreateOptions{})
		return err
	}); err != nil {
		return err
	}
	if err := m.createOrIgnore(ctx, "deployment", func() error {
		_, err := m.client.AppsV1().Deployments(m.namespace).
			Create(ctx, m.deployment(ws.ID, ws.UserID, image, 1), metav1.CreateOptions{})
		return err
	}); err != nil {
		return err
	}
	return nil
}

// Start resumes a hibernated workspace (Deployment replicas -> 1).
func (m *Manager) Start(ctx context.Context, id string) error { return m.scale(ctx, id, 1) }

// Stop hibernates a workspace (Deployment replicas -> 0). The PVC is retained,
// so files survive; this is the cost-saving idle state.
func (m *Manager) Stop(ctx context.Context, id string) error { return m.scale(ctx, id, 0) }

func (m *Manager) scale(ctx context.Context, id string, replicas int32) error {
	patch := []byte(fmt.Sprintf(`{"spec":{"replicas":%d}}`, replicas))
	_, err := m.client.AppsV1().Deployments(m.namespace).
		Patch(ctx, resourceName(id), types.StrategicMergePatchType, patch, metav1.PatchOptions{})
	if apierrors.IsNotFound(err) {
		return fmt.Errorf("workspace %s deployment not found: %w", id, err)
	}
	return err
}

// Delete removes all Kubernetes objects for a workspace, including the PVC (and
// thus the files). Best-effort: NotFound is ignored so delete is idempotent.
func (m *Manager) Delete(ctx context.Context, id string) error {
	name := resourceName(id)
	del := func(kind string, fn func() error) error {
		if err := fn(); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("delete %s %s: %w", kind, name, err)
		}
		return nil
	}
	if err := del("deployment", func() error {
		return m.client.AppsV1().Deployments(m.namespace).Delete(ctx, name, metav1.DeleteOptions{})
	}); err != nil {
		return err
	}
	if err := del("ingress", func() error {
		return m.client.NetworkingV1().Ingresses(m.namespace).Delete(ctx, name, metav1.DeleteOptions{})
	}); err != nil {
		return err
	}
	if err := del("service", func() error {
		return m.client.CoreV1().Services(m.namespace).Delete(ctx, name, metav1.DeleteOptions{})
	}); err != nil {
		return err
	}
	if err := del("pvc", func() error {
		return m.client.CoreV1().PersistentVolumeClaims(m.namespace).Delete(ctx, name, metav1.DeleteOptions{})
	}); err != nil {
		return err
	}
	return nil
}

func (m *Manager) createOrIgnore(_ context.Context, kind string, fn func() error) error {
	if err := fn(); err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("create %s: %w", kind, err)
	}
	return nil
}
