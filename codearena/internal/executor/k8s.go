package executor

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	pollInterval = 2 * time.Second
	// resultPrefix marks the runner's final verdict line in the pod log.
	resultPrefix = "__CODEARENA_RESULT__"
	// jobDeadlineSeconds is the pod-level hard cap (compile + run + slack).
	jobDeadlineSeconds = int64(60)
)

// K8sExecutor runs user programs inside Kubernetes Jobs using the runner
// image. Runner contract (/run.sh): stream compile errors and program output
// to the pod log as raw lines in real time, then print a final line
// "__CODEARENA_RESULT__{...ExecResult JSON...}".
type K8sExecutor struct {
	client    kubernetes.Interface
	namespace string
	image     string
}

// NewK8sExecutor builds a client from in-cluster config, falling back to the
// local kubeconfig for out-of-cluster development.
func NewK8sExecutor(namespace, image string) (*K8sExecutor, error) {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
		cfg, err = clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
			loadingRules, &clientcmd.ConfigOverrides{}).ClientConfig()
		if err != nil {
			return nil, fmt.Errorf("no in-cluster config and no kubeconfig: %w", err)
		}
	}
	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("build k8s client: %w", err)
	}
	return &K8sExecutor{client: client, namespace: namespace, image: image}, nil
}

// Execute creates a ConfigMap with the code, launches a Job with the runner
// image, follows the pod log streaming each line through emit, and parses
// the final result line. Any infrastructure failure surfaces as an error
// (mapped to internal_error by the caller).
func (e *K8sExecutor) Execute(ctx context.Context, req ExecRequest, emit EmitFunc) (ExecResult, error) {
	name := "run-" + randomSuffix()

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: map[string]string{"app": "codearena-run"},
		},
		Data: map[string]string{"main.go": req.Code},
	}
	if _, err := e.client.CoreV1().ConfigMaps(e.namespace).Create(ctx, cm, metav1.CreateOptions{}); err != nil {
		return ExecResult{}, fmt.Errorf("create configmap: %w", err)
	}
	// Always clean up, even on failure paths. Uses a background context so
	// cleanup still runs when ctx is already cancelled.
	defer e.cleanup(name)

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: map[string]string{"app": "codearena-run"},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:            ptr(int32(0)),
			TTLSecondsAfterFinished: ptr(int32(60)),
			ActiveDeadlineSeconds:   ptr(jobDeadlineSeconds),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "codearena-run"},
				},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{{
						Name:    "runner",
						Image:   e.image,
						Command: []string{"/run.sh"},
						Env: []corev1.EnvVar{{
							Name:  "TIME_LIMIT_MS",
							Value: strconv.Itoa(req.TimeLimitMS),
						}},
						Resources: corev1.ResourceRequirements{
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("500m"),
								corev1.ResourceMemory: resource.MustParse("256Mi"),
							},
						},
						VolumeMounts: []corev1.VolumeMount{{
							Name:      "input",
							MountPath: "/input",
							ReadOnly:  true,
						}},
					}},
					Volumes: []corev1.Volume{{
						Name: "input",
						VolumeSource: corev1.VolumeSource{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{Name: name},
							},
						},
					}},
				},
			},
		},
	}
	if _, err := e.client.BatchV1().Jobs(e.namespace).Create(ctx, job, metav1.CreateOptions{}); err != nil {
		return ExecResult{}, fmt.Errorf("create job: %w", err)
	}

	// Everything below runs under a hard deadline slightly above the pod's
	// activeDeadlineSeconds so a stuck pod can never wedge the worker.
	runCtx, cancel := context.WithTimeout(ctx, time.Duration(jobDeadlineSeconds+30)*time.Second)
	defer cancel()

	podName, err := e.waitForPod(runCtx, name)
	if err != nil {
		return ExecResult{}, err
	}

	return e.streamLogs(runCtx, podName, emit)
}

// waitForPod polls until the Job's pod exists and has started (or already
// finished), so its log can be followed.
func (e *K8sExecutor) waitForPod(ctx context.Context, jobName string) (string, error) {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		pods, err := e.client.CoreV1().Pods(e.namespace).List(ctx, metav1.ListOptions{
			LabelSelector: "job-name=" + jobName,
		})
		if err != nil {
			return "", fmt.Errorf("list job pods: %w", err)
		}
		for i := range pods.Items {
			pod := &pods.Items[i]
			switch pod.Status.Phase {
			case corev1.PodRunning, corev1.PodSucceeded, corev1.PodFailed:
				return pod.Name, nil
			}
		}
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("timed out waiting for run pod (job %s): %w", jobName, ctx.Err())
		case <-ticker.C:
		}
	}
}

// streamLogs follows the pod log, emitting every raw line as stdout except
// the final result marker line, which is parsed as the verdict.
func (e *K8sExecutor) streamLogs(ctx context.Context, podName string, emit EmitFunc) (ExecResult, error) {
	stream, err := e.client.CoreV1().Pods(e.namespace).
		GetLogs(podName, &corev1.PodLogOptions{Follow: true}).
		Stream(ctx)
	if err != nil {
		return ExecResult{}, fmt.Errorf("open pod log stream: %w", err)
	}
	defer stream.Close()

	sc := bufio.NewScanner(stream)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var (
		res       ExecResult
		gotResult bool
	)
	for sc.Scan() {
		line := sc.Text()
		if rest, ok := strings.CutPrefix(strings.TrimSpace(line), resultPrefix); ok {
			if err := json.Unmarshal([]byte(rest), &res); err != nil {
				return ExecResult{}, fmt.Errorf("parse result line: %w", err)
			}
			gotResult = true
			continue // never emit the marker line
		}
		emit(StreamStdout, line+"\n")
	}
	if err := sc.Err(); err != nil {
		return ExecResult{}, fmt.Errorf("read pod log stream: %w", err)
	}
	if !gotResult {
		// Pod died (OOM, activeDeadline, evicted...) before printing a verdict.
		return ExecResult{}, fmt.Errorf("pod log ended without a %s line", resultPrefix)
	}
	return res, nil
}

// cleanup deletes the Job (and its pods) and the ConfigMap. Best effort.
func (e *K8sExecutor) cleanup(name string) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	propagation := metav1.DeletePropagationBackground
	if err := e.client.BatchV1().Jobs(e.namespace).Delete(ctx, name, metav1.DeleteOptions{
		PropagationPolicy: &propagation,
	}); err != nil && !apierrors.IsNotFound(err) {
		slog.Warn("delete run job failed", "job", name, "error", err)
	}
	if err := e.client.CoreV1().ConfigMaps(e.namespace).Delete(ctx, name, metav1.DeleteOptions{}); err != nil &&
		!apierrors.IsNotFound(err) {
		slog.Warn("delete run configmap failed", "configmap", name, "error", err)
	}
}

func randomSuffix() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func ptr[T any](v T) *T { return &v }
