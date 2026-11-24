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

package webhook

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/GoogleContainerTools/config-sync/pkg/applier"
	"github.com/GoogleContainerTools/config-sync/pkg/core"
	"github.com/GoogleContainerTools/config-sync/pkg/core/k8sobjects"
	"github.com/GoogleContainerTools/config-sync/pkg/declared"
	csmetadata "github.com/GoogleContainerTools/config-sync/pkg/metadata"
	"github.com/GoogleContainerTools/config-sync/pkg/testing/openapitest"
	admissionv1 "k8s.io/api/admission/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/cli-utils/pkg/common"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

const (
	rootSyncName = "my-root-sync"
	repoSyncName = "my-repo-sync"
)

func rootSyncManagerAnnotation(rsName string) string {
	return declared.ResourceManager(declared.RootScope, rsName)
}

func repoSyncManagerAnnotation(ns, rsName string) string {
	return declared.ResourceManager(declared.Scope(ns), rsName)
}

func TestValidator_Handle(t *testing.T) {
	testCases := []struct {
		name   string
		oldObj client.Object
		newObj client.Object
		user   authenticationv1.UserInfo
		deny   metav1.StatusReason
	}{
		{
			name: "Root reconciler deletes an object it manages",
			oldObj: k8sobjects.RoleObject(
				core.Name("hello"),
				core.Namespace("world"),
				core.Label(csmetadata.ManagedByKey, csmetadata.ManagedByValue),
				csmetadata.WithManagementMode(csmetadata.ManagementEnabled),
				core.Annotation(csmetadata.ResourceManagerKey, rootSyncManagerAnnotation(rootSyncName)),
				setRules([]rbacv1.PolicyRule{
					{
						APIGroups: []string{""},
						Resources: []string{"pods"},
						Verbs:     []string{"get", "list"},
					},
				}),
				core.Annotation(csmetadata.DeclaredFieldsKey, `{"f:metadata":{"f:labels":{"f:app.kubernetes.io/managed-by":{}},"f:annotations":{"f:configmanagement.gke.io/managed":{},"f:configsync.gke.io/manager":{}}},"f:rules":{}}`),
			),
			user: configSyncRootReconciler(rootSyncName),
		},
		{
			name: "Root reconciler deletes an object managed by a namespace reconciler",
			oldObj: k8sobjects.RoleObject(
				core.Name("hello"),
				core.Namespace("world"),
				core.Label(csmetadata.ManagedByKey, csmetadata.ManagedByValue),
				csmetadata.WithManagementMode(csmetadata.ManagementEnabled),
				core.Annotation(csmetadata.ResourceManagerKey, repoSyncManagerAnnotation("bookstore", repoSyncName)),
				setRules([]rbacv1.PolicyRule{
					{
						APIGroups: []string{""},
						Resources: []string{"pods"},
						Verbs:     []string{"get", "list"},
					},
				}),
				core.Annotation(csmetadata.DeclaredFieldsKey, `{"f:metadata":{"f:labels":{"f:app.kubernetes.io/managed-by":{}},"f:annotations":{"f:configmanagement.gke.io/managed":{},"f:configsync.gke.io/manager":{}}},"f:rules":{}}`),
			),
			user: configSyncRootReconciler(rootSyncName),
			deny: metav1.StatusReasonForbidden,
		},
		{
			name: "Namespace reconciler deletes an object it manages",
			oldObj: k8sobjects.RoleObject(
				core.Name("hello"),
				core.Namespace("world"),
				core.Label(csmetadata.ManagedByKey, csmetadata.ManagedByValue),
				csmetadata.WithManagementMode(csmetadata.ManagementEnabled),
				core.Annotation(csmetadata.ResourceManagerKey, repoSyncManagerAnnotation("bookstore", repoSyncName)),
				setRules([]rbacv1.PolicyRule{
					{
						APIGroups: []string{""},
						Resources: []string{"pods"},
						Verbs:     []string{"get", "list"},
					},
				}),
				core.Annotation(csmetadata.DeclaredFieldsKey, `{"f:metadata":{"f:labels":{"f:app.kubernetes.io/managed-by":{}},"f:annotations":{"f:configmanagement.gke.io/managed":{},"f:configsync.gke.io/manager":{}}},"f:rules":{}}`),
			),
			user: configSyncNamespaceReconciler("bookstore", repoSyncName),
		},
		{
			name: "Namespace reconciler deletes an object it does not manage",
			oldObj: k8sobjects.RoleObject(
				core.Name("hello"),
				core.Namespace("world"),
				core.Label(csmetadata.ManagedByKey, csmetadata.ManagedByValue),
				csmetadata.WithManagementMode(csmetadata.ManagementEnabled),
				core.Annotation(csmetadata.ResourceManagerKey, repoSyncManagerAnnotation("videostore", repoSyncName)),
				setRules([]rbacv1.PolicyRule{
					{
						APIGroups: []string{""},
						Resources: []string{"pods"},
						Verbs:     []string{"get", "list"},
					},
				}),
				core.Annotation(csmetadata.DeclaredFieldsKey, `{"f:metadata":{"f:labels":{"f:app.kubernetes.io/managed-by":{}},"f:annotations":{"f:configmanagement.gke.io/managed":{},"f:configsync.gke.io/manager":{}}},"f:rules":{}}`),
			),
			user: configSyncNamespaceReconciler("bookstore", repoSyncName),
			deny: metav1.StatusReasonForbidden,
		},
		{
			name: "Namespace reconciler deletes an object it does not manage",
			oldObj: k8sobjects.RoleObject(
				core.Name("hello"),
				core.Namespace("world"),
				core.Label(csmetadata.ManagedByKey, csmetadata.ManagedByValue),
				csmetadata.WithManagementMode(csmetadata.ManagementEnabled),
				core.Annotation(csmetadata.ResourceManagerKey, repoSyncManagerAnnotation("videostore", repoSyncName)),
				setRules([]rbacv1.PolicyRule{
					{
						APIGroups: []string{""},
						Resources: []string{"pods"},
						Verbs:     []string{"get", "list"},
					},
				}),
				core.Annotation(csmetadata.DeclaredFieldsKey, `{"f:metadata":{"f:labels":{"f:app.kubernetes.io/managed-by":{}},"f:annotations":{"f:configmanagement.gke.io/managed":{},"f:configsync.gke.io/manager":{}}},"f:rules":{}}`),
			),
			user: configSyncNamespaceReconciler("bookstore", repoSyncName),
			deny: metav1.StatusReasonForbidden,
		},
		{
			name: "Bob creates an unmanaged object, which does not have the configmanagement.gke.io/managed and configsync.gke.io/resource-id annotations.",
			newObj: k8sobjects.RoleObject(
				core.Name("hello"),
				core.Namespace("world"),
				setRules([]rbacv1.PolicyRule{
					{
						APIGroups: []string{""},
						Resources: []string{"pods"},
						Verbs:     []string{"get", "list"},
					},
				}),
			),
			user: bob(),
		},
		{
			name: "Bob creates an unmanaged object, whose configmanagement.gke.io/managed annotation is set to enabled, but whose configsync.gke.io/resource-id annotation is unset.",
			newObj: k8sobjects.RoleObject(
				core.Name("hello"),
				core.Namespace("world"),
				csmetadata.WithManagementMode(csmetadata.ManagementEnabled),
				setRules([]rbacv1.PolicyRule{
					{
						APIGroups: []string{""},
						Resources: []string{"pods"},
						Verbs:     []string{"get", "list"},
					},
				}),
			),
			user: bob(),
		},
		{
			name: "Bob creates an unmanaged object, whose configmanagement.gke.io/managed annotation is unset, but whose configsync.gke.io/resource-id annotation is set.",
			newObj: k8sobjects.RoleObject(
				core.Name("hello"),
				core.Namespace("world"),
				core.Annotation(csmetadata.ResourceIDKey, "rbac.authorization.k8s.io_role_world_hello"),
				setRules([]rbacv1.PolicyRule{
					{
						APIGroups: []string{""},
						Resources: []string{"pods"},
						Verbs:     []string{"get", "list"},
					},
				}),
			),
			user: bob(),
		},
		{
			name: "Bob deletes an unmanaged object, which does not have the configmanagement.gke.io/managed and configsync.gke.io/resource-id annotations.",
			oldObj: k8sobjects.RoleObject(
				core.Name("hello"),
				core.Namespace("world"),
				setRules([]rbacv1.PolicyRule{
					{
						APIGroups: []string{""},
						Resources: []string{"pods"},
						Verbs:     []string{"get", "list"},
					},
				}),
			),
			user: bob(),
		},
		{
			name: "Bob deletes an unmanaged object, whose configmanagement.gke.io/managed annotation is set to enabled, but whose configsync.gke.io/resource-id annotation is unset.",
			oldObj: k8sobjects.RoleObject(
				core.Name("hello"),
				core.Namespace("world"),
				csmetadata.WithManagementMode(csmetadata.ManagementEnabled),
				setRules([]rbacv1.PolicyRule{
					{
						APIGroups: []string{""},
						Resources: []string{"pods"},
						Verbs:     []string{"get", "list"},
					},
				}),
			),
			user: bob(),
		},
		{
			name: "Bob deletes an unmanaged object, whose configmanagement.gke.io/managed annotation is unset, but whose configsync.gke.io/resource-id annotation is set.",
			oldObj: k8sobjects.RoleObject(
				core.Name("hello"),
				core.Namespace("world"),
				core.Annotation(csmetadata.ResourceIDKey, "rbac.authorization.k8s.io_role_world_hello"),
				setRules([]rbacv1.PolicyRule{
					{
						APIGroups: []string{""},
						Resources: []string{"pods"},
						Verbs:     []string{"get", "list"},
					},
				}),
			),
			user: bob(),
		},
		{
			name: "Bob updates an unmanaged object, oldObj and newObj both do not have the configmanagement.gke.io/managed and configsync.gke.io/resource-id annotations.",
			oldObj: k8sobjects.RoleObject(
				core.Name("hello"),
				core.Namespace("world"),
				setRules([]rbacv1.PolicyRule{
					{
						APIGroups: []string{""},
						Resources: []string{"pods"},
						Verbs:     []string{"get", "list"},
					},
				}),
			),
			newObj: k8sobjects.RoleObject(
				core.Name("hello"),
				core.Namespace("world"),
				setRules([]rbacv1.PolicyRule{
					{
						APIGroups: []string{""},
						Resources: []string{"pods"},
						Verbs:     []string{"*"},
					},
				}),
			),
			user: bob(),
		},
		{
			name: "Bob updates an unmanaged object, oldObj has the configmanagement.gke.io/managed  annotation, newObj has the configsync.gke.io/resource-id annotation.",
			oldObj: k8sobjects.RoleObject(
				core.Name("hello"),
				core.Namespace("world"),
				csmetadata.WithManagementMode(csmetadata.ManagementEnabled),
				setRules([]rbacv1.PolicyRule{
					{
						APIGroups: []string{""},
						Resources: []string{"pods"},
						Verbs:     []string{"get", "list"},
					},
				}),
			),
			newObj: k8sobjects.RoleObject(
				core.Name("hello"),
				core.Namespace("world"),
				core.Annotation(csmetadata.ResourceIDKey, "rbac.authorization.k8s.io_role_world_hello"),
				setRules([]rbacv1.PolicyRule{
					{
						APIGroups: []string{""},
						Resources: []string{"pods"},
						Verbs:     []string{"*"},
					},
				}),
			),
			user: bob(),
		},
		{
			name: "Bob creates a managed object",
			newObj: k8sobjects.RoleObject(
				core.Name("hello"),
				core.Namespace("world"),
				core.Label(csmetadata.ManagedByKey, csmetadata.ManagedByValue),
				csmetadata.WithManagementMode(csmetadata.ManagementEnabled),
				core.Annotation(csmetadata.ResourceIDKey, "rbac.authorization.k8s.io_role_world_hello"),
				setRules([]rbacv1.PolicyRule{
					{
						APIGroups: []string{""},
						Resources: []string{"pods"},
						Verbs:     []string{"get", "list"},
					},
				}),
				core.Annotation(csmetadata.DeclaredFieldsKey, `{"f:metadata":{"f:labels":{"f:app.kubernetes.io/managed-by":{}},"f:annotations":{"f:configmanagement.gke.io/managed":{}}},"f:rules":{}}`),
			),
			user: bob(),
			deny: metav1.StatusReasonForbidden,
		},
		{
			name: "Bob deletes a managed object",
			oldObj: k8sobjects.RoleObject(
				core.Name("hello"),
				core.Namespace("world"),
				core.Label(csmetadata.ManagedByKey, csmetadata.ManagedByValue),
				csmetadata.WithManagementMode(csmetadata.ManagementEnabled),
				core.Annotation(csmetadata.ResourceIDKey, "rbac.authorization.k8s.io_role_world_hello"),
				setRules([]rbacv1.PolicyRule{
					{
						APIGroups: []string{""},
						Resources: []string{"pods"},
						Verbs:     []string{"get", "list"},
					},
				}),
				core.Annotation(csmetadata.DeclaredFieldsKey, `{"f:metadata":{"f:labels":{"f:app.kubernetes.io/managed-by":{}},"f:annotations":{"f:configmanagement.gke.io/managed":{}}},"f:rules":{}}`),
			),
			user: bob(),
			deny: metav1.StatusReasonForbidden,
		},
		{
			name: "Bob updates a managed object: undeclared fields",
			oldObj: k8sobjects.RoleObject(
				core.Name("hello"),
				core.Namespace("world"),
				core.Label(csmetadata.ManagedByKey, csmetadata.ManagedByValue),
				csmetadata.WithManagementMode(csmetadata.ManagementEnabled),
				core.Annotation(csmetadata.ResourceIDKey, "rbac.authorization.k8s.io_role_world_hello"),
				setRules([]rbacv1.PolicyRule{
					{
						APIGroups: []string{""},
						Resources: []string{"pods"},
						Verbs:     []string{"get", "list"},
					},
				}),
				core.Annotation(csmetadata.DeclaredFieldsKey, `{"f:metadata":{"f:labels":{"f:app.kubernetes.io/managed-by":{}},"f:annotations":{"f:configmanagement.gke.io/managed":{}}},"f:rules":{}}`),
			),
			newObj: k8sobjects.RoleObject(
				core.Name("hello"),
				core.Namespace("world"),
				core.Label(csmetadata.ManagedByKey, csmetadata.ManagedByValue),
				core.Label("acme.com/foo", "bar"),
				csmetadata.WithManagementMode(csmetadata.ManagementEnabled),
				core.Annotation(csmetadata.ResourceIDKey, "rbac.authorization.k8s.io_role_world_hello"),
				setRules([]rbacv1.PolicyRule{
					{
						APIGroups: []string{""},
						Resources: []string{"pods"},
						Verbs:     []string{"get", "list"},
					},
				}),
				core.Annotation(csmetadata.DeclaredFieldsKey, `{"f:metadata":{"f:labels":{"f:app.kubernetes.io/managed-by":{}},"f:annotations":{"f:configmanagement.gke.io/managed":{}}},"f:rules":{}}`),
			),
			user: bob(),
		},
		{
			name: "Bob updates a managed object: declared fields",
			oldObj: k8sobjects.RoleObject(
				core.Name("hello"),
				core.Namespace("world"),
				core.Label(csmetadata.ManagedByKey, csmetadata.ManagedByValue),
				csmetadata.WithManagementMode(csmetadata.ManagementEnabled),
				core.Annotation(csmetadata.ResourceIDKey, "rbac.authorization.k8s.io_role_world_hello"),
				setRules([]rbacv1.PolicyRule{
					{
						APIGroups: []string{""},
						Resources: []string{"pods"},
						Verbs:     []string{"get", "list"},
					},
				}),
				core.Annotation(csmetadata.DeclaredFieldsKey, `{"f:metadata":{"f:labels":{"f:app.kubernetes.io/managed-by":{}},"f:annotations":{"f:configmanagement.gke.io/managed":{}}},"f:rules":{}}`),
			),
			newObj: k8sobjects.RoleObject(
				core.Name("hello"),
				core.Namespace("world"),
				core.Label(csmetadata.ManagedByKey, csmetadata.ManagedByValue),
				csmetadata.WithManagementMode(csmetadata.ManagementEnabled),
				core.Annotation(csmetadata.ResourceIDKey, "rbac.authorization.k8s.io_role_world_hello"),
				setRules([]rbacv1.PolicyRule{
					{
						APIGroups: []string{""},
						Resources: []string{"pods"},
						Verbs:     []string{"*"},
					},
				}),
				core.Annotation(csmetadata.DeclaredFieldsKey, `{"f:metadata":{"f:labels":{"f:app.kubernetes.io/managed-by":{}},"f:annotations":{"f:configmanagement.gke.io/managed":{}}},"f:rules":{}}`),
			),
			user: bob(),
			deny: metav1.StatusReasonForbidden,
		},
		{
			name: "Bob updates a managed object: Config Sync metadata",
			oldObj: k8sobjects.RoleObject(
				core.Name("hello"),
				core.Namespace("world"),
				core.Label(csmetadata.ManagedByKey, csmetadata.ManagedByValue),
				csmetadata.WithManagementMode(csmetadata.ManagementEnabled),
				core.Annotation(csmetadata.ResourceIDKey, "rbac.authorization.k8s.io_role_world_hello"),
				setRules([]rbacv1.PolicyRule{
					{
						APIGroups: []string{""},
						Resources: []string{"pods"},
						Verbs:     []string{"get", "list"},
					},
				}),
				core.Annotation(csmetadata.DeclaredFieldsKey, `{"f:metadata":{"f:labels":{"f:app.kubernetes.io/managed-by":{}},"f:annotations":{"f:configmanagement.gke.io/managed":{}}},"f:rules":{}}`),
			),
			newObj: k8sobjects.RoleObject(
				core.Name("hello"),
				core.Namespace("world"),
				// Removed managed-by label
				csmetadata.WithManagementMode(csmetadata.ManagementEnabled),
				core.Annotation(csmetadata.ResourceIDKey, "rbac.authorization.k8s.io_role_world_hello"),
				setRules([]rbacv1.PolicyRule{
					{
						APIGroups: []string{""},
						Resources: []string{"pods"},
						Verbs:     []string{"get", "list"},
					},
				}),
				core.Annotation(csmetadata.DeclaredFieldsKey, `{"f:metadata":{"f:labels":{"f:app.kubernetes.io/managed-by":{}},"f:annotations":{"f:configmanagement.gke.io/managed":{}}},"f:rules":{}}`),
			),
			user: bob(),
			deny: metav1.StatusReasonForbidden,
		},
		{
			name: "Bob updates a object (whose configmanagement.gke.io/managed annotation is unset, but whose configsync.gke.io/resource-id annotation is set): Config Sync metadata",
			oldObj: k8sobjects.RoleObject(
				core.Name("hello"),
				core.Namespace("world"),
				core.Label(csmetadata.ManagedByKey, csmetadata.ManagedByValue),
				csmetadata.WithManagementMode(csmetadata.ManagementEnabled),
				setRules([]rbacv1.PolicyRule{
					{
						APIGroups: []string{""},
						Resources: []string{"pods"},
						Verbs:     []string{"get", "list"},
					},
				}),
				core.Annotation(csmetadata.DeclaredFieldsKey, `{"f:metadata":{"f:labels":{"f:app.kubernetes.io/managed-by":{}},"f:annotations":{"f:configmanagement.gke.io/managed":{}}},"f:rules":{}}`),
			),
			newObj: k8sobjects.RoleObject(
				core.Name("hello"),
				core.Namespace("world"),
				// Removed managed-by label
				csmetadata.WithManagementMode(csmetadata.ManagementEnabled),
				setRules([]rbacv1.PolicyRule{
					{
						APIGroups: []string{""},
						Resources: []string{"pods"},
						Verbs:     []string{"get", "list"},
					},
				}),
				core.Annotation(csmetadata.DeclaredFieldsKey, `{"f:metadata":{"f:labels":{"f:app.kubernetes.io/managed-by":{}},"f:annotations":{"f:configmanagement.gke.io/managed":{}}},"f:rules":{}}`),
			),
			user: bob(),
		},
		{
			name: "Bob creates a ResourceGroup generated by ConfigSync",
			newObj: k8sobjects.ResourceGroupObject(
				"bookstore",
				"repo-sync",
				csmetadata.WithManagementMode(csmetadata.ManagementEnabled),
				core.Label(common.InventoryLabel, applier.InventoryID("repo-sync", "bookstore"))),
			user: bob(),
			deny: metav1.StatusReasonForbidden,
		},
		{
			name: "Bob deletes a ResourceGroup generated by ConfigSync",
			oldObj: k8sobjects.ResourceGroupObject(
				"bookstore",
				"repo-sync",
				csmetadata.WithManagementMode(csmetadata.ManagementEnabled),
				core.Label(common.InventoryLabel, applier.InventoryID("repo-sync", "bookstore"))),
			user: bob(),
			deny: metav1.StatusReasonForbidden,
		},
		{
			name: "Bob updates a ResourceGroup generated by ConfigSync",
			oldObj: k8sobjects.ResourceGroupObject(
				"bookstore",
				"repo-sync",
				csmetadata.WithManagementMode(csmetadata.ManagementEnabled),
				core.Label(common.InventoryLabel, applier.InventoryID("repo-sync", "bookstore"))),
			newObj: k8sobjects.ResourceGroupObject(
				"bookstore",
				"repo-sync",
				csmetadata.WithManagementMode(csmetadata.ManagementEnabled),
				core.Label(common.InventoryLabel, applier.InventoryID("repo-sync", "bookstore")),
				core.Label("acme.com/foo", "bar")),
			user: bob(),
			deny: metav1.StatusReasonForbidden,
		},
		{
			name: "Bob creates an independent ResourceGroup",
			newObj: k8sobjects.ResourceGroupObject(
				"bookstore",
				"user-created",
				csmetadata.WithManagementMode(csmetadata.ManagementEnabled)),
			user: bob(),
		},
		{
			name: "Bob deletes an independent ResourceGroup",
			oldObj: k8sobjects.ResourceGroupObject(
				"bookstore",
				"user-created",
				csmetadata.WithManagementMode(csmetadata.ManagementEnabled)),
			user: bob(),
		},
		{
			name: "Bob updates an independent ResourceGroup",
			oldObj: k8sobjects.ResourceGroupObject(
				"bookstore",
				"user-created",
				csmetadata.WithManagementMode(csmetadata.ManagementEnabled)),
			newObj: k8sobjects.ResourceGroupObject(
				"bookstore",
				"user-created",
				csmetadata.WithManagementMode(csmetadata.ManagementEnabled),
				core.Label("acme.com/foo", "bar")),
			user: bob(),
		},
		{
			name: "Bob manually modifies lifecycle annotation of an object, whose configmanagement.gke.io/managed annotation is set to enabled, but whose configsync.gke.io/resource-id annotation is unset",
			oldObj: k8sobjects.RoleObject(
				csmetadata.WithManagementMode(csmetadata.ManagementEnabled),
				core.Annotation(csmetadata.LifecycleMutationAnnotation, csmetadata.IgnoreMutation)),
			newObj: k8sobjects.RoleObject(
				csmetadata.WithManagementMode(csmetadata.ManagementEnabled),
				core.Annotation(csmetadata.LifecycleMutationAnnotation, "other")),
		},
		{
			name: "Bob manually modifies lifecycle annotation of an object, whose configsync.gke.io/resource-id annotation is incorrect",
			oldObj: k8sobjects.RoleObject(
				csmetadata.WithManagementMode(csmetadata.ManagementEnabled),
				core.Annotation(csmetadata.ResourceIDKey, "rbac.authorization.k8s.io_role_world_hello"),
				core.Annotation(csmetadata.LifecycleMutationAnnotation, csmetadata.IgnoreMutation)),
			newObj: k8sobjects.RoleObject(
				csmetadata.WithManagementMode(csmetadata.ManagementEnabled),
				core.Annotation(csmetadata.ResourceIDKey, "rbac.authorization.k8s.io_role_world_hello"),
				core.Annotation(csmetadata.LifecycleMutationAnnotation, "other")),
		},
		{
			name: "Bob manually modifies lifecycle annotation of a managed object",
			oldObj: k8sobjects.RoleObject(
				csmetadata.WithManagementMode(csmetadata.ManagementEnabled),
				core.Annotation(csmetadata.ResourceIDKey, "rbac.authorization.k8s.io_role_default-name"),
				core.Annotation(csmetadata.LifecycleMutationAnnotation, csmetadata.IgnoreMutation)),
			newObj: k8sobjects.RoleObject(
				csmetadata.WithManagementMode(csmetadata.ManagementEnabled),
				core.Annotation(csmetadata.ResourceIDKey, "rbac.authorization.k8s.io_role_default-name"),
				core.Annotation(csmetadata.LifecycleMutationAnnotation, "other")),
			deny: metav1.StatusReasonForbidden,
		},
		{
			name: "Bob manually adds lifecycle annotation",
			oldObj: k8sobjects.RoleObject(
				csmetadata.WithManagementMode(csmetadata.ManagementEnabled),
				core.Annotation(csmetadata.ResourceIDKey, "rbac.authorization.k8s.io_role_default-name")),
			newObj: k8sobjects.RoleObject(
				csmetadata.WithManagementMode(csmetadata.ManagementEnabled),
				core.Annotation(csmetadata.ResourceIDKey, "rbac.authorization.k8s.io_role_default-name"),
				core.Annotation(csmetadata.LifecycleMutationAnnotation, csmetadata.IgnoreMutation)),
			deny: metav1.StatusReasonForbidden,
		},
	}

	v := validatorForTest(t)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := request(tc.oldObj, tc.newObj)
			req.UserInfo = tc.user

			resp := v.Handle(context.Background(), req)
			if resp.Allowed {
				if tc.deny != "" {
					t.Errorf("got Handle() response allowed, want denied %q", tc.deny)
				}
			} else if tc.deny == "" {
				t.Errorf("got Handle() response denied %q, want allowed", resp.Result.Reason)
			} else if tc.deny != resp.Result.Reason {
				t.Errorf("got Handle() response denied %q, want denied %q", resp.Result.Reason, tc.deny)
			}
		})
	}
}

func validatorForTest(t *testing.T) *Validator {
	vc, err := openapitest.ValueConverterForTest()
	if err != nil {
		t.Fatalf("Failed to create ValueConverter: %v", err)
	}
	od := &ObjectDiffer{converter: vc}
	return &Validator{differ: od}
}

func configSyncRootReconciler(rsName string) authenticationv1.UserInfo {
	return authenticationv1.UserInfo{
		Groups:   []string{saGroup, saNamespaceGroup},
		Username: saNamespaceGroupPrefix + core.RootReconcilerName(rsName),
	}
}

func configSyncNamespaceReconciler(ns, rsName string) authenticationv1.UserInfo {
	return authenticationv1.UserInfo{
		Groups:   []string{saGroup, saNamespaceGroup},
		Username: saNamespaceGroupPrefix + core.NsReconcilerName(ns, rsName),
	}
}

func bob() authenticationv1.UserInfo {
	return authenticationv1.UserInfo{
		Groups:   []string{"devs@acme.com"},
		Username: "bob@acme.com",
	}
}

func request(oldObj, newObj client.Object) admission.Request {
	var gvk schema.GroupVersionKind
	var name, namespace string

	var operation admissionv1.Operation
	if oldObj == nil {
		operation = admissionv1.Create
		gvk = newObj.GetObjectKind().GroupVersionKind()
		name = newObj.GetName()
		namespace = newObj.GetNamespace()
	} else {
		gvk = oldObj.GetObjectKind().GroupVersionKind()
		name = oldObj.GetName()
		namespace = oldObj.GetNamespace()

		if newObj == nil {
			operation = admissionv1.Delete
		} else {
			operation = admissionv1.Update
		}
	}

	return admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Operation: operation,
			Kind: metav1.GroupVersionKind{
				Group:   gvk.Group,
				Version: gvk.Version,
				Kind:    gvk.Kind,
			},
			Name:      name,
			Namespace: namespace,
			Object: runtime.RawExtension{
				Object: newObj,
			},
			OldObject: runtime.RawExtension{
				Object: oldObj,
			},
		},
	}
}

func rawJSONRequest(t *testing.T, oldObj, newObj client.Object) admission.Request {
	t.Helper()
	req := request(oldObj, newObj)
	req.Object = runtime.RawExtension{}
	req.OldObject = runtime.RawExtension{}

	if newObj != nil {
		raw, err := json.Marshal(newObj)
		if err != nil {
			t.Fatalf("failed to marshal newObj: %v", err)
		}
		req.Object.Raw = raw
	}
	if oldObj != nil {
		raw, err := json.Marshal(oldObj)
		if err != nil {
			t.Fatalf("failed to marshal oldObj: %v", err)
		}
		req.OldObject.Raw = raw
	}
	return req
}

func TestValidator_Handle_RawJSONResourceGroup(t *testing.T) {
	managedRG := func(extraLabels ...core.MetaMutator) client.Object {
		mutators := append([]core.MetaMutator{
			csmetadata.WithManagementMode(csmetadata.ManagementEnabled),
			core.Label(common.InventoryLabel, applier.InventoryID("repo-sync", "bookstore")),
		}, extraLabels...)
		return k8sobjects.ResourceGroupObject("bookstore", "repo-sync", mutators...)
	}

	unmanagedRG := func(extraLabels ...core.MetaMutator) client.Object {
		mutators := append([]core.MetaMutator{
			csmetadata.WithManagementMode(csmetadata.ManagementEnabled),
		}, extraLabels...)
		return k8sobjects.ResourceGroupObject("bookstore", "user-created", mutators...)
	}

	testCases := []struct {
		name   string
		oldObj client.Object
		newObj client.Object
		user   authenticationv1.UserInfo
		deny   metav1.StatusReason
	}{
		{
			name:   "Bob creates a ResourceGroup generated by ConfigSync via raw JSON",
			newObj: managedRG(),
			user:   bob(),
			deny:   metav1.StatusReasonForbidden,
		},
		{
			name:   "Bob deletes a ResourceGroup generated by ConfigSync via raw JSON",
			oldObj: managedRG(),
			user:   bob(),
			deny:   metav1.StatusReasonForbidden,
		},
		{
			name:   "Bob updates a ResourceGroup generated by ConfigSync via raw JSON",
			oldObj: managedRG(),
			newObj: managedRG(core.Label("acme.com/foo", "bar")),
			user:   bob(),
			deny:   metav1.StatusReasonForbidden,
		},
		{
			name:   "Bob creates an independent ResourceGroup via raw JSON",
			newObj: unmanagedRG(),
			user:   bob(),
		},
		{
			name:   "Bob deletes an independent ResourceGroup via raw JSON",
			oldObj: unmanagedRG(),
			user:   bob(),
		},
		{
			name:   "Bob updates an independent ResourceGroup via raw JSON",
			oldObj: unmanagedRG(),
			newObj: unmanagedRG(core.Label("acme.com/foo", "bar")),
			user:   bob(),
		},
	}

	v := validatorForTest(t)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := rawJSONRequest(t, tc.oldObj, tc.newObj)
			req.UserInfo = tc.user

			resp := v.Handle(context.Background(), req)
			if resp.Allowed {
				if tc.deny != "" {
					t.Errorf("got Handle() response allowed, want denied %q", tc.deny)
				}
			} else if tc.deny == "" {
				t.Errorf("got Handle() response denied %q, want allowed", resp.Result.Reason)
			} else if tc.deny != resp.Result.Reason {
				t.Errorf("got Handle() response denied %q, want denied %q", resp.Result.Reason, tc.deny)
			}
		})
	}

	t.Run("handleResourceGroupRequest fails closed when both decoded objects are nil", func(t *testing.T) {
		req := rawJSONRequest(t, nil, managedRG())
		req.UserInfo = bob()
		resp := handleResourceGroupRequest(req, nil, nil)
		if resp.Allowed {
			t.Fatalf("expected handleResourceGroupRequest to deny when both oldObj and newObj are nil")
		}
		if resp.Result == nil || resp.Result.Reason != metav1.StatusReasonForbidden {
			t.Fatalf("expected deny reason %q, got %#v", metav1.StatusReasonForbidden, resp.Result)
		}
	})
}
