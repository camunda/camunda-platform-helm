package deploy

import (
	"errors"
	"fmt"
	"testing"
)

func TestTestError_Error(t *testing.T) {
	inner := fmt.Errorf("integration tests failed with exit code 1")
	te := &TestError{Err: inner, Output: "some output"}

	if te.Error() != inner.Error() {
		t.Errorf("Error() = %q, want %q", te.Error(), inner.Error())
	}
}

func TestTestError_Unwrap(t *testing.T) {
	inner := fmt.Errorf("root cause")
	te := &TestError{Err: inner, Output: "captured output"}

	if !errors.Is(te, inner) {
		t.Error("errors.Is should find the wrapped error")
	}
}

func TestTestError_ErrorsAs(t *testing.T) {
	inner := fmt.Errorf("test failures:\n  - integration tests: exit code 1")
	te := &TestError{Err: inner, Output: "FAIL: TestSomething\n"}

	// Wrap it further, like deploy.Execute does
	wrapped := fmt.Errorf("post-deployment tests failed: %w", te)

	var extracted *TestError
	if !errors.As(wrapped, &extracted) {
		t.Fatal("errors.As should find *TestError through wrapping")
	}
	if extracted.Output != "FAIL: TestSomething\n" {
		t.Errorf("Output = %q, want %q", extracted.Output, "FAIL: TestSomething\n")
	}
}

func TestTestError_ErrorsAs_NotPresent(t *testing.T) {
	plain := fmt.Errorf("deployment failed: helm timeout")

	var extracted *TestError
	if errors.As(plain, &extracted) {
		t.Error("errors.As should NOT find *TestError in a plain error")
	}
}
