package subroutines

import (
	"bufio"
	"bytes"
	"context"
	"embed"
	"fmt"
	"io"
	"time"

	apisv1alpha2 "github.com/kcp-dev/sdk/apis/apis/v1alpha2"
	"github.com/platform-mesh/golang-commons/controller/lifecycle/runtimeobject"
	"github.com/platform-mesh/golang-commons/controller/lifecycle/subroutine"
	"github.com/platform-mesh/golang-commons/errors"
	"github.com/platform-mesh/golang-commons/logger"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type DeployAPIResourceSchemasSubroutine struct {
	kcpProvider   KCPClientProvider
	toolName      string
	apiExportName string
	crdFS         embed.FS
	finalizerName string
}

func NewDeployAPIResourceSchemasSubroutine(provider KCPClientProvider, toolName string, apiExportName string, crdFS embed.FS, finalizerName string) *DeployAPIResourceSchemasSubroutine {
	return &DeployAPIResourceSchemasSubroutine{
		kcpProvider:   provider,
		toolName:      toolName,
		apiExportName: apiExportName,
		crdFS:         crdFS,
		finalizerName: finalizerName,
	}
}

var _ subroutine.Subroutine = &DeployAPIResourceSchemasSubroutine{}

func (d *DeployAPIResourceSchemasSubroutine) GetName() string {
	return "DeployAPIResourceSchemas-" + d.toolName
}

func (d *DeployAPIResourceSchemasSubroutine) Finalizers(_ runtimeobject.RuntimeObject) []string {
	return []string{d.finalizerName}
}

func (d *DeployAPIResourceSchemasSubroutine) Process(ctx context.Context, runtimeObj runtimeobject.RuntimeObject) (ctrl.Result, errors.OperatorError) {
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

	schemas, err := d.loadSchemas(version)
	if err != nil {
		log.Error().Err(err).Str("version", version).Msg("failed to load embedded schemas")
		return ctrl.Result{}, errors.NewOperatorError(err, false, true)
	}

	// Apply each APIResourceSchema via server-side apply
	for _, schema := range schemas {
		schema.SetManagedFields(nil)
		applyErr := kcpClient.Patch(ctx, schema, client.Apply, client.FieldOwner("openmcp-init-operator"), client.ForceOwnership) //nolint:staticcheck // Apply() requires typed ApplyConfiguration; Patch+Apply is correct for unstructured SSA
		if applyErr != nil {
			log.Error().Err(applyErr).Str("name", schema.GetName()).Msg("failed to apply APIResourceSchema")
			return ctrl.Result{}, errors.NewOperatorError(applyErr, true, true)
		}
		log.Info().Str("name", schema.GetName()).Msg("APIResourceSchema applied to workspace")
	}

	// Build resource list for APIExport from the applied schemas
	resources, err := d.buildResourceList(schemas)
	if err != nil {
		return ctrl.Result{}, errors.NewOperatorError(err, false, true)
	}

	// Update the existing APIExport with schema resources.
	// The empty APIExport is pre-created by SetupSyncAgentSubroutine;
	// the APIBinding is created by the user via the UI.
	apiExport := &apisv1alpha2.APIExport{
		ObjectMeta: metav1.ObjectMeta{Name: d.apiExportName},
	}
	_, err = controllerutil.CreateOrUpdate(ctx, kcpClient, apiExport, func() error {
		apiExport.Spec.Resources = resources
		return nil
	})
	if err != nil {
		log.Error().Err(err).Str("name", d.apiExportName).Msg("failed to create/update APIExport")
		return ctrl.Result{}, errors.NewOperatorError(err, true, true)
	}
	log.Info().Str("name", d.apiExportName).Msg("APIExport updated with schema resources")

	// Ensure RBAC so users can bind the APIExport
	if err := d.ensureAPIExportBindRBAC(ctx, kcpClient); err != nil {
		log.Error().Err(err).Msg("failed to ensure APIExport bind RBAC")
		return ctrl.Result{}, errors.NewOperatorError(err, true, true)
	}

	return ctrl.Result{}, nil
}

func (d *DeployAPIResourceSchemasSubroutine) Finalize(ctx context.Context, runtimeObj runtimeobject.RuntimeObject) (ctrl.Result, errors.OperatorError) {
	log := logger.LoadLoggerFromContext(ctx)

	version := extractChartVersion(runtimeObj)
	if version == "" {
		return ctrl.Result{}, errors.NewOperatorError(fmt.Errorf("tool resource has no version set"), false, true)
	}

	kcpClient, err := d.kcpProvider.KCPClientFromContext(ctx)
	if err != nil {
		return ctrl.Result{}, errors.NewOperatorError(err, true, true)
	}

	// Wait for all APIBindings referencing this tool's APIExport to be removed before
	// deleting schemas. Other controllers (e.g. security-operator) may need to read the
	// APIResourceSchemas referenced by the APIExport during their own finalization.
	bound, checkErr := apiExportHasBindings(ctx, kcpClient, d.apiExportName)
	if checkErr != nil {
		log.Error().Err(checkErr).Str("apiExport", d.apiExportName).Msg("DeploySchemas: failed to check APIBindings")
		return ctrl.Result{}, errors.NewOperatorError(checkErr, true, true)
	}
	if bound {
		log.Info().Str("apiExport", d.apiExportName).Msg("DeploySchemas: APIBindings still reference APIExport, waiting before deleting schemas")
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	// Clear the APIExport resources (revert to empty export)
	apiExport := &apisv1alpha2.APIExport{
		ObjectMeta: metav1.ObjectMeta{Name: d.apiExportName},
	}
	_, err = controllerutil.CreateOrUpdate(ctx, kcpClient, apiExport, func() error {
		apiExport.Spec.Resources = nil
		return nil
	})
	if err != nil && !apierrors.IsNotFound(err) {
		log.Error().Err(err).Str("name", d.apiExportName).Msg("failed to clear APIExport resources")
		return ctrl.Result{}, errors.NewOperatorError(err, true, true)
	}
	log.Info().Str("name", d.apiExportName).Msg("APIExport resources cleared")

	// Delete each APIResourceSchema
	schemas, err := d.loadSchemas(version)
	if err != nil {
		log.Error().Err(err).Str("version", version).Msg("failed to load embedded schemas for cleanup")
		return ctrl.Result{}, errors.NewOperatorError(err, false, true)
	}

	for _, schema := range schemas {
		obj := &unstructured.Unstructured{}
		obj.SetAPIVersion(schema.GetAPIVersion())
		obj.SetKind(schema.GetKind())
		obj.SetName(schema.GetName())

		if deleteErr := kcpClient.Delete(ctx, obj); deleteErr != nil && !apierrors.IsNotFound(deleteErr) {
			log.Error().Err(deleteErr).Str("name", schema.GetName()).Msg("failed to delete APIResourceSchema")
			return ctrl.Result{}, errors.NewOperatorError(deleteErr, true, true)
		}
		log.Info().Str("name", schema.GetName()).Msg("APIResourceSchema deleted from workspace")
	}

	return ctrl.Result{}, nil
}

func (d *DeployAPIResourceSchemasSubroutine) loadSchemas(version string) ([]*unstructured.Unstructured, error) {
	dir := d.toolName + "/" + version
	data, err := readAllYAML(d.crdFS, dir)
	if err != nil {
		entries, readErr := d.crdFS.ReadDir(d.toolName)
		if readErr == nil && len(entries) == 1 && entries[0].IsDir() {
			dir = d.toolName + "/" + entries[0].Name()
			data, err = readAllYAML(d.crdFS, dir)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("no embedded schemas for %s %s: %w", d.toolName, version, err)
	}
	return parseManifests(data)
}

// buildResourceList extracts group and plural from each schema to build the APIExport resource list.
func (d *DeployAPIResourceSchemasSubroutine) buildResourceList(schemas []*unstructured.Unstructured) ([]apisv1alpha2.ResourceSchema, error) {
	var resources []apisv1alpha2.ResourceSchema

	seen := make(map[string]bool)
	for _, schema := range schemas {
		spec, ok := schema.Object["spec"].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("schema %s has no spec", schema.GetName())
		}

		group, _ := spec["group"].(string)
		names, _ := spec["names"].(map[string]any)
		plural, _ := names["plural"].(string)

		if group == "" || plural == "" {
			return nil, fmt.Errorf("schema %s missing group or plural name", schema.GetName())
		}

		key := group + "/" + plural
		if seen[key] {
			continue
		}
		seen[key] = true

		resources = append(resources, apisv1alpha2.ResourceSchema{
			Group:   group,
			Name:    plural,
			Schema:  schema.GetName(),
			Storage: apisv1alpha2.ResourceSchemaStorage{CRD: &apisv1alpha2.ResourceSchemaStorageCRD{}},
		})
	}

	return resources, nil
}

func (d *DeployAPIResourceSchemasSubroutine) ensureAPIExportBindRBAC(ctx context.Context, kcpClient client.Client) error {
	log := logger.LoadLoggerFromContext(ctx)

	roleName := d.apiExportName + "-bind"

	clusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: roleName},
	}
	_, err := controllerutil.CreateOrUpdate(ctx, kcpClient, clusterRole, func() error {
		clusterRole.Rules = []rbacv1.PolicyRule{
			{
				APIGroups: []string{"apis.kcp.io"},
				Resources: []string{"apiexports"},
				Verbs:     []string{"bind"},
			},
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to create/update ClusterRole for APIExport bind: %w", err)
	}

	clusterRoleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: roleName},
	}
	_, err = controllerutil.CreateOrUpdate(ctx, kcpClient, clusterRoleBinding, func() error {
		clusterRoleBinding.RoleRef = rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     roleName,
		}
		clusterRoleBinding.Subjects = []rbacv1.Subject{
			{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "Group",
				Name:     "system:authenticated",
			},
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to create/update ClusterRoleBinding for APIExport bind: %w", err)
	}

	log.Info().Str("apiExportName", d.apiExportName).Msg("APIExport bind RBAC ensured")
	return nil
}

// readAllYAML reads all YAML files from an embed.FS directory and concatenates them.
func readAllYAML(fsys embed.FS, dir string) ([]byte, error) {
	entries, err := fsys.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var combined []byte
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		data, err := fsys.ReadFile(dir + "/" + entry.Name())
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

// parseManifests parses a multi-document YAML byte slice into unstructured objects.
func parseManifests(data []byte) ([]*unstructured.Unstructured, error) {
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
