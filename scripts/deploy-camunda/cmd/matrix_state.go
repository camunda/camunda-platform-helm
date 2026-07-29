package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/types"

	"scripts/camunda-core/pkg/kube"
	"scripts/deploy-camunda/auth0"
	"scripts/deploy-camunda/config"
	"scripts/deploy-camunda/entra"
	"scripts/deploy-camunda/matrix"
	"scripts/prepare-helm-values/pkg/env"
)

type cleanupKubeClient interface {
	OwnedNamespaceIdentity(context.Context, string, string, string) (types.UID, string, error)
	DeleteNamespaceWithIdentity(context.Context, string, types.UID, string) error
}

var newCleanupKubeClient = func(kubeContext string) (cleanupKubeClient, error) {
	return kube.NewClient("", kubeContext)
}

var cleanupEntraObject = entra.CleanupVenomAppObjectStrict
var cleanupAuth0ClientIDs = auth0.CleanupClientIDsStrict

func defaultMatrixStateRoot(explicit string) (string, error) {
	if explicit != "" {
		return filepath.Abs(explicit)
	}
	repoRoot, err := config.DetectRepoRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(repoRoot, ".deploy-camunda", "runs"), nil
}

func newMatrixStatusCommand() *cobra.Command {
	var stateDir, output string
	cmd := &cobra.Command{
		Use:   "status [run-id]",
		Short: "Show durable matrix run state",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 {
				if err := matrix.ValidateRunID(args[0]); err != nil {
					return err
				}
			}
			root, err := defaultMatrixStateRoot(stateDir)
			if err != nil {
				return err
			}
			if len(args) == 0 {
				ids, err := matrix.ListRunIDs(root)
				if err != nil {
					return err
				}
				if output == "json" {
					states := make([]any, 0, len(ids))
					for _, id := range ids {
						if state, err := matrix.NewRunStateStore(root, id).Load(); err == nil {
							states = append(states, state)
						} else {
							states = append(states, map[string]any{"id": id, "status": "corrupt", "error": err.Error()})
						}
					}
					encoder := json.NewEncoder(cmd.OutOrStdout())
					encoder.SetIndent("", "  ")
					return encoder.Encode(states)
				}
				for _, id := range ids {
					state, err := matrix.NewRunStateStore(root, id).Load()
					if err != nil {
						fmt.Fprintf(cmd.OutOrStdout(), "%s\tcorrupt\t%s\n", id, err)
						continue
					}
					fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%d entries\t%s\n", state.ID, state.Status, len(state.Entries), state.UpdatedAt.Format(time.RFC3339))
				}
				return nil
			}
			state, err := matrix.NewRunStateStore(root, args[0]).Load()
			if err != nil {
				return err
			}
			if output == "json" {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(state)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Run: %s\nStatus: %s\nUpdated: %s\n", state.ID, state.Status, state.UpdatedAt.Format(time.RFC3339))
			for _, item := range state.Entries {
				detail := item.Phase
				if item.Failure != nil {
					detail = item.Failure.Code + ": " + item.Failure.Message
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%-36s %-12s %-16s %s\n", item.ID, item.Status, item.Namespace, detail)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&stateDir, "state-dir", "", "Matrix run state directory")
	cmd.Flags().StringVar(&output, "output", "table", "Output format: table, json")
	return cmd
}

func newMatrixResumeCommand() *cobra.Command {
	var stateDir, entryID string
	var recoverStaleLock bool
	cmd := &cobra.Command{
		Use:   "resume <run-id>",
		Short: "Resume failed, interrupted, and pending matrix entries",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := matrix.ValidateRunID(args[0]); err != nil {
				return err
			}
			root, err := defaultMatrixStateRoot(stateDir)
			if err != nil {
				return err
			}
			store := matrix.NewRunStateStore(root, args[0])
			if recoverStaleLock {
				if err := store.RecoverStaleLock(); err != nil {
					return err
				}
			}
			lock, err := store.Acquire()
			if err != nil {
				return err
			}
			defer lock.Close()
			if err := store.MarkInterrupted(); err != nil {
				return err
			}
			entries, opts, err := store.PrepareResume(entryID)
			if err != nil {
				return err
			}
			opts.StateStore = store
			startedAt := time.Now()
			results, runErr := matrix.Run(cmd.Context(), entries, opts)
			fmt.Fprintln(cmd.OutOrStdout(), matrix.PrintRunSummary(results, time.Since(startedAt), ""))
			return runErr
		},
	}
	cmd.Flags().StringVar(&stateDir, "state-dir", "", "Matrix run state directory")
	cmd.Flags().StringVar(&entryID, "entry", "", "Resume only this entry ID")
	cmd.Flags().BoolVar(&recoverStaleLock, "recover-stale-lock", false, "Explicitly remove a same-host stale run lock after confirming no deploy-camunda process is active")
	return cmd
}

func newMatrixCleanupCommand() *cobra.Command {
	var stateDir, entryID string
	var yes bool
	var recoverStaleLock bool
	cmd := &cobra.Command{
		Use:   "cleanup <run-id>",
		Short: "Delete namespaces recorded by a matrix run",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := matrix.ValidateRunID(args[0]); err != nil {
				return err
			}
			if !yes {
				return fmt.Errorf("cleanup is destructive; pass --yes after reviewing matrix status")
			}
			root, err := defaultMatrixStateRoot(stateDir)
			if err != nil {
				return err
			}
			store := matrix.NewRunStateStore(root, args[0])
			if recoverStaleLock {
				if err := store.RecoverStaleLock(); err != nil {
					return err
				}
			}
			lock, err := store.Acquire()
			if err != nil {
				return err
			}
			defer lock.Close()
			state, err := store.Load()
			if err != nil {
				return err
			}
			var failures []string
			if entryID != "" {
				var targetNamespace string
				for _, item := range state.Entries {
					if item.ID == entryID {
						targetNamespace = item.Namespace
						break
					}
				}
				if targetNamespace != "" {
					for _, item := range state.Entries {
						if item.ID != entryID && item.Namespace == targetNamespace && !item.Cleaned {
							return fmt.Errorf("entry %q shares namespace %q with entry %q; clean the whole run instead", entryID, targetNamespace, item.ID)
						}
					}
				}
			}
			matchedEntry := entryID == ""
			for _, item := range state.Entries {
				if entryID != "" && entryID != item.ID {
					continue
				}
				matchedEntry = true
				if item.Cleaned || item.Namespace == "" || item.Status == matrix.RunCleaned {
					continue
				}
				providerCtx, providerCancel := context.WithTimeout(cmd.Context(), 2*time.Minute)
				client, err := newCleanupKubeClient(item.KubeContext)
				var uid types.UID
				var resourceVersion string
				if err == nil {
					uid, resourceVersion, err = client.OwnedNamespaceIdentity(providerCtx, item.Namespace, "deploy-camunda-run", state.ID)
				}
				if err != nil {
					providerCancel()
					failures = append(failures, item.ID+": "+err.Error())
					continue
				}
				envPath := state.Options.EnvFile
				if path := state.Options.EnvFiles[item.Entry.Version]; path != "" {
					envPath = path
				}
				if envPath == "" {
					envPath = ".env"
				}
				values := cleanupCredentialEnv(envPath)
				if item.ExternalProvisioningStarted {
					providerCancel()
					failures = append(failures, item.ID+": external provisioning started but no exact provider resource checkpoint exists; namespace preserved")
					continue
				}
				if item.Entry.Auth == "oidc" || item.Entry.Identity == "oidc" {
					if item.EntraObjectID == "" {
						err = nil
					} else if item.EntraDirectoryID == "" || values["ENTRA_APP_DIRECTORY_ID"] != item.EntraDirectoryID {
						err = fmt.Errorf("Entra directory does not match recorded tenant")
					} else if item.EntraObjectID != "" {
						err = cleanupEntraObject(providerCtx, entra.Options{
							Namespace: item.Namespace, DirectoryID: values["ENTRA_APP_DIRECTORY_ID"], ClientID: values["ENTRA_APP_CLIENT_ID"], ClientSecret: values["ENTRA_APP_CLIENT_SECRET"],
						}, item.EntraObjectID)
					}
				}
				if err == nil && item.Entry.Identity == "auth0" {
					if len(item.Auth0ClientIDs) == 0 {
						err = nil
					} else if item.Auth0Domain == "" || strings.TrimSuffix(values["AUTH0_DOMAIN"], "/") != strings.TrimSuffix(item.Auth0Domain, "/") {
						err = fmt.Errorf("Auth0 domain does not match recorded tenant")
					} else if len(item.Auth0ClientIDs) > 0 {
						err = cleanupAuth0ClientIDs(providerCtx, auth0.Options{
							Namespace: item.Namespace, Domain: values["AUTH0_DOMAIN"], MgmtToken: values["AUTH0_MGMT_TOKEN"], MgmtClientID: values["AUTH0_MGMT_CLIENT_ID"], MgmtClientSecret: values["AUTH0_MGMT_CLIENT_SECRET"],
						}, item.Auth0ClientIDs)
					}
				}
				if err != nil {
					providerCancel()
					failures = append(failures, item.ID+": external identity cleanup: "+err.Error())
					continue
				}
				providerCancel()
				if uid != "" {
					deleteCtx, deleteCancel := context.WithTimeout(cmd.Context(), 5*time.Minute)
					err = client.DeleteNamespaceWithIdentity(deleteCtx, item.Namespace, uid, resourceVersion)
					deleteCancel()
				}
				if err != nil {
					failures = append(failures, item.ID+": "+err.Error())
					continue
				}
				if err := store.MarkCleaned(item.ID); err != nil {
					failures = append(failures, item.ID+": "+err.Error())
				}
			}
			if !matchedEntry {
				return fmt.Errorf("matrix entry %q not found in run %q", entryID, state.ID)
			}
			if len(failures) > 0 {
				return fmt.Errorf("cleanup partially failed:\n%s", strings.Join(failures, "\n"))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&stateDir, "state-dir", "", "Matrix run state directory")
	cmd.Flags().StringVar(&entryID, "entry", "", "Clean up only this entry ID")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Confirm namespace deletion")
	cmd.Flags().BoolVar(&recoverStaleLock, "recover-stale-lock", false, "Explicitly remove a same-host stale run lock after confirming no deploy-camunda process is active")
	return cmd
}

func cleanupCredentialEnv(envPath string) map[string]string {
	values := make(map[string]string)
	for _, item := range os.Environ() {
		if key, value, ok := strings.Cut(item, "="); ok {
			values[key] = value
		}
	}
	if fileValues, err := env.ReadFile(envPath); err == nil {
		for key, value := range fileValues {
			if _, exists := values[key]; !exists {
				values[key] = value
			}
		}
	}
	return values
}
