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

package setup

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"operator/api/v1alpha1"
)

type SetupTest struct {
	suite.Suite
}

func TestSetup(t *testing.T) {
	suite.Run(t, new(SetupTest))
}

// TestSchemeRegistersCamundaHub catches the failure that would make the operator
// start cleanly and then be unable to read its own resource.
func (s *SetupTest) TestSchemeRegistersCamundaHub() {
	scheme, err := Scheme()
	s.Require().NoError(err)

	for _, kind := range []string{"CamundaHub", "CamundaHubList"} {
		gvk := schema.GroupVersionKind{Group: "camunda.io", Version: "v1alpha1", Kind: kind}
		s.True(scheme.Recognizes(gvk), "scheme must recognise %s", gvk)
	}

	// Core types are needed for the ConfigMap and Secret reads behind valuesFrom
	// and the pod reads behind phase convergence.
	s.True(scheme.Recognizes(schema.GroupVersionKind{Version: "v1", Kind: "ConfigMap"}))
	s.True(scheme.Recognizes(schema.GroupVersionKind{Version: "v1", Kind: "Pod"}))
	s.True(scheme.Recognizes(schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}))
}

func (s *SetupTest) TestDefaultsAreSafe() {
	o := DefaultOptions()
	s.True(o.LeaderElect, "leader election defaults on because Helm has no distributed lock")
	s.NotEmpty(o.ChartCacheDir)
	s.Empty(o.WatchNamespace, "watching all namespaces is the default")
}

func (s *SetupTest) TestWatchNamespaceScopesTheCache() {
	scheme, err := Scheme()
	s.Require().NoError(err)

	all := ManagerOptions(scheme, DefaultOptions())
	s.Empty(all.Cache.DefaultNamespaces, "an empty watch-namespace must not scope the cache")

	o := DefaultOptions()
	o.WatchNamespace = "camunda-hub"
	scoped := ManagerOptions(scheme, o)
	s.Len(scoped.Cache.DefaultNamespaces, 1)
	s.Contains(scoped.Cache.DefaultNamespaces, "camunda-hub")
}

// TestLeaderElectionIDIsStable guards a value that must not drift: a changed ID
// would let an old and a new operator each hold their own lease and both write to
// the same Helm release.
func (s *SetupTest) TestLeaderElectionIDIsStable() {
	scheme, err := Scheme()
	s.Require().NoError(err)

	opts := ManagerOptions(scheme, DefaultOptions())
	s.Equal("camunda-hub-operator.camunda.io", opts.LeaderElectionID)
	s.True(opts.LeaderElection)
}

// TestLeaseGivesUpBeforeItExpires is the property that keeps two managers from
// writing at once: the outgoing leader must stop renewing before the lease can be
// taken by anyone else.
func (s *SetupTest) TestLeaseGivesUpBeforeItExpires() {
	scheme, err := Scheme()
	s.Require().NoError(err)

	opts := ManagerOptions(scheme, DefaultOptions())
	s.Require().NotNil(opts.LeaseDuration)
	s.Require().NotNil(opts.RenewDeadline)
	s.Require().NotNil(opts.RetryPeriod)

	s.Less(*opts.RenewDeadline, *opts.LeaseDuration,
		"the renew deadline must be shorter than the lease, or a leader can keep writing after losing it")
	s.Less(*opts.RetryPeriod, *opts.RenewDeadline)
	s.Positive(*opts.LeaseDuration)
	s.LessOrEqual(*opts.LeaseDuration, 60*time.Second)
}

func (s *SetupTest) TestManagerOptionsCarryTheScheme() {
	scheme, err := Scheme()
	s.Require().NoError(err)

	opts := ManagerOptions(scheme, DefaultOptions())
	s.Same(scheme, opts.Scheme)
	s.True(opts.Scheme.Recognizes(v1alpha1.GroupVersion.WithKind("CamundaHub")))
}
