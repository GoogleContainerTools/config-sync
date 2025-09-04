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

package watch

import (
	"context"

	"github.com/GoogleContainerTools/config-sync/pkg/declared"
	"github.com/GoogleContainerTools/config-sync/pkg/remediator/conflict"
	"github.com/GoogleContainerTools/config-sync/pkg/remediator/queue"
	"github.com/GoogleContainerTools/config-sync/pkg/status"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
)

// watcherConfig contains the options needed
// to create a watcher.
type watcherConfig struct {
	gvk             schema.GroupVersionKind
	resources       *declared.Resources
	queue           *queue.ObjectQueue
	scope           declared.Scope
	syncName        string
	startWatch      WatchFunc
	conflictHandler conflict.Handler
	labelSelector   labels.Selector
	commit          string
}

// WatcherFactory knows how to build watch.Runnables.
type WatcherFactory func(cfg watcherConfig) (Runnable, status.Error)

// WatcherFactoryFromListerWatcherFactory returns a new watcherFactory that uses
// the specified ListerWatcherFactory.
func WatcherFactoryFromListerWatcherFactory(factory ListerWatcherFactory) WatcherFactory {
	factoryPtr := factory
	return func(cfg watcherConfig) (Runnable, status.Error) {
		if cfg.startWatch == nil {
			cfg.startWatch = func(ctx context.Context, options metav1.ListOptions) (watch.Interface, error) {
				namespace := "" // RootSync watches at the cluster scope or all namespaces
				if cfg.scope != declared.RootScope {
					// RepoSync only watches at the namespace scope
					namespace = string(cfg.scope)
				}
				lw := factoryPtr(cfg.gvk, namespace)
				if cfg.labelSelector != nil {
					options.LabelSelector = cfg.labelSelector.String()
				}
				return ListAndWatch(ctx, lw, options)
			}
		}
		return NewFiltered(cfg), nil
	}
}
