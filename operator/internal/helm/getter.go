// Copyright 2026 Camunda Services GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package helm

import (
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

// namespacedGetter pins the namespace Helm uses for manifests that declare none.
//
// action.Install.Namespace and Configuration.Init only decide where the release
// record is stored. The namespace a namespace-less manifest lands in comes from
// the kube client's own resolution, which for a RESTClientGetter built from an
// in-cluster config or bare ConfigFlags is "default". Without this wrapper an
// operator managing namespace foo silently creates its workloads in default.
type namespacedGetter struct {
	genericclioptions.RESTClientGetter
	namespace string
}

func (g namespacedGetter) ToRawKubeConfigLoader() clientcmd.ClientConfig {
	return namespacedClientConfig{
		delegate:  g.RESTClientGetter.ToRawKubeConfigLoader(),
		namespace: g.namespace,
	}
}

type namespacedClientConfig struct {
	delegate  clientcmd.ClientConfig
	namespace string
}

func (c namespacedClientConfig) RawConfig() (clientcmdapi.Config, error) {
	return c.delegate.RawConfig()
}

func (c namespacedClientConfig) ClientConfig() (*rest.Config, error) {
	return c.delegate.ClientConfig()
}

func (c namespacedClientConfig) ConfigAccess() clientcmd.ConfigAccess {
	return c.delegate.ConfigAccess()
}

func (c namespacedClientConfig) Namespace() (string, bool, error) {
	return c.namespace, true, nil
}
