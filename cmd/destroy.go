package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/dataplanelabs/gcplane/internal/manifest"
	"github.com/dataplanelabs/gcplane/internal/provider/goclaw"
	"github.com/dataplanelabs/gcplane/internal/reconciler"
	"github.com/spf13/cobra"
)

var destroyAutoApprove bool

var destroyCmd = &cobra.Command{
	Use:   "destroy",
	Short: "Destroy all gcplane-managed resources",
	Long: `Remove all resources from GoClaw that were created by gcplane.
Only deletes resources with created_by=gcplane. Resources created
via the UI or other tools are not affected.

Deletes in reverse dependency order for safe cascading.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ep := endpoint
		if ep == "" {
			ep = os.Getenv("GCPLANE_ENDPOINT")
		}
		tok := token
		if tok == "" {
			tok = os.Getenv("GCPLANE_TOKEN")
		}

		// If -f provided, load manifest for connection
		if configFile != "" {
			m, err := loadAndValidateManifest()
			if err != nil {
				return err
			}
			var resolveErr error
			ep, tok, resolveErr = resolveConnection(m)
			if resolveErr != nil {
				return resolveErr
			}
		}

		if ep == "" || tok == "" {
			return fmt.Errorf("--endpoint and --token (or -f manifest) required")
		}

		provider := goclaw.New(ep, tok)
		defer provider.Close()

		// Discover all gcplane-managed resources in reverse dependency order
		var toDelete []reconciler.ResourceInfo
		for _, kind := range manifest.DeleteOrder() {
			if kind == manifest.KindSkill || kind == manifest.KindTTSConfig {
				continue
			}
			infos, err := provider.ListAll(kind)
			if err != nil {
				continue
			}
			for _, info := range infos {
				if info.CreatedBy == "gcplane" {
					toDelete = append(toDelete, info)
				}
			}
		}

		if len(toDelete) == 0 {
			fmt.Println("No gcplane-managed resources found.")
			return nil
		}

		// Show what will be deleted
		fmt.Printf("\n\033[1m\033[31mWill destroy %d resource(s):\033[0m\n\n", len(toDelete))
		for _, r := range toDelete {
			fmt.Printf("  \033[31m- %s/%s\033[0m\n", r.Kind, r.Name)
		}

		// Confirm unless auto-approve
		if !destroyAutoApprove {
			fmt.Print("\nDestroy these resources? This cannot be undone. [y/N] ")
			reader := bufio.NewReader(os.Stdin)
			answer, _ := reader.ReadString('\n')
			answer = strings.TrimSpace(strings.ToLower(answer))
			if answer != "y" && answer != "yes" {
				fmt.Println("Destroy cancelled.")
				return nil
			}
		}

		// Execute deletions
		var failed int
		for _, r := range toDelete {
			if err := provider.Delete(r.Kind, r.Name); err != nil {
				fmt.Printf("  \033[31mx %s/%s: %v\033[0m\n", r.Kind, r.Name, err)
				failed++
			} else {
				fmt.Printf("  \033[32m+ %s/%s deleted\033[0m\n", r.Kind, r.Name)
			}
		}

		fmt.Printf("\n\033[1mDestroy complete!\033[0m %d deleted, %d failed.\n", len(toDelete)-failed, failed)

		if failed > 0 {
			return fmt.Errorf("%d resource(s) failed to delete", failed)
		}
		return nil
	},
}

func init() {
	destroyCmd.Flags().BoolVar(&destroyAutoApprove, "auto-approve", false, "skip confirmation prompt")
}
