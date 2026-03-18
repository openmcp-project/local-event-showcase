package subroutines

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/platform-mesh/golang-commons/controller/lifecycle/runtimeobject"
	"github.com/platform-mesh/golang-commons/controller/lifecycle/subroutine"
	"github.com/platform-mesh/golang-commons/errors"
	"github.com/platform-mesh/golang-commons/logger"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/openmcp/local-event-showcase/demo/openmcp-init-operator/internal/tool"
)

type DeployToolContentConfigurationsSubroutine struct {
	kcpProvider     KCPClientProvider
	toolName        string
	contentForValue string
	entries         []tool.ContentConfigEntry
	finalizerName   string
}

func NewDeployToolContentConfigurationsSubroutine(
	provider KCPClientProvider,
	toolName string,
	contentForValue string,
	entries []tool.ContentConfigEntry,
	finalizerName string,
) *DeployToolContentConfigurationsSubroutine {
	return &DeployToolContentConfigurationsSubroutine{
		kcpProvider:     provider,
		toolName:        toolName,
		contentForValue: contentForValue,
		entries:         entries,
		finalizerName:   finalizerName,
	}
}

var _ subroutine.Subroutine = &DeployToolContentConfigurationsSubroutine{}

func (d *DeployToolContentConfigurationsSubroutine) GetName() string {
	return "DeployToolContentConfigurations-" + d.toolName
}

func (d *DeployToolContentConfigurationsSubroutine) Finalizers(_ runtimeobject.RuntimeObject) []string {
	return []string{d.finalizerName}
}

func (d *DeployToolContentConfigurationsSubroutine) Process(ctx context.Context, runtimeObj runtimeobject.RuntimeObject) (ctrl.Result, errors.OperatorError) {
	log := logger.LoadLoggerFromContext(ctx)

	kcpClient, err := d.kcpProvider.KCPClientFromContext(ctx)
	if err != nil {
		return ctrl.Result{}, errors.NewOperatorError(err, true, true)
	}

	for _, entry := range d.entries {
		cc, buildErr := buildToolContentConfiguration(d.toolName, d.contentForValue, entry)
		if buildErr != nil {
			return ctrl.Result{}, errors.NewOperatorError(fmt.Errorf("building ContentConfiguration for %s: %w", entry.Kind, buildErr), false, true)
		}

		existing := &unstructured.Unstructured{}
		existing.SetAPIVersion(contentConfigAPIVersion)
		existing.SetKind(contentConfigKind)
		existing.SetName(cc.GetName())

		_, createErr := controllerutil.CreateOrUpdate(ctx, kcpClient, existing, func() error {
			existing.SetLabels(cc.GetLabels())
			existing.Object["spec"] = cc.Object["spec"]
			return nil
		})
		if createErr != nil {
			log.Error().Err(createErr).Str("name", cc.GetName()).Msg("failed to create/update ContentConfiguration")
			return ctrl.Result{}, errors.NewOperatorError(createErr, true, true)
		}
		log.Info().Str("name", cc.GetName()).Msg("ContentConfiguration created/updated")
	}

	// Wait for all ContentConfigurations to become Ready before setting tool phase.
	// The extension-manager-operator reconciles CCs asynchronously; the portal only
	// serves CCs that have Ready=True, so we must not signal Ready to the UI earlier.
	for _, entry := range d.entries {
		name := toolContentConfigName(d.toolName, entry.Kind)
		cc := &unstructured.Unstructured{}
		cc.SetAPIVersion(contentConfigAPIVersion)
		cc.SetKind(contentConfigKind)

		if getErr := kcpClient.Get(ctx, client.ObjectKey{Name: name}, cc); getErr != nil {
			log.Info().Err(getErr).Str("name", name).Msg("ContentConfiguration not yet available, requeuing")
			return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
		}

		if !isContentConfigReady(cc) {
			log.Info().Str("name", name).Msg("ContentConfiguration not yet ready, requeuing")
			return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
		}
	}

	setPhase(runtimeObj, "Ready")
	return ctrl.Result{}, nil
}

func (d *DeployToolContentConfigurationsSubroutine) Finalize(ctx context.Context, _ runtimeobject.RuntimeObject) (ctrl.Result, errors.OperatorError) {
	log := logger.LoadLoggerFromContext(ctx)

	kcpClient, err := d.kcpProvider.KCPClientFromContext(ctx)
	if err != nil {
		return ctrl.Result{}, errors.NewOperatorError(err, true, true)
	}

	for _, entry := range d.entries {
		name := toolContentConfigName(d.toolName, entry.Kind)
		cc := &unstructured.Unstructured{}
		cc.SetAPIVersion(contentConfigAPIVersion)
		cc.SetKind(contentConfigKind)
		cc.SetName(name)

		if deleteErr := kcpClient.Delete(ctx, cc); deleteErr != nil && !apierrors.IsNotFound(deleteErr) {
			log.Error().Err(deleteErr).Str("name", name).Msg("failed to delete ContentConfiguration")
			return ctrl.Result{}, errors.NewOperatorError(deleteErr, true, true)
		}
		log.Info().Str("name", name).Msg("ContentConfiguration deleted")
	}

	return ctrl.Result{}, nil
}

func toolContentConfigName(toolName, kind string) string {
	return fmt.Sprintf("opencp-%s-%s", toolName, strings.ToLower(kind))
}

func buildToolContentConfiguration(toolName string, contentFor string, entry tool.ContentConfigEntry) (*unstructured.Unstructured, error) {
	name := toolContentConfigName(toolName, entry.Kind)

	node := map[string]any{
		"pathSegment":             entry.PathSegment,
		"navigationContext":       entry.PathSegment,
		"label":                   entry.DisplayLabel,
		"icon":                    entry.Icon,
		"order":                   entry.Order,
		"hideSideNav":             false,
		"keepSelectedForChildren": true,
		"virtualTree":             true,
		"entityType":              "main.core_platform-mesh_io_account.core_opencp_cloud_managedcontrolplane",
		"loadingIndicator":        map[string]any{"enabled": false},
		"category": map[string]any{
			"id":      entry.CategoryID,
			"isGroup": true,
			"label":   entry.CategoryLabel,
			"order":   entry.CategoryOrder,
		},
		"url":     "https://{context.organization}.portal.localhost:8443/ui/generic-resource/#/",
		"context": buildContext(entry),
	}

	fragment := map[string]any{
		"name": name,
		"luigiConfigFragment": map[string]any{
			"data": map[string]any{
				"nodes": []any{node},
			},
		},
	}

	contentBytes, err := json.Marshal(fragment)
	if err != nil {
		return nil, err
	}

	cc := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": contentConfigAPIVersion,
			"kind":       contentConfigKind,
			"metadata": map[string]any{
				"name": name,
				"labels": map[string]any{
					entityLabel:     entityValue,
					contentForLabel: contentFor,
				},
			},
			"spec": map[string]any{
				"inlineConfiguration": map[string]any{
					"contentType": "json",
					"content":     string(contentBytes),
				},
			},
		},
	}

	return cc, nil
}

func buildContext(entry tool.ContentConfigEntry) map[string]any {
	return map[string]any{
		"resourceDefinition": map[string]any{
			"group":    entry.Group,
			"version":  entry.Version,
			"kind":     entry.Kind,
			"plural":   entry.Plural,
			"singular": strings.ToLower(entry.Kind),
			"scope":    entry.Scope,
		},
	}
}

// isContentConfigReady checks whether an unstructured ContentConfiguration has
// a Ready condition with status "True".
func isContentConfigReady(obj *unstructured.Unstructured) bool {
	conditions, found, err := unstructured.NestedSlice(obj.Object, "status", "conditions")
	if err != nil || !found {
		return false
	}
	for _, c := range conditions {
		cond, ok := c.(map[string]any)
		if !ok {
			continue
		}
		if cond["type"] == "Ready" && cond["status"] == "True" {
			return true
		}
	}
	return false
}
