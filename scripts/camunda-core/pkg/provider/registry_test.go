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
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"testing"
)

type fakeProvider struct {
	slot Slot
	name string
}

func (f fakeProvider) Slot() Slot                                       { return f.slot }
func (f fakeProvider) Name() string                                     { return f.name }
func (f fakeProvider) Preflight(context.Context, Env) []Finding         { return nil }
func (f fakeProvider) Prepare(context.Context, Env) error               { return nil }
func (f fakeProvider) Describe() map[string]any                         { return map[string]any{"name": f.name} }
func (f fakeProvider) Values(context.Context, Env) (ValuesLayer, error) { return ValuesLayer{}, nil }

func factoryFor(slot Slot, name string) Factory {
	return func() Provider { return fakeProvider{slot: slot, name: name} }
}

func TestSlotsFixedOrder(t *testing.T) {
	want := []Slot{"credentials", "registry", "domain", "tls", "persistence", "nodePools"}
	if got := Slots(); !reflect.DeepEqual(got, want) {
		t.Fatalf("Slots() = %v, want %v", got, want)
	}
	s := Slots()
	s[0] = "mutated"
	if Slots()[0] != SlotCredentials {
		t.Fatal("Slots() must return a fresh slice")
	}
}

func TestRegisterAndNew(t *testing.T) {
	r := NewRegistry()
	if err := r.Register(SlotTLS, "selfsigned", factoryFor(SlotTLS, "selfsigned")); err != nil {
		t.Fatalf("Register: %v", err)
	}
	p, err := r.New(SlotTLS, "selfsigned")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if p.Slot() != SlotTLS || p.Name() != "selfsigned" {
		t.Fatalf("unexpected provider %s/%s", p.Slot(), p.Name())
	}
	if _, err := r.Lookup(SlotTLS, "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Lookup missing: got %v, want ErrNotFound", err)
	}
}

func TestRegisterDuplicate(t *testing.T) {
	r := NewRegistry()
	r.MustRegister(SlotDomain, "nip", factoryFor(SlotDomain, "nip"))
	if err := r.Register(SlotDomain, "nip", factoryFor(SlotDomain, "nip")); !errors.Is(err, ErrDuplicate) {
		t.Fatalf("got %v, want ErrDuplicate", err)
	}
	defer func() {
		if recover() == nil {
			t.Fatal("MustRegister duplicate did not panic")
		}
	}()
	r.MustRegister(SlotDomain, "nip", factoryFor(SlotDomain, "nip"))
}

func TestUnknownSlot(t *testing.T) {
	r := NewRegistry()
	if err := r.Register("bogus", "x", factoryFor("bogus", "x")); !errors.Is(err, ErrUnknownSlot) {
		t.Fatalf("Register: got %v, want ErrUnknownSlot", err)
	}
	if _, err := r.Lookup("bogus", "x"); !errors.Is(err, ErrUnknownSlot) {
		t.Fatalf("Lookup: got %v, want ErrUnknownSlot", err)
	}
}

func TestRegisterInvalid(t *testing.T) {
	r := NewRegistry()
	if err := r.Register(SlotTLS, "", factoryFor(SlotTLS, "")); !errors.Is(err, ErrInvalid) {
		t.Fatalf("empty name: got %v", err)
	}
	if err := r.Register(SlotTLS, "x", nil); !errors.Is(err, ErrInvalid) {
		t.Fatalf("nil factory: got %v", err)
	}
}

func TestNamesSorted(t *testing.T) {
	r := NewRegistry()
	for _, n := range []string{"zeta", "alpha", "mid"} {
		r.MustRegister(SlotPersistence, n, factoryFor(SlotPersistence, n))
	}
	want := []string{"alpha", "mid", "zeta"}
	if got := r.Names(SlotPersistence); !reflect.DeepEqual(got, want) {
		t.Fatalf("Names = %v, want %v", got, want)
	}
	if got := r.Names(SlotNodePools); len(got) != 0 {
		t.Fatalf("Names empty slot = %v", got)
	}
}

func TestConcurrentRegisterLookup(t *testing.T) {
	r := NewRegistry()
	const n = 50
	var wg sync.WaitGroup
	for i := range n {
		name := fmt.Sprintf("p%02d", i)
		wg.Add(2)
		go func() {
			defer wg.Done()
			if err := r.Register(SlotRegistry, name, factoryFor(SlotRegistry, name)); err != nil {
				t.Errorf("Register %s: %v", name, err)
			}
		}()
		go func() {
			defer wg.Done()
			_, _ = r.Lookup(SlotRegistry, name)
			_ = r.Names(SlotRegistry)
		}()
	}
	wg.Wait()
	if got := len(r.Names(SlotRegistry)); got != n {
		t.Fatalf("registered %d, want %d", got, n)
	}
}

func TestDefaultRegistry(t *testing.T) {
	MustRegister(SlotCredentials, "test-default", factoryFor(SlotCredentials, "test-default"))
	if err := Register(SlotCredentials, "test-default", factoryFor(SlotCredentials, "test-default")); !errors.Is(err, ErrDuplicate) {
		t.Fatalf("got %v, want ErrDuplicate", err)
	}
	if _, err := Lookup(SlotCredentials, "test-default"); err != nil {
		t.Fatal(err)
	}
	if _, err := New(SlotCredentials, "test-default"); err != nil {
		t.Fatal(err)
	}
	if got := Names(SlotCredentials); !reflect.DeepEqual(got, []string{"test-default"}) {
		t.Fatalf("Names = %v", got)
	}
}
