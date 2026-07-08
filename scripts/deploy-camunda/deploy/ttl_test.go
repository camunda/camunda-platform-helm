// Copyright Camunda Services GmbH and/or licensed to Camunda Services GmbH under
// one or more contributor license agreements. See the NOTICE file distributed
// with this work for additional information regarding copyright ownership.
// Licensed under the Camunda License 1.0. You may not use this file
// except in compliance with the Camunda License 1.0.

package deploy

import "testing"

func TestResolveDeployTTL(t *testing.T) {
	tests := []struct {
		name    string
		flagTTL string
		envTTL  string
		want    string
	}{
		{
			name:    "flag wins over env",
			flagTTL: "12h",
			envTTL:  "60m",
			want:    "12h",
		},
		{
			name:   "env used when flag empty",
			envTTL: "12h",
			want:   "12h",
		},
		{
			name: "default used when flag and env empty",
			want: "60m",
		},
		{
			name:    "whitespace flag and env treated as empty",
			flagTTL: "  ",
			envTTL:  "\t",
			want:    "60m",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveDeployTTL(tt.flagTTL, tt.envTTL); got != tt.want {
				t.Fatalf("resolveDeployTTL(%q, %q) = %q, want %q", tt.flagTTL, tt.envTTL, got, tt.want)
			}
		})
	}
}
