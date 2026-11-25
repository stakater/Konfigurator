package v1alpha1

import (
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"

	"sigs.k8s.io/yaml"
)

// The CRD enums are hand-applied while controller-gen can't run (H.1 plan,
// regenerated in H.7). This test pins both CRD copies to the Go constants.
func TestCRDEnumsMatchGoConstants(t *testing.T) {
	files := []string{
		"../../config/crd/bases/konfigurator.stakater.com_konfiguratortemplates.yaml",
		"../../charts/konfigurator/templates/crd.yaml",
	}
	want := map[string][]string{
		"app.kind":     {string(AppKindDaemonSet), string(AppKindDeployment), string(AppKindStatefulSet)},
		"renderTarget": {string(RenderTargetConfigMap), string(RenderTargetSecret)},
	}
	for _, f := range files {
		t.Run(f, func(t *testing.T) {
			spec := loadTemplateSpecSchema(t, f)
			got := map[string][]string{
				"app.kind":     enumOf(t, spec, "app", "kind"),
				"renderTarget": enumOf(t, spec, "renderTarget"),
			}
			for field, w := range want {
				if !reflect.DeepEqual(got[field], w) {
					t.Errorf("%s enum = %v, want %v", field, got[field], w)
				}
			}
		})
	}
}

// loadTemplateSpecSchema returns spec.versions[0].schema.openAPIV3Schema.properties.spec
// of the KonfiguratorTemplate CRD found in file (which may hold several documents
// and Helm template lines).
func loadTemplateSpecSchema(t *testing.T, file string) map[string]interface{} {
	t.Helper()
	raw, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	for _, doc := range strings.Split(string(raw), "\n---") {
		var kept []string
		for _, line := range strings.Split(doc, "\n") {
			if !strings.HasPrefix(strings.TrimSpace(line), "{{") {
				kept = append(kept, line)
			}
		}
		var crd map[string]interface{}
		if err := yaml.Unmarshal([]byte(strings.Join(kept, "\n")), &crd); err != nil || crd == nil {
			continue
		}
		if dig(crd, "spec", "names", "kind") != "KonfiguratorTemplate" {
			continue
		}
		versions, _ := dig(crd, "spec", "versions").([]interface{})
		if len(versions) == 0 {
			t.Fatalf("%s: no versions", file)
		}
		spec, _ := dig(versions[0], "schema", "openAPIV3Schema", "properties", "spec").(map[string]interface{})
		if spec == nil {
			t.Fatalf("%s: no spec schema", file)
		}
		return spec
	}
	t.Fatalf("%s: KonfiguratorTemplate CRD not found", file)
	return nil
}

func enumOf(t *testing.T, spec map[string]interface{}, path ...string) []string {
	t.Helper()
	node := interface{}(spec)
	for _, p := range path {
		node = dig(node, "properties", p)
	}
	items, _ := dig(node, "enum").([]interface{})
	out := make([]string, 0, len(items))
	for _, i := range items {
		out = append(out, i.(string))
	}
	sort.Strings(out)
	return out
}

func dig(node interface{}, keys ...string) interface{} {
	for _, k := range keys {
		m, ok := node.(map[string]interface{})
		if !ok {
			return nil
		}
		node = m[k]
	}
	return node
}
