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

package controller

import "sync"

// releaseLocks serialises work per target Helm release.
//
// controller-runtime already serialises reconciles of one object, but two
// CamundaHub objects can name the same release, and Helm has no distributed lock:
// concurrent operations on one release fail with "another operation
// (install/upgrade/rollback) is in progress" and can leave it in a pending state
// that only a human can clear. Locking by release rather than by object closes
// the in-process half of that; leader election closes the cross-process half.
type releaseLocks struct {
	mu    sync.Mutex
	locks map[string]*sync.Mutex
}

func newReleaseLocks() *releaseLocks {
	return &releaseLocks{locks: map[string]*sync.Mutex{}}
}

func (r *releaseLocks) lock(key string) func() {
	r.mu.Lock()
	m, ok := r.locks[key]
	if !ok {
		m = &sync.Mutex{}
		r.locks[key] = m
	}
	r.mu.Unlock()

	m.Lock()
	return m.Unlock
}
