// Copyright 2022 Google LLC
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

package fileobjects

import (
	"slices"

	"github.com/GoogleContainerTools/config-sync/pkg/declared"
	"github.com/GoogleContainerTools/config-sync/pkg/importer/analyzer/ast"
	"github.com/GoogleContainerTools/config-sync/pkg/importer/customresources"
	"github.com/GoogleContainerTools/config-sync/pkg/importer/filesystem/cmpath"
	"github.com/GoogleContainerTools/config-sync/pkg/reconciler/namespacecontroller"
	"github.com/GoogleContainerTools/config-sync/pkg/status"
	utildiscovery "github.com/GoogleContainerTools/config-sync/pkg/util/discovery"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog/v2"
)

// RawVisitor is a function that validates or hydrates Raw objects.
type RawVisitor func(r *Raw) status.MultiError

// ObjectVisitor is a function that validates a single FileObject at a time.
type ObjectVisitor func(obj ast.FileObject) status.Error

// Raw contains a collection of FileObjects that have just been parsed from a
// Git repo for a cluster.
type Raw struct {
	ClusterName  string
	Scope        declared.Scope
	SyncName     string
	PolicyDir    cmpath.Relative
	Objects      []ast.FileObject
	PreviousCRDs []*apiextensionsv1.CustomResourceDefinition
	BuildScoper  utildiscovery.BuildScoperFunc
	Converter    *declared.ValueConverter
	Scheme       *runtime.Scheme
	// AllowAPICall indicates whether the hydration process can send k8s API
	// calls. Currently, only dynamic NamespaceSelector requires talking to
	// k8s-api-server.
	AllowAPICall bool
	// DynamicNSSelectorEnabled indicates whether the dynamic mode of
	// NamespaceSelector is enabled.
	DynamicNSSelectorEnabled bool
	// NSControllerState caches the NamespaceSelectors and selected Namespaces
	// in the namespace controller.
	NSControllerState *namespacecontroller.State
	// WebhookEnabled indicates whether Webhook configuration is enabled
	WebhookEnabled bool
	// AllowUnknownKindMatcher indicates which object kinds should ignore
	// scoper errors for when getting the object scope from the API server.
	AllowUnknownKindMatcher ObjectMatcher
}

// Scoped builds a Scoped collection of objects from the Raw objects.
func (r *Raw) Scoped() (*Scoped, status.MultiError) {
	declaredCRDs, errs := customresources.GetCRDs(r.Objects, r.Scheme)
	if errs != nil {
		return nil, errs
	}
	scoper, errs := r.BuildScoper(declaredCRDs, r.Objects)
	if errs != nil {
		return nil, errs
	}

	scoped := &Scoped{
		Scope:                    r.Scope,
		SyncName:                 r.SyncName,
		AllowAPICall:             r.AllowAPICall,
		DynamicNSSelectorEnabled: r.DynamicNSSelectorEnabled,
		NSControllerState:        r.NSControllerState,
	}

	for _, obj := range r.Objects {
		s, err := scoper.GetObjectScope(obj)
		if err != nil {
			if r.AllowUnknownKindMatcher != nil && r.AllowUnknownKindMatcher.Matches(obj.GroupVersionKind()) {
				klog.V(6).Infof("ignoring error: %v", err)
			} else {
				errs = status.Append(errs, err)
			}
		}

		switch s {
		case utildiscovery.ClusterScope:
			scoped.Cluster = append(scoped.Cluster, obj)
		case utildiscovery.NamespaceScope:
			scoped.Namespace = append(scoped.Namespace, obj)
		case utildiscovery.UnknownScope:
			scoped.Unknown = append(scoped.Unknown, obj)
		default:
			errs = status.Append(errs, status.InternalErrorf("unrecognized discovery scope: %s", s))
		}
	}
	return scoped, errs
}

// VisitAllRaw returns a RawVisitor which will call the given ObjectVisitor on
// every FileObject in the Raw objects.
func VisitAllRaw(visit ObjectVisitor) RawVisitor {
	return func(r *Raw) status.MultiError {
		var errs status.MultiError
		for _, obj := range r.Objects {
			errs = status.Append(errs, visit(obj))
		}
		return errs
	}
}

// ObjectMatcher matches objects based on their GroupVersionKind.
type ObjectMatcher interface {
	Matches(schema.GroupVersionKind) bool
}

// GroupMatcher matches objects based on their group.
type GroupMatcher struct {
	Groups []string
}

// Matches returns true if the object's group is one of the allowed groups.
func (gm *GroupMatcher) Matches(gvk schema.GroupVersionKind) bool {
	return slices.Contains(gm.Groups, gvk.Group)
}

// MatchAll matches all objects.
type MatchAll struct{}

// Matches returns true for all objects.
func (m *MatchAll) Matches(_ schema.GroupVersionKind) bool {
	return true
}
