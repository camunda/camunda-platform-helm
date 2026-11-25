// Copyright 2025 Camunda Services GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package matrix

import (
	"maps"
	"testing"

	"scripts/deploy-camunda/config"
	"scripts/deploy-camunda/entra"
)

func TestApplyEntraCredentials(t *testing.T) {
	cases := []struct {
		name         string
		login        *entra.LoginApp
		wantAudience string
		wantEnv      map[string]string
	}{
		{
			name:         "oidc-qa uses the login app",
			login:        &entra.LoginApp{AppID: "login-app", ClientSecret: "login-secret"},
			wantAudience: "login-app",
			wantEnv: map[string]string{
				"ENTRA_APP_CLIENT_ID":     "login-app",
				"ENTRA_APP_CLIENT_SECRET": "login-secret",
				"ENTRA_PARENT_CLIENT_ID":  "parent-app",
				"VENOM_CLIENT_ID":         "venom-app",
				"CONNECTORS_CLIENT_ID":    "login-app",
			},
		},
		{
			name:         "shared app keeps the parent credentials",
			wantAudience: "parent-app",
			wantEnv: map[string]string{
				"ENTRA_PARENT_CLIENT_ID": "parent-app",
				"VENOM_CLIENT_ID":        "venom-app",
				"CONNECTORS_CLIENT_ID":   "parent-app",
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			flags := &config.RuntimeFlags{}
			audience := applyEntraCredentials(flags, "parent-app", "venom-app", tc.login)
			if audience != tc.wantAudience {
				t.Errorf("audience = %q, want %q", audience, tc.wantAudience)
			}
			if !maps.Equal(flags.ExtraEnv, tc.wantEnv) {
				t.Errorf("ExtraEnv = %v, want %v", flags.ExtraEnv, tc.wantEnv)
			}
		})
	}
}
