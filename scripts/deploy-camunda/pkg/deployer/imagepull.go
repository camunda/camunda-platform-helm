// Copyright 2026 Camunda Services GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package deployer

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"

	"scripts/camunda-core/pkg/kube"
	"scripts/camunda-core/pkg/logging"
	"scripts/deploy-camunda/pkg/types"
)

// An unresolvable image reference is not something `helm --wait` can recover
// from: kubelet keeps retrying until the Helm timeout expires and the run ends
// as "context deadline exceeded", which names neither the image nor the reason.
// The guard below watches pods during the wait and aborts as soon as a pull
// failure is provably terminal, turning a 20-minute timeout into a ~1-minute
// failure that names the pod, container, image and registry error.
//
// It never turns a failing deploy into a passing one. It only shortens a deploy
// that was already doomed, and replaces the diagnosis.

const (
	// imagePullGuardEnvVar disables the guard when set to "off" or "false", for
	// the case where aborting early is worse than waiting out the timeout.
	imagePullGuardEnvVar = "DEPLOY_CAMUNDA_IMAGE_PULL_GUARD"
)

// Vars rather than consts so tests can drive the poll loop without wall-clock
// delay.
var (
	// imagePullGuardInterval is how often pods are polled during the wait.
	imagePullGuardInterval = 15 * time.Second
	// imagePullGuardThreshold is how many consecutive polls must report the same
	// terminal failure before the wait is aborted. Two observations spaced by the
	// interval rule out a snapshot taken mid-reconciliation, at a cost of one
	// extra interval.
	imagePullGuardThreshold = 2
)

// newPodLister builds the pod source used by the guard. It is a package-level
// variable so tests can inject a fake without a cluster.
var newPodLister = func(kubeconfig, kubeContext string) (podLister, error) {
	return kube.NewClient(kubeconfig, kubeContext)
}

// podLister is the slice of the Kubernetes client the guard depends on.
type podLister interface {
	ListPods(ctx context.Context, namespace string) (*corev1.PodList, error)
}

// ImagePullFailure is a container image that the registry will not serve.
type ImagePullFailure struct {
	Pod       string
	Container string
	Image     string
	// Reason is the kubelet waiting reason (ErrImagePull, ImagePullBackOff).
	Reason string
	// Message is the kubelet waiting message, which carries the registry error.
	Message string
}

func (f *ImagePullFailure) Error() string {
	return fmt.Sprintf(
		"image %q for container %q in pod %q cannot be pulled and will not become available (%s): %s",
		f.Image, f.Container, f.Pod, f.Reason, f.Message,
	)
}

// terminalPullReasons are the kubelet waiting reasons that *may* denote an
// unresolvable image. They are necessary but not sufficient — kubelet also uses
// them for registry throttling and for auth failures — so a message match is
// required as well.
var terminalPullReasons = map[string]bool{
	"ErrImagePull":     true,
	"ImagePullBackOff": true,
}

// terminalPullMessages are the registry responses that no amount of retrying
// within a Helm timeout will fix: either the tag does not resolve, or — the case
// that motivated this guard — the multi-arch index resolves but the per-platform
// child manifest it references is absent.
//
// Deliberately narrow. Auth failures ("unauthorized", "pull access denied") are
// excluded: they are already covered by the credential preflight, and treating
// them as terminal here would abort deploys that a retry could still save.
var terminalPullMessages = []string{
	"not found",
	"manifest unknown",
	"manifest_unknown",
}

// terminalImagePullFailure reports whether a pod is blocked on an image the
// registry will not serve. Init containers are inspected as well as app
// containers: Camunda pods routinely fail in an init container first.
func terminalImagePullFailure(pod *corev1.Pod) (*ImagePullFailure, bool) {
	statuses := make([]corev1.ContainerStatus, 0,
		len(pod.Status.InitContainerStatuses)+len(pod.Status.ContainerStatuses))
	statuses = append(statuses, pod.Status.InitContainerStatuses...)
	statuses = append(statuses, pod.Status.ContainerStatuses...)

	for _, cs := range statuses {
		waiting := cs.State.Waiting
		if waiting == nil || !terminalPullReasons[waiting.Reason] {
			continue
		}
		if !matchesTerminalPullMessage(waiting.Message) {
			continue
		}
		return &ImagePullFailure{
			Pod:       pod.Name,
			Container: cs.Name,
			Image:     cs.Image,
			Reason:    waiting.Reason,
			Message:   strings.TrimSpace(waiting.Message),
		}, true
	}
	return nil, false
}

func matchesTerminalPullMessage(message string) bool {
	lower := strings.ToLower(message)
	for _, m := range terminalPullMessages {
		if strings.Contains(lower, m) {
			return true
		}
	}
	return false
}

// imagePullGuardEnabled reports whether the guard should run.
func imagePullGuardEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(imagePullGuardEnvVar))) {
	case "off", "false", "0", "no":
		return false
	default:
		return true
	}
}

// imagePullGuard aborts an in-flight Helm wait when a pod hits a terminal image
// pull failure.
type imagePullGuard struct {
	cancel context.CancelFunc
	done   chan struct{}

	mu      sync.Mutex
	failure *ImagePullFailure
	stopped bool
}

// startImagePullGuard derives a cancellable context for the Helm run and starts
// watching pods in the release namespace. The returned context is cancelled as
// soon as a terminal image pull failure is confirmed, which kills the helm child
// process and ends the wait.
//
// The parent context is never cancelled: parallel scenario deploys and matrix
// entries share one, so cancelling it would take down unrelated work.
func startImagePullGuard(ctx context.Context, o types.Options) (context.Context, *imagePullGuard) {
	guardCtx, cancel := context.WithCancel(ctx)
	g := &imagePullGuard{cancel: cancel, done: make(chan struct{})}

	lister, err := newPodLister(o.Kubeconfig, o.KubeContext)
	if err != nil {
		// Losing the guard must never fail a deploy that would otherwise work.
		logging.Logger.Debug().Err(err).
			Msg("Image pull guard disabled: could not build a Kubernetes client")
		close(g.done)
		return guardCtx, g
	}

	deps := imagePullWatchDeps{
		list:      lister.ListPods,
		sleep:     sleepCtx,
		interval:  imagePullGuardInterval,
		threshold: imagePullGuardThreshold,
	}
	go func() {
		defer close(g.done)
		failure := watchTerminalImagePull(guardCtx, deps, o.Namespace)
		if failure == nil {
			return
		}
		g.mu.Lock()
		g.failure = failure
		g.mu.Unlock()

		logging.Logger.Error().
			Str("pod", failure.Pod).
			Str("container", failure.Container).
			Str("image", failure.Image).
			Str("reason", failure.Reason).
			Msg("Aborting the Helm wait: image cannot be pulled")
		cancel()
	}()

	return guardCtx, g
}

// noopImagePullGuard returns a guard that watches nothing, so callers can treat
// the disabled path exactly like the enabled one.
func noopImagePullGuard() *imagePullGuard {
	g := &imagePullGuard{cancel: func() {}, done: make(chan struct{})}
	close(g.done)
	return g
}

// Stop shuts the guard down and reports the terminal failure it observed, if
// any. It is idempotent and safe to call from a defer as well as inline.
func (g *imagePullGuard) Stop() *ImagePullFailure {
	g.mu.Lock()
	alreadyStopped := g.stopped
	g.stopped = true
	g.mu.Unlock()

	if !alreadyStopped {
		g.cancel()
		<-g.done
	}

	g.mu.Lock()
	defer g.mu.Unlock()
	return g.failure
}

// imagePullWatchDeps isolates the watcher from the clock and the cluster so the
// poll loop is unit-testable without either.
type imagePullWatchDeps struct {
	list      func(ctx context.Context, namespace string) (*corev1.PodList, error)
	sleep     func(ctx context.Context, d time.Duration) error
	interval  time.Duration
	threshold int
}

// watchTerminalImagePull polls until the same terminal failure has been observed
// threshold times in a row, or the context ends. Listing errors are ignored: the
// namespace may not exist yet, and a transient API error must not be mistaken
// for a healthy cluster or for a broken image.
func watchTerminalImagePull(ctx context.Context, deps imagePullWatchDeps, namespace string) *ImagePullFailure {
	if namespace == "" {
		return nil
	}
	threshold := deps.threshold
	if threshold < 1 {
		threshold = 1
	}

	var lastKey string
	streak := 0

	for {
		if err := deps.sleep(ctx, deps.interval); err != nil {
			return nil
		}

		pods, err := deps.list(ctx, namespace)
		if err != nil {
			continue
		}

		failure := firstTerminalFailure(pods)
		if failure == nil {
			lastKey, streak = "", 0
			continue
		}

		key := failure.Pod + "/" + failure.Container + "/" + failure.Image
		if key != lastKey {
			lastKey, streak = key, 0
		}
		streak++
		if streak >= threshold {
			return failure
		}
	}
}

// firstTerminalFailure returns the terminal failure of the alphabetically first
// affected pod, so repeated polls of the same broken state agree on one pod
// rather than alternating with map iteration order.
func firstTerminalFailure(pods *corev1.PodList) *ImagePullFailure {
	var chosen *ImagePullFailure
	for i := range pods.Items {
		failure, ok := terminalImagePullFailure(&pods.Items[i])
		if !ok {
			continue
		}
		if chosen == nil || failure.Pod < chosen.Pod {
			chosen = failure
		}
	}
	return chosen
}

// sleepCtx waits for d, or returns early if the context ends.
func sleepCtx(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
