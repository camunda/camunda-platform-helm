package cmd

import (
	"context"
	"encoding/json"
	"fmt"
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
				for _, id := range ids {
					state, err := matrix.NewRunStateStore(root, id).Load()
					if err != nil {
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
			results, runErr := matrix.Run(cmd.Context(), entries, opts)
			fmt.Fprintln(cmd.OutOrStdout(), matrix.PrintRunSummary(results, 0, ""))
			return runErr
		},
	}
	cmd.Flags().StringVar(&stateDir, "state-dir", "", "Matrix run state directory")
	cmd.Flags().StringVar(&entryID, "entry", "", "Resume only this entry ID")
	return cmd
}

func newMatrixCleanupCommand() *cobra.Command {
	var stateDir, entryID string
	var yes bool
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
			for _, item := range state.Entries {
				if entryID != "" && entryID != item.ID {
					continue
				}
				if item.Namespace == "" || item.Status == matrix.RunCleaned {
					continue
				}
				cleanupCtx, cancel := context.WithTimeout(cmd.Context(), 2*time.Minute)
				client, err := kube.NewClient("", item.KubeContext)
				var uid types.UID
				var resourceVersion string
				if err == nil {
					uid, resourceVersion, err = client.OwnedNamespaceIdentity(cleanupCtx, item.Namespace, "deploy-camunda-run", state.ID)
				}
				if err != nil {
					cancel()
					failures = append(failures, item.ID+": "+err.Error())
					continue
				}
				envFile := state.Options.EnvFile
				if versionFile := state.Options.EnvFiles[item.Entry.Version]; versionFile != "" {
					envFile = versionFile
				}
				if envFile == "" {
					envFile = ".env"
				}
				values, envErr := env.ReadFile(envFile)
				if envErr != nil && (entra.IsOIDCEntry(item.Entry.Auth, item.Entry.Identity) || auth0.IsAuth0Identity(item.Entry.Identity)) {
					cancel()
					failures = append(failures, item.ID+": read identity cleanup credentials: "+envErr.Error())
					continue
				}
				if entra.IsOIDCEntry(item.Entry.Auth, item.Entry.Identity) {
					entra.CleanupVenomApp(cleanupCtx, entra.Options{
						Namespace: item.Namespace, KubeContext: item.KubeContext,
						DirectoryID: values["ENTRA_APP_DIRECTORY_ID"], ClientID: values["ENTRA_APP_CLIENT_ID"], ClientSecret: values["ENTRA_APP_CLIENT_SECRET"],
					})
				}
				if auth0.IsAuth0Identity(item.Entry.Identity) {
					auth0.CleanupClients(cleanupCtx, auth0.Options{
						Namespace: item.Namespace, KubeContext: item.KubeContext, Domain: values["AUTH0_DOMAIN"],
						MgmtToken: values["AUTH0_MGMT_TOKEN"], MgmtClientID: values["AUTH0_MGMT_CLIENT_ID"], MgmtClientSecret: values["AUTH0_MGMT_CLIENT_SECRET"],
					})
				}
				if currentUID, currentVersion, ownershipErr := client.OwnedNamespaceIdentity(cleanupCtx, item.Namespace, "deploy-camunda-run", state.ID); ownershipErr != nil || currentUID != uid || currentVersion != resourceVersion {
					cancel()
					failures = append(failures, item.ID+": namespace ownership changed during identity cleanup: "+ownershipErr.Error())
					continue
				}
				if uid != "" {
					err = client.DeleteNamespaceWithIdentity(cleanupCtx, item.Namespace, uid, resourceVersion)
				}
				cancel()
				if err != nil {
					failures = append(failures, item.ID+": "+err.Error())
					continue
				}
				if err := store.MarkCleaned(item.ID); err != nil {
					failures = append(failures, item.ID+": "+err.Error())
				}
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
	return cmd
}
