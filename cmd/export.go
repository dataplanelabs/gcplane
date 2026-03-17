package cmd

import (
	"fmt"
	"os"

	"github.com/dataplanelabs/gcplane/internal/manifest"
	"github.com/dataplanelabs/gcplane/internal/provider/goclaw"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var exportAll bool

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export GoClaw state as manifest YAML",
	Long: `Dump the current GoClaw configuration as a gcplane manifest.
By default only exports gcplane-managed resources (created_by=gcplane).
Use --all to include all resources.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ep := endpoint
		if ep == "" {
			ep = os.Getenv("GCPLANE_ENDPOINT")
		}
		tok := token
		if tok == "" {
			tok = os.Getenv("GCPLANE_TOKEN")
		}
		if ep == "" || tok == "" {
			return fmt.Errorf("--endpoint and --token (or GCPLANE_ENDPOINT/GCPLANE_TOKEN env) required")
		}

		provider := goclaw.New(ep, tok)
		defer provider.Close()

		m, err := buildExportManifest(provider, ep, tok)
		if err != nil {
			return err
		}

		data, err := yaml.Marshal(m)
		if err != nil {
			return err
		}

		fmt.Print(string(data))
		return nil
	},
}

func init() {
	exportCmd.Flags().BoolVar(&exportAll, "all", false, "export all resources (not just gcplane-managed)")
}

// internalFields are stripped from exported specs to keep manifests clean.
var internalFields = []string{"id", "createdAt", "updatedAt", "createdBy", "created_at", "updated_at", "created_by"}

func buildExportManifest(provider *goclaw.Provider, ep, tok string) (*manifest.Manifest, error) {
	m := &manifest.Manifest{
		APIVersion: "gcplane.io/v1",
		Kind:       "Manifest",
		Metadata:   manifest.Metadata{Name: "exported"},
		Connection: manifest.Connection{Endpoint: ep, Token: "${GCPLANE_TOKEN}"},
	}

	for _, kind := range manifest.ApplyOrder() {
		infos, err := provider.ListAll(kind)
		if err != nil {
			// skip kinds that are unavailable (e.g. WS not connected)
			continue
		}

		for _, info := range infos {
			if !exportAll && info.CreatedBy != "gcplane" {
				continue
			}

			observed, err := provider.Observe(kind, info.Name)
			if err != nil {
				continue
			}

			// Strip internal/server-generated fields
			for _, f := range internalFields {
				delete(observed, f)
			}
			// Name is promoted to resource.Name; agent key field also removed
			delete(observed, "name")
			delete(observed, "agentKey")
			delete(observed, "agent_key")

			m.Resources = append(m.Resources, manifest.Resource{
				Kind: kind,
				Name: info.Name,
				Spec: observed,
			})
		}
	}

	return m, nil
}
