package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestAnnotateFlagGroupsAssignsExplicitGroup(t *testing.T) {
	cmd := &cobra.Command{Use: "fake"}
	var placeholder string
	cmd.Flags().StringVar(&placeholder, "ingress-base-domain", "", "")
	cmd.Flags().StringVar(&placeholder, "log-level", "", "")

	annotateFlagGroups(cmd, map[string][]string{
		grpInfrastructure: {"ingress-base-domain"},
		grpLogging:        {"log-level"},
	})

	if got := cmd.Flags().Lookup("ingress-base-domain").Annotations[flagGroupAnnotation]; len(got) == 0 || got[0] != grpInfrastructure {
		t.Errorf("ingress-base-domain group = %v, want %s", got, grpInfrastructure)
	}
	if got := cmd.Flags().Lookup("log-level").Annotations[flagGroupAnnotation]; len(got) == 0 || got[0] != grpLogging {
		t.Errorf("log-level group = %v, want %s", got, grpLogging)
	}
}

func TestAnnotateFlagGroupsFallsBackToOther(t *testing.T) {
	cmd := &cobra.Command{Use: "fake"}
	var placeholder string
	cmd.Flags().StringVar(&placeholder, "unspecified-flag", "", "")

	annotateFlagGroups(cmd, map[string][]string{})

	if got := cmd.Flags().Lookup("unspecified-flag").Annotations[flagGroupAnnotation]; len(got) == 0 || got[0] != grpOther {
		t.Errorf("unlisted flag should default to %q; got %v", grpOther, got)
	}
}

func TestAnnotateFlagGroupsIgnoresUnknownFlagNames(t *testing.T) {
	cmd := &cobra.Command{Use: "fake"}
	var placeholder string
	cmd.Flags().StringVar(&placeholder, "real-flag", "", "")

	// Should not panic on the phantom flag name.
	annotateFlagGroups(cmd, map[string][]string{
		grpInfrastructure: {"real-flag", "removed-flag"},
	})

	if got := cmd.Flags().Lookup("real-flag").Annotations[flagGroupAnnotation]; len(got) == 0 || got[0] != grpInfrastructure {
		t.Errorf("real-flag group = %v, want %s", got, grpInfrastructure)
	}
	if f := cmd.Flags().Lookup("removed-flag"); f != nil {
		t.Errorf("removed-flag should not have been created; got %#v", f)
	}
}

func TestGroupedUsageFuncRendersGroupHeaders(t *testing.T) {
	cmd := &cobra.Command{Use: "fake", Short: "test"}
	var placeholder string
	cmd.Flags().StringVar(&placeholder, "kube-context", "", "help1")
	cmd.Flags().StringVar(&placeholder, "log-level", "", "help2")
	cmd.Flags().StringVar(&placeholder, "unrelated", "", "help3")

	annotateFlagGroups(cmd, map[string][]string{
		grpInfrastructure: {"kube-context"},
		grpLogging:        {"log-level"},
	})

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	if err := cmd.Usage(); err != nil {
		t.Fatalf("Usage() error: %v", err)
	}
	got := buf.String()

	for _, want := range []string{
		"Infrastructure Flags:",
		"--kube-context",
		"Logging Flags:",
		"--log-level",
		"Other Flags:",
		"--unrelated",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("usage output missing %q\n---\n%s", want, got)
		}
	}
}

func TestGroupedUsageFuncSkipsEmptyGroups(t *testing.T) {
	cmd := &cobra.Command{Use: "fake", Short: "test"}
	var placeholder string
	cmd.Flags().StringVar(&placeholder, "only-flag", "", "")

	annotateFlagGroups(cmd, map[string][]string{
		grpAuth: {"only-flag"},
	})

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	_ = cmd.Usage()

	if got := buf.String(); strings.Contains(got, "Registry Flags:") {
		t.Errorf("empty group header should be suppressed; output:\n%s", got)
	}
}

func TestGroupedUsageFuncHidesHiddenFlags(t *testing.T) {
	cmd := &cobra.Command{Use: "fake", Short: "test"}
	var placeholder string
	cmd.Flags().StringVar(&placeholder, "public", "", "")
	cmd.Flags().StringVar(&placeholder, "hidden-legacy", "", "DEPRECATED")
	_ = cmd.Flags().MarkHidden("hidden-legacy")

	annotateFlagGroups(cmd, map[string][]string{
		grpInfrastructure: {"public", "hidden-legacy"},
	})

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	_ = cmd.Usage()

	if strings.Contains(buf.String(), "hidden-legacy") {
		t.Errorf("hidden flag rendered in usage output:\n%s", buf.String())
	}
}

func TestRootAndMatrixRunFlagGroupsCoverKnownFlags(t *testing.T) {
	// Guard against typos in the group tables: every flag listed must resolve
	// to a real flag on the command it targets.
	rootCmd := NewRootCommand()
	for _, names := range rootFlagGroups() {
		for _, n := range names {
			if rootCmd.Flags().Lookup(n) == nil {
				t.Errorf("rootFlagGroups references unknown flag %q", n)
			}
		}
	}

	matrixRun := findSub(newMatrixCommand(), "run")
	if matrixRun == nil {
		t.Fatal("matrix run subcommand not found")
	}
	for _, names := range matrixRunFlagGroups() {
		for _, n := range names {
			if matrixRun.Flags().Lookup(n) == nil {
				t.Errorf("matrixRunFlagGroups references unknown flag %q", n)
			}
		}
	}
}

func findSub(cmd *cobra.Command, name string) *cobra.Command {
	for _, sub := range cmd.Commands() {
		if sub.Name() == name {
			return sub
		}
	}
	return nil
}
