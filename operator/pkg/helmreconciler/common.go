// Copyright 2020 Istio Authors
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

package helmreconciler

import (
	"io"
	"strings"

	"istio.io/istio/operator/pkg/name"
	"istio.io/pkg/log"
)

const (
	// MetadataNamespace is the namespace for mesh metadata (labels, annotations)
	MetadataNamespace = "install.operator.istio.io"
	// OwningResourceName represents the name of the owner to which the resource relates
	OwningResourceName = MetadataNamespace + "/owning-resource"
	// OwningResourceNamespace represents the namespace of the owner to which the resource relates
	OwningResourceNamespace = MetadataNamespace + "/owning-resource-namespace"
	// operatorLabelStr indicates Istio operator is managing this resource.
	operatorLabelStr = name.OperatorAPINamespace + "/managed"
	// operatorReconcileStr indicates that the operator will reconcile the resource.
	operatorReconcileStr = "Reconcile"
	// istioComponentLabelStr indicates which Istio component a resource belongs to.
	istioComponentLabelStr = name.OperatorAPINamespace + "/component"
	// istioVersionLabelStr indicates the Istio version of the installation.
	istioVersionLabelStr = name.OperatorAPINamespace + "/version"
)

var (
	scope = log.RegisterScope("installer", "installer", 0)
)

func init() {
	// Tree representation and wait channels are an inversion of ComponentDependencies and are constructed from it.
	buildInstallTree()
	for _, parent := range ComponentDependencies {
		for _, child := range parent {
			DependencyWaitCh[child] = make(chan struct{}, 1)
		}
	}
}

// ComponentTree represents a tree of component dependencies.
type ComponentTree map[name.ComponentName]interface{}
type componentNameToListMap map[name.ComponentName][]name.ComponentName

var (
	// ComponentDependencies is a tree of component dependencies. The semantics are ComponentDependencies[cname] gives
	// the subtree of components that must wait for cname to be installed before starting installation themselves.
	ComponentDependencies = componentNameToListMap{
		name.PilotComponentName: {
			name.PolicyComponentName,
			name.TelemetryComponentName,
			name.CNIComponentName,
			name.IngressComponentName,
			name.EgressComponentName,
			name.AddonComponentName,
		},
		name.IstioBaseComponentName: {
			name.PilotComponentName,
		},
	}

	// InstallTree is a top down hierarchy tree of dependencies where children must wait for the parent to complete
	// before starting installation.
	InstallTree = make(ComponentTree)
	// DependencyWaitCh is a map of signaling channels. A parent with children ch1...chN will signal
	// DependencyWaitCh[ch1]...DependencyWaitCh[chN] when it's completely installed.
	DependencyWaitCh = make(map[name.ComponentName]chan struct{})
)

// buildInstallTree builds a tree from buildInstallTree where parents are the root of each subtree.
func buildInstallTree() {
	// Starting with root, recursively insert each first level child into each node.
	insertChildrenRecursive(name.IstioBaseComponentName, InstallTree, ComponentDependencies)
}

func insertChildrenRecursive(componentName name.ComponentName, tree ComponentTree, children componentNameToListMap) {
	tree[componentName] = make(ComponentTree)
	for _, child := range children[componentName] {
		insertChildrenRecursive(child, tree[componentName].(ComponentTree), children)
	}
}

// InstallTreeString returns a string representation of the dependency tree.
func InstallTreeString() string {
	var sb strings.Builder
	buildInstallTreeString(name.IstioBaseComponentName, "", &sb)
	return sb.String()
}

func buildInstallTreeString(componentName name.ComponentName, prefix string, sb io.StringWriter) {
	_, _ = sb.WriteString(prefix + string(componentName) + "\n")
	if _, ok := InstallTree[componentName].(ComponentTree); !ok {
		return
	}
	for k := range InstallTree[componentName].(ComponentTree) {
		buildInstallTreeString(k, prefix+"  ", sb)
	}
}
