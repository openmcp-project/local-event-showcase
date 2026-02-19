package predicates

import (
	"testing"

	"github.com/kcp-dev/sdk/apis/apis/v1alpha2"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

func TestAPIBindingExportReferencePredicate(t *testing.T) {
	const (
		exportName = "mcp.openmfp.org"
		exportPath = "root:providers:openmcp"
	)

	tests := []struct {
		name     string
		binding  *v1alpha2.APIBinding
		expected bool
	}{
		{
			name: "matching export name and path",
			binding: &v1alpha2.APIBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-binding",
				},
				Spec: v1alpha2.APIBindingSpec{
					Reference: v1alpha2.BindingReference{
						Export: &v1alpha2.ExportBindingReference{
							Name: exportName,
							Path: exportPath,
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "non-matching export name",
			binding: &v1alpha2.APIBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-binding",
				},
				Spec: v1alpha2.APIBindingSpec{
					Reference: v1alpha2.BindingReference{
						Export: &v1alpha2.ExportBindingReference{
							Name: "different.export.name",
							Path: exportPath,
						},
					},
				},
			},
			expected: false,
		},
		{
			name: "non-matching export path",
			binding: &v1alpha2.APIBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-binding",
				},
				Spec: v1alpha2.APIBindingSpec{
					Reference: v1alpha2.BindingReference{
						Export: &v1alpha2.ExportBindingReference{
							Name: exportName,
							Path: "root:different:path",
						},
					},
				},
			},
			expected: false,
		},
		{
			name: "nil export reference",
			binding: &v1alpha2.APIBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-binding",
				},
				Spec: v1alpha2.APIBindingSpec{
					Reference: v1alpha2.BindingReference{
						Export: nil,
					},
				},
			},
			expected: false,
		},
	}

	predicate := APIBindingExportReferencePredicate(exportName, exportPath)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			createEvent := event.CreateEvent{Object: tt.binding}
			updateEvent := event.UpdateEvent{ObjectNew: tt.binding}
			deleteEvent := event.DeleteEvent{Object: tt.binding}
			genericEvent := event.GenericEvent{Object: tt.binding}

			assert.Equal(t, tt.expected, predicate.Create(createEvent))
			assert.Equal(t, tt.expected, predicate.Update(updateEvent))
			assert.Equal(t, tt.expected, predicate.Delete(deleteEvent))
			assert.Equal(t, tt.expected, predicate.Generic(genericEvent))
		})
	}
}

func TestAPIBindingExportReferencePredicate_NonAPIBindingObject(t *testing.T) {
	predicate := APIBindingExportReferencePredicate("mcp.openmfp.org", "root:providers:openmcp")

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-pod",
		},
	}

	createEvent := event.CreateEvent{Object: pod}
	assert.False(t, predicate.Create(createEvent))
}
