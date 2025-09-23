// Copyright 2025 The Kubernetes Authors.
// SPDX-License-Identifier: Apache-2.0

package filter

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/metadata"
	"sigs.k8s.io/cli-utils/pkg/common"
	"sigs.k8s.io/cli-utils/pkg/object"
)

// PreventUpdateFilter implements ValidationFilter interface to determine
// if an object should not be updated because of an "ignore mutation" annotation.
type PreventUpdateFilter struct {
	Client metadata.Interface
	Mapper meta.RESTMapper
}

const PreventUpdateFilterName = "PreventUpdateFilter"

// Name returns the preferred name for the filter. Usually
// used for logging.
func (puf PreventUpdateFilter) Name() string {
	return PreventUpdateFilterName
}

// Filter returns a AnnotationPreventedUpdateError if the object apply
// should be skipped.
func (puf PreventUpdateFilter) Filter(ctx context.Context, obj *unstructured.Unstructured) error {
	a := obj.GetAnnotations()
	if val, ok := a[common.LifecycleMutationAnnotation]; ok && val == common.IgnoreMutation {
		_, err := puf.getObject(ctx, object.UnstructuredToObjMetadata(obj))
		if apierrors.IsNotFound(err) { // object NotFound - apply
			return nil
		} else if err != nil { // unexpected error - fatal
			return NewFatalError(fmt.Errorf("failed to get current object from cluster: %w", err))
		}
		// Object exists - skip apply
		return &AnnotationPreventedUpdateError{
			Annotation: common.LifecycleMutationAnnotation,
			Value:      common.IgnoreMutation,
		}
	}
	return nil
}

// getObject retrieves the passed object from the cluster, or an error if one occurred.
func (puf PreventUpdateFilter) getObject(ctx context.Context, id object.ObjMetadata) (*metav1.PartialObjectMetadata, error) {
	mapping, err := puf.Mapper.RESTMapping(id.GroupKind)
	if err != nil {
		return nil, err
	}
	namespacedClient := puf.Client.Resource(mapping.Resource).Namespace(id.Namespace)
	return namespacedClient.Get(ctx, id.Name, metav1.GetOptions{})
}

type AnnotationPreventedUpdateError struct {
	Annotation string
	Value      string
}

func (e *AnnotationPreventedUpdateError) Error() string {
	return fmt.Sprintf("annotation prevents apply (%q: %q)", e.Annotation, e.Value)
}

func (e *AnnotationPreventedUpdateError) Is(err error) bool {
	if err == nil {
		return false
	}
	tErr, ok := err.(*AnnotationPreventedUpdateError)
	if !ok {
		return false
	}
	return e.Annotation == tErr.Annotation &&
		e.Value == tErr.Value
}
