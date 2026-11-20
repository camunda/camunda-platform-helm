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

package provider

import (
	"errors"
	"fmt"
	"sort"
	"sync"
)

// Registry errors, usable with errors.Is.
var (
	ErrUnknownSlot = errors.New("unknown provider slot")
	ErrDuplicate   = errors.New("provider already registered")
	ErrNotFound    = errors.New("provider not found")
	ErrInvalid     = errors.New("invalid provider registration")
)

// Registry maps (slot, name) to a Factory. It is safe for concurrent use.
type Registry struct {
	mu        sync.RWMutex
	factories map[Slot]map[string]Factory
}

// NewRegistry returns an empty Registry.
func NewRegistry() *Registry {
	return &Registry{factories: map[Slot]map[string]Factory{}}
}

// Register adds a factory under slot and name. It fails on an unknown slot,
// an empty name, a nil factory, or a duplicate (slot, name).
func (r *Registry) Register(slot Slot, name string, factory Factory) error {
	if !slot.Valid() {
		return fmt.Errorf("%w: %q", ErrUnknownSlot, slot)
	}
	if name == "" || factory == nil {
		return fmt.Errorf("%w: slot %q requires a name and a factory", ErrInvalid, slot)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	byName, ok := r.factories[slot]
	if !ok {
		byName = map[string]Factory{}
		r.factories[slot] = byName
	}
	if _, exists := byName[name]; exists {
		return fmt.Errorf("%w: %s/%s", ErrDuplicate, slot, name)
	}
	byName[name] = factory
	return nil
}

// MustRegister is Register that panics on error.
func (r *Registry) MustRegister(slot Slot, name string, factory Factory) {
	if err := r.Register(slot, name, factory); err != nil {
		panic(err)
	}
}

// Lookup returns the factory registered under slot and name.
func (r *Registry) Lookup(slot Slot, name string) (Factory, error) {
	if !slot.Valid() {
		return nil, fmt.Errorf("%w: %q", ErrUnknownSlot, slot)
	}
	r.mu.RLock()
	f, ok := r.factories[slot][name]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("%w: %s/%s", ErrNotFound, slot, name)
	}
	return f, nil
}

// New instantiates the provider registered under slot and name.
func (r *Registry) New(slot Slot, name string) (Provider, error) {
	f, err := r.Lookup(slot, name)
	if err != nil {
		return nil, err
	}
	return f(), nil
}

// Names returns the sorted provider names registered for slot.
func (r *Registry) Names(slot Slot) []string {
	r.mu.RLock()
	names := make([]string, 0, len(r.factories[slot]))
	for n := range r.factories[slot] {
		names = append(names, n)
	}
	r.mu.RUnlock()
	sort.Strings(names)
	return names
}

var defaultRegistry = NewRegistry()

// Register adds a factory to the default registry.
func Register(slot Slot, name string, factory Factory) error {
	return defaultRegistry.Register(slot, name, factory)
}

// MustRegister adds a factory to the default registry and panics on error.
func MustRegister(slot Slot, name string, factory Factory) {
	defaultRegistry.MustRegister(slot, name, factory)
}

// Lookup returns a factory from the default registry.
func Lookup(slot Slot, name string) (Factory, error) {
	return defaultRegistry.Lookup(slot, name)
}

// New instantiates a provider from the default registry.
func New(slot Slot, name string) (Provider, error) {
	return defaultRegistry.New(slot, name)
}

// Names returns the sorted provider names for slot in the default registry.
func Names(slot Slot) []string {
	return defaultRegistry.Names(slot)
}
