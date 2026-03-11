package subroutines

import (
	"bufio"
	"bytes"
	"context"
	"embed"
	"fmt"
	"io"

	"github.com/platform-mesh/golang-commons/controller/lifecycle/runtimeobject"
	"github.com/platform-mesh/golang-commons/controller/lifecycle/subroutine"
	"github.com/platform-mesh/golang-commons/errors"
	"github.com/platform-mesh/golang-commons/logger"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type DeployCRDsSubroutine struct {
	kcpProvider   KCPClientProvider
	toolName      string
	crdFS         embed.FS
	finalizerName string
}

// NewDeployCRDsSubroutine creates a subroutine that deploys CRDs into a KCP workspace.
// The crdFS must contain CRD YAML files organized as {tool}/{version}/*.yaml.
func NewDeployCRDsSubroutine(provider KCPClientProvider, toolName string, crdFS embed.FS, finalizerName string) *DeployCRDsSubroutine {
	return &DeployCRDsSubroutine{
		kcpProvider:   provider,
		toolName:      toolName,
		crdFS:         crdFS,
		finalizerName: finalizerName,
	}
}

var _ subroutine.Subroutine = &DeployCRDsSubroutine{}

func (d *DeployCRDsSubroutine) GetName() string {
	return "DeployCRDs-" + d.toolName
}

func (d *DeployCRDsSubroutine) Finalizers(_ runtimeobject.RuntimeObject) []string {
	return []string{d.finalizerName}
}

func (d *DeployCRDsSubroutine) Process(ctx context.Context, runtimeObj runtimeobject.RuntimeObject) (ctrl.Result, errors.OperatorError) {
	log := logger.LoadLoggerFromContext(ctx)

	setPhase(runtimeObj, "Provisioning")

	version := extractChartVersion(runtimeObj)
	if version == "" {
		return ctrl.Result{}, errors.NewOperatorError(fmt.Errorf("tool resource has no version set"), false, true)
	}

	kcpClient, err := d.kcpProvider.KCPClientFromContext(ctx)
	if err != nil {
		return ctrl.Result{}, errors.NewOperatorError(err, true, true)
	}

	crds, err := d.loadCRDs(version)
	if err != nil {
		log.Error().Err(err).Str("version", version).Msg("failed to load embedded CRDs")
		return ctrl.Result{}, errors.NewOperatorError(err, false, true)
	}

	for _, crd := range crds {
		crd.SetManagedFields(nil)
		applyErr := kcpClient.Patch(ctx, crd, client.Apply, client.FieldOwner("openmcp-init-operator"), client.ForceOwnership)
		if applyErr != nil {
			log.Error().Err(applyErr).Str("name", crd.GetName()).Msg("failed to apply CRD")
			return ctrl.Result{}, errors.NewOperatorError(applyErr, true, true)
		}
		log.Info().Str("name", crd.GetName()).Msg("CRD applied to workspace")
	}

	return ctrl.Result{}, nil
}

func (d *DeployCRDsSubroutine) Finalize(ctx context.Context, runtimeObj runtimeobject.RuntimeObject) (ctrl.Result, errors.OperatorError) {
	log := logger.LoadLoggerFromContext(ctx)

	version := extractChartVersion(runtimeObj)
	if version == "" {
		return ctrl.Result{}, errors.NewOperatorError(fmt.Errorf("tool resource has no version set"), false, true)
	}

	kcpClient, err := d.kcpProvider.KCPClientFromContext(ctx)
	if err != nil {
		return ctrl.Result{}, errors.NewOperatorError(err, true, true)
	}

	crds, err := d.loadCRDs(version)
	if err != nil {
		log.Error().Err(err).Str("version", version).Msg("failed to load embedded CRDs for cleanup")
		return ctrl.Result{}, errors.NewOperatorError(err, false, true)
	}

	for _, crd := range crds {
		obj := &unstructured.Unstructured{}
		obj.SetAPIVersion(crd.GetAPIVersion())
		obj.SetKind(crd.GetKind())
		obj.SetName(crd.GetName())

		if deleteErr := kcpClient.Delete(ctx, obj); deleteErr != nil && !apierrors.IsNotFound(deleteErr) {
			log.Error().Err(deleteErr).Str("name", crd.GetName()).Msg("failed to delete CRD")
			return ctrl.Result{}, errors.NewOperatorError(deleteErr, true, true)
		}
		log.Info().Str("name", crd.GetName()).Msg("CRD deleted from workspace")
	}

	return ctrl.Result{}, nil
}

// loadCRDs reads all YAML files from the embedded FS at {tool}/{version}/ and
// parses them into unstructured CRD objects. If the exact version directory is
// not found, it falls back to the only available version directory (if exactly one exists).
func (d *DeployCRDsSubroutine) loadCRDs(version string) ([]*unstructured.Unstructured, error) {
	dir := d.toolName + "/" + version
	data, err := readAllYAML(d.crdFS, dir)
	if err != nil {
		// Fall back: scan for available version directories under the tool name
		entries, readErr := d.crdFS.ReadDir(d.toolName)
		if readErr == nil && len(entries) == 1 && entries[0].IsDir() {
			dir = d.toolName + "/" + entries[0].Name()
			data, err = readAllYAML(d.crdFS, dir)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("no embedded CRDs for %s %s: %w", d.toolName, version, err)
	}
	return parseCRDManifests(data)
}

// readAllYAML reads all YAML files from an embed.FS directory and concatenates them.
func readAllYAML(fs embed.FS, dir string) ([]byte, error) {
	entries, err := fs.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var combined []byte
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		data, err := fs.ReadFile(dir + "/" + entry.Name())
		if err != nil {
			return nil, err
		}
		if len(combined) > 0 {
			combined = append(combined, []byte("\n---\n")...)
		}
		combined = append(combined, data...)
	}
	return combined, nil
}

// parseCRDManifests parses a multi-document YAML byte slice into unstructured objects.
func parseCRDManifests(data []byte) ([]*unstructured.Unstructured, error) {
	var result []*unstructured.Unstructured
	reader := yaml.NewYAMLReader(bufio.NewReader(bytes.NewReader(data)))

	for {
		doc, err := reader.Read()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}

		doc = bytes.TrimSpace(doc)
		if len(doc) == 0 {
			continue
		}

		obj := &unstructured.Unstructured{}
		if err := yaml.Unmarshal(doc, &obj.Object); err != nil {
			return nil, err
		}
		if obj.Object == nil {
			continue
		}

		result = append(result, obj)
	}
	return result, nil
}
