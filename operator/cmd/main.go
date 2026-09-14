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

// Command manager runs the experimental Camunda Hub operator.
package main

import (
	"flag"
	"os"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"operator/internal/setup"
)

func main() {
	opts := setup.DefaultOptions()

	flag.StringVar(&opts.MetricsAddr, "metrics-bind-address", opts.MetricsAddr,
		"address the metric endpoint binds to")
	flag.StringVar(&opts.ProbeAddr, "health-probe-bind-address", opts.ProbeAddr,
		"address the probe endpoint binds to")
	flag.BoolVar(&opts.LeaderElect, "leader-elect", opts.LeaderElect,
		"enable leader election; keep this on, Helm has no distributed lock")
	flag.StringVar(&opts.ChartCacheDir, "chart-cache-dir", opts.ChartCacheDir,
		"directory for charts pulled from OCI registries")
	flag.StringVar(&opts.WatchNamespace, "watch-namespace", opts.WatchNamespace,
		"restrict watches to one namespace; empty watches all namespaces")

	zapOpts := zap.Options{}
	zapOpts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&zapOpts)))
	log := ctrl.Log.WithName("setup")

	// SetupSignalHandler panics if called twice, so the context is made once and
	// reused for controller registration and for the manager itself.
	ctx := ctrl.SetupSignalHandler()

	scheme, err := setup.Scheme()
	if err != nil {
		log.Error(err, "building scheme")
		os.Exit(1)
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), setup.ManagerOptions(scheme, opts))
	if err != nil {
		log.Error(err, "starting manager")
		os.Exit(1)
	}

	if err := setup.Register(ctx, mgr, opts); err != nil {
		log.Error(err, "registering controllers")
		os.Exit(1)
	}
	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		log.Error(err, "adding health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		log.Error(err, "adding ready check")
		os.Exit(1)
	}

	log.Info("starting manager")
	if err := mgr.Start(ctx); err != nil {
		log.Error(err, "running manager")
		os.Exit(1)
	}
}
