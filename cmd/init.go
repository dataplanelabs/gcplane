package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Generate a starter manifest",
	Long:  "Create a gcplane.yaml manifest with basic configuration.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if _, err := os.Stat("gcplane.yaml"); err == nil {
			return fmt.Errorf("gcplane.yaml already exists")
		}

		reader := bufio.NewReader(os.Stdin)

		fmt.Println("GCPlane Init — Generate starter manifest")
		fmt.Println()

		fmt.Print("Deployment name [my-setup]: ")
		name := readLine(reader, "my-setup")

		fmt.Print("GoClaw endpoint [http://localhost:18790]: ")
		ep := readLine(reader, "http://localhost:18790")

		content := fmt.Sprintf(`apiVersion: gcplane.io/v1
kind: Manifest
metadata:
  name: %s
  environment: dev

connection:
  endpoint: %s
  token: ${GOCLAW_TOKEN}

resources:
  - kind: Provider
    name: openrouter
    spec:
      displayName: "OpenRouter"
      providerType: openrouter
      apiBase: https://openrouter.ai/api/v1
      apiKey: ${OPENROUTER_API_KEY}
      enabled: true

  - kind: Agent
    name: assistant
    spec:
      displayName: "Assistant"
      provider: openrouter
      model: google/gemini-2.5-flash-preview
      agentType: open
      status: active
      isDefault: true
      toolsConfig:
        profile: coding
`, name, ep)

		if err := os.WriteFile("gcplane.yaml", []byte(content), 0644); err != nil {
			return fmt.Errorf("write gcplane.yaml: %w", err)
		}

		if _, err := os.Stat(".env.example"); err != nil {
			envContent := "GOCLAW_TOKEN=\nOPENROUTER_API_KEY=\n"
			_ = os.WriteFile(".env.example", []byte(envContent), 0644)
			fmt.Println("Created .env.example")
		}

		fmt.Println("Created gcplane.yaml")
		fmt.Println("\nNext steps:")
		fmt.Println("  1. cp .env.example .env && edit .env")
		fmt.Println("  2. gcplane plan")
		fmt.Println("  3. gcplane apply")
		return nil
	},
}

func readLine(reader *bufio.Reader, defaultVal string) string {
	line, _ := reader.ReadString('\n')
	line = strings.TrimSpace(line)
	if line == "" {
		return defaultVal
	}
	return line
}
