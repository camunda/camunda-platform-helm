package kube

import (
	"context"
	"testing"
)

func TestCheckConnectivity_InvalidContext(t *testing.T) {
	err := CheckConnectivity(context.Background(), "nonexistent-context-that-should-not-exist")
	if err == nil {
		t.Fatal("expected error for non-existent kube context, got nil")
	}
}
