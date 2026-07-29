package matrix

import (
	"context"
	"fmt"
	"sort"

	"scripts/camunda-core/pkg/kube"
	"scripts/deploy-camunda/deploy"
)

type DoctorEntry struct {
	Entry     Entry
	Namespace string
	Report    *deploy.Report
}

type DoctorReport struct {
	Shared  []deploy.Check
	Entries []DoctorEntry
}

type DoctorOptions struct {
	SkipKube    bool
	ConfigPath  string
	ConfigFound bool
}

func (r *DoctorReport) OK() bool {
	for _, check := range r.Shared {
		if check.Status == deploy.StatusFail {
			return false
		}
	}
	for _, entry := range r.Entries {
		if !entry.Report.OK() {
			return false
		}
	}
	return true
}

func Doctor(ctx context.Context, entries []Entry, opts RunOptions, doctorOpts DoctorOptions) (*DoctorReport, error) {
	report := &DoctorReport{}
	contexts := map[string]struct{}{}
	for _, entry := range entries {
		if entry.Topology != nil {
			if kubeContext := ResolveKubeContext(opts, entry); kubeContext != "" {
				contexts[kubeContext] = struct{}{}
			}
			report.Entries = append(report.Entries, DoctorEntry{
				Entry: entry, Namespace: ResolveNamespace(opts, entry),
				Report: &deploy.Report{Checks: []deploy.Check{{
					Name: "topology release preflight", Status: deploy.StatusWarn,
					Detail:      "not yet supported for this topology entry",
					Remediation: "run the topology scenario's dedicated validation workflow",
				}}},
			})
			continue
		}
		flags, namespace, kubeContext, _, cleanup, err := BuildEntryFlags(entry, opts)
		if err != nil {
			return nil, fmt.Errorf("build %s doctor flags: %w", EntryID(entry), err)
		}
		entryReport := deploy.Preflight(ctx, flags, deploy.PreflightOptions{
			ConfigPath: doctorOpts.ConfigPath, ConfigFound: doctorOpts.ConfigFound, SkipKubeReachability: true,
		})
		cleanup()
		report.Entries = append(report.Entries, DoctorEntry{Entry: entry, Namespace: namespace, Report: entryReport})
		if kubeContext != "" {
			contexts[kubeContext] = struct{}{}
		}
	}
	promoteSharedChecks(report, []string{"config file", "docker creds (Harbor)", "docker creds (Docker Hub)"})
	removeEntryChecks(report, "kube context")

	ordered := make([]string, 0, len(contexts))
	for kubeContext := range contexts {
		ordered = append(ordered, kubeContext)
	}
	sort.Strings(ordered)
	for _, kubeContext := range ordered {
		check := deploy.Check{Name: "kube context " + kubeContext, Status: deploy.StatusOK, Detail: "reachable"}
		if doctorOpts.SkipKube {
			check.Detail = "reachability not checked"
		} else if err := kube.CheckConnectivity(ctx, kubeContext); err != nil {
			check.Status = deploy.StatusFail
			check.Detail = "not reachable"
			check.Remediation = "check cluster authentication, VPN, and context configuration"
		}
		report.Shared = append(report.Shared, check)
	}
	if len(ordered) == 0 {
		report.Shared = append(report.Shared, deploy.Check{
			Name: "kube context", Status: deploy.StatusFail, Detail: "no context resolved",
			Remediation: "configure --kube-context or a platform-specific context",
		})
	}
	return report, nil
}

func removeEntryChecks(report *DoctorReport, name string) {
	for i := range report.Entries {
		checks := report.Entries[i].Report.Checks
		kept := checks[:0]
		for _, check := range checks {
			if check.Name != name {
				kept = append(kept, check)
			}
		}
		report.Entries[i].Report.Checks = kept
	}
}

func promoteSharedChecks(report *DoctorReport, names []string) {
	for _, name := range names {
		var promoted *deploy.Check
		for i := range report.Entries {
			checks := report.Entries[i].Report.Checks
			kept := checks[:0]
			for _, check := range checks {
				if check.Name != name {
					kept = append(kept, check)
					continue
				}
				if promoted == nil {
					copy := check
					promoted = &copy
				} else if check.Status == deploy.StatusFail || (check.Status == deploy.StatusWarn && promoted.Status == deploy.StatusOK) {
					promoted.Status, promoted.Detail, promoted.Remediation = check.Status, check.Detail, check.Remediation
				}
			}
			report.Entries[i].Report.Checks = kept
		}
		if promoted != nil {
			report.Shared = append(report.Shared, *promoted)
		}
	}
}
