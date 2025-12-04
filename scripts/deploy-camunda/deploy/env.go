package deploy

import (
	"os"
	"sync"
)

// envMutex protects environment variable access during parallel deployments.
// This is still needed because os.Setenv/os.Getenv are process-global.
var envMutex sync.Mutex

// EnvScope represents a scoped environment modification that can be safely
// applied and restored. Use this to temporarily modify environment variables
// in a thread-safe manner.
type EnvScope struct {
	original map[string]string
	applied  bool
	mu       sync.Mutex
}

// NewEnvScope creates a new environment scope that will capture the specified keys.
func NewEnvScope(keys []string) *EnvScope {
	return &EnvScope{
		original: captureEnv(keys),
		applied:  false,
	}
}

// Apply sets the environment variables and returns a cleanup function.
// The cleanup function MUST be called to restore the original values.
// This method is safe to call from multiple goroutines.
//
// Usage:
//
//	scope := NewEnvScope([]string{"VAR1", "VAR2"})
//	cleanup := scope.Apply(func() {
//	    os.Setenv("VAR1", "value1")
//	    os.Setenv("VAR2", "value2")
//	})
//	defer cleanup()
func (e *EnvScope) Apply(setter func()) func() {
	e.mu.Lock()
	if e.applied {
		e.mu.Unlock()
		panic("EnvScope.Apply called twice without cleanup")
	}
	e.applied = true
	e.mu.Unlock()

	// Acquire global mutex, apply changes
	envMutex.Lock()
	setter()

	// Return cleanup function that restores and unlocks
	return func() {
		e.mu.Lock()
		defer e.mu.Unlock()
		if !e.applied {
			return // already cleaned up
		}
		restoreEnv(e.original)
		envMutex.Unlock()
		e.applied = false
	}
}

// WithEnv executes a function with temporary environment variables set.
// The environment is automatically restored after the function returns.
// This is a convenience wrapper around EnvScope for simple use cases.
//
// Usage:
//
//	result, err := WithEnv(
//	    []string{"VAR1", "VAR2"},
//	    func() { os.Setenv("VAR1", "value1") },
//	    func() (string, error) {
//	        return someFunction()
//	    },
//	)
func WithEnv[T any](keys []string, setter func(), fn func() (T, error)) (T, error) {
	scope := NewEnvScope(keys)
	cleanup := scope.Apply(setter)
	defer cleanup()
	return fn()
}

// captureEnv saves current values of specified environment variables.
func captureEnv(keys []string) map[string]string {
	envVars := make(map[string]string, len(keys))
	for _, key := range keys {
		envVars[key] = os.Getenv(key)
	}
	return envVars
}

// restoreEnv restores environment variables to captured values.
func restoreEnv(envVars map[string]string) {
	for key, val := range envVars {
		if val == "" {
			_ = os.Unsetenv(key)
		} else {
			_ = os.Setenv(key, val)
		}
	}
}

// DeploymentEnvKeys returns the standard environment variable keys used during deployment.
func DeploymentEnvKeys() []string {
	return []string{
		"KEYCLOAK_REALM",
		"OPTIMIZE_INDEX_PREFIX",
		"ORCHESTRATION_INDEX_PREFIX",
		"TASKLIST_INDEX_PREFIX",
		"OPERATE_INDEX_PREFIX",
		"CAMUNDA_HOSTNAME",
		"FLOW",
	}
}
