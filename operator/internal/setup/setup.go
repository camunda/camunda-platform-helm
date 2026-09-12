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

// Package setup builds the manager and its controllers.
//
// It exists so the wiring is testable: main() is then thin enough that nothing
// interesting happens there, and the scheme, cache scoping and controller
// registration can be asserted without starting a manager or reaching a cluster.
package setup

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"operator/api/v1alpha1"
	"operator/internal/controller"
)

// LeaderElectionID must stay stable across releases: changing it would let an old
// and a new operator both believe they are leader, and Helm has no distributed
// lock to fall back on.
const LeaderElectionID = "camunda-hub-operator.camunda.io"

// Options are the manager's command-line inputs.
type Options struct {
	MetricsAddr    string
	ProbeAddr      string
	LeaderElect    bool
	ChartCacheDir  string
	WatchNamespace string
}

// DefaultOptions returns the defaults the flags advertise.
func DefaultOptions() Options {
	return Options{
		MetricsAddr:   ":8080",
		ProbeAddr:     ":8081",
		LeaderElect:   true,
		ChartCacheDir: "/var/cache/camunda-operator",
	}
}

// Scheme returns a scheme with the core and CamundaHub types registered.
func Scheme() (*runtime.Scheme, error) {
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		return nil, fmt.Errorf("register core types: %w", err)
	}
	if err := v1alpha1.AddToScheme(scheme); err != nil {
		return nil, fmt.Errorf("register camunda.io types: %w", err)
	}
	return scheme, nil
}

// ManagerOptions translates Options into controller-runtime's configuration.
func ManagerOptions(scheme *runtime.Scheme, o Options) ctrl.Options {
	opts := ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsserver.Options{BindAddress: o.MetricsAddr},
		HealthProbeBindAddress: o.ProbeAddr,
		LeaderElection:         o.LeaderElect,
		LeaderElectionID:       LeaderElectionID,
		// Helm serialises poorly: two managers acting on one release produce
		// "another operation (install/upgrade/rollback) is in progress". Give up
		// leadership well before the lease expires so the old leader stops writing
		// before a new one starts.
		LeaseDuration: durationPtr(15 * time.Second),
		RenewDeadline: durationPtr(10 * time.Second),
		RetryPeriod:   durationPtr(2 * time.Second),
	}

	if o.WatchNamespace != "" {
		opts.Cache.DefaultNamespaces = map[string]cache.Config{o.WatchNamespace: {}}
	}
	return opts
}

// Register wires the controllers and health checks onto a manager.
func Register(ctx context.Context, mgr ctrl.Manager, o Options) error {
	reconciler := &controller.CamundaHubReconciler{
		Client:        mgr.GetClient(),
		Scheme:        mgr.GetScheme(),
		Recorder:      mgr.GetEventRecorderFor("camundahub"),
		RESTGetter:    genericclioptions.NewConfigFlags(false),
		ChartCacheDir: o.ChartCacheDir,
	}
	if err := reconciler.SetupWithManager(ctx, mgr); err != nil {
		return fmt.Errorf("set up CamundaHub controller: %w", err)
	}
	return nil
}

func durationPtr(d time.Duration) *time.Duration { return &d }
