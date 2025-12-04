// Package deploy handles the orchestration and execution of Camunda Platform deployments.
//
// The package is organized into the following files:
//   - deploy.go: Main orchestration logic (Execute, parallel/single deployment coordination)
//   - scenario.go: Scenario context generation, validation, and helper functions
//   - executor.go: Individual scenario deployment execution
//   - secrets.go: Secret generation and management
//   - output.go: Formatted output for deployment results
//   - env.go: Environment variable management for parallel deployments
package deploy

import (
	"context"
	"fmt"
	"scripts/camunda-core/pkg/logging"
	"scripts/deploy-camunda/config"
	"sync"
)

// Execute performs the actual Camunda deployment based on the provided flags.
func Execute(ctx context.Context, flags *config.RuntimeFlags) error {
	// Check if we're deploying multiple scenarios in parallel
	if len(flags.Scenarios) > 1 {
		return executeParallelDeployments(ctx, flags)
	}

	// Single scenario deployment (original behavior)
	return executeSingleDeployment(ctx, flags)
}

// executeParallelDeployments deploys multiple scenarios concurrently.
func executeParallelDeployments(ctx context.Context, flags *config.RuntimeFlags) error {
	logging.Logger.Info().
		Int("count", len(flags.Scenarios)).
		Strs("scenarios", flags.Scenarios).
		Msg("Starting parallel deployment of multiple scenarios")

	// Validate all scenarios exist before starting any deployments
	if err := validateScenarios(flags); err != nil {
		return err
	}

	// Create scenario contexts
	contexts := make([]*ScenarioContext, len(flags.Scenarios))
	for i, scenario := range flags.Scenarios {
		contexts[i] = generateScenarioContext(scenario, flags)
	}

	// Deploy scenarios in parallel independently (don't cancel on first failure)
	// Use WaitGroup instead of errgroup to allow all scenarios to complete
	var wg sync.WaitGroup
	resultCh := make(chan *ScenarioResult, len(flags.Scenarios))

	for _, scenarioCtx := range contexts {
		scenarioCtx := scenarioCtx // capture for closure
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Use original context (not a cancellable one) so failures don't cancel others
			result := deployScenario(ctx, scenarioCtx, flags)
			resultCh <- result
		}()
	}

	// Wait for all deployments to complete
	wg.Wait()
	close(resultCh)

	// Collect results
	results := make([]*ScenarioResult, 0, len(flags.Scenarios))
	for result := range resultCh {
		results = append(results, result)
	}

	// Print summary
	printMultiScenarioSummary(results)

	// Return error if any scenario failed
	var hasErrors bool
	for _, r := range results {
		if r.Error != nil {
			hasErrors = true
			break
		}
	}

	if hasErrors {
		return fmt.Errorf("one or more scenarios failed deployment")
	}
	return nil
}

// executeSingleDeployment deploys a single scenario (original behavior).
func executeSingleDeployment(ctx context.Context, flags *config.RuntimeFlags) error {
	scenario := flags.Scenarios[0]
	scenarioCtx := generateScenarioContext(scenario, flags)
	result := deployScenario(ctx, scenarioCtx, flags)

	if result.Error != nil {
		return result.Error
	}

	// Print single deployment summary
	printDeploymentSummary(result.KeycloakRealm, result.OptimizeIndexPrefix, result.OrchestrationIndexPrefix)
	return nil
}
