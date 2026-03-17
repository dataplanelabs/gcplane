package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// Provider templates for init command.
type providerTemplate struct {
	displayName  string
	providerType string
	apiBase      string
	envVar       string
	defaultModel string
}

var providers = map[string]providerTemplate{
	"anthropic": {
		displayName: "Anthropic", providerType: "anthropic",
		apiBase: "https://api.anthropic.com", envVar: "ANTHROPIC_API_KEY",
		defaultModel: "claude-sonnet-4-20250514",
	},
	"openai": {
		displayName: "OpenAI", providerType: "openai",
		apiBase: "https://api.openai.com/v1", envVar: "OPENAI_API_KEY",
		defaultModel: "gpt-4o",
	},
	"openrouter": {
		displayName: "OpenRouter", providerType: "openrouter",
		apiBase: "https://openrouter.ai/api/v1", envVar: "OPENROUTER_API_KEY",
		defaultModel: "google/gemini-2.5-flash-preview",
	},
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Generate a starter manifest",
	Long:  "Create a gcplane.yaml manifest with provider, agent, and connection config.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if _, err := os.Stat("gcplane.yaml"); err == nil {
			return fmt.Errorf("gcplane.yaml already exists")
		}

		reader := bufio.NewReader(os.Stdin)

		fmt.Println("GCPlane Init — Generate starter manifest")
		fmt.Println()

		// Deployment name
		fmt.Print("Deployment name [my-setup]: ")
		name := readLine(reader, "my-setup")

		// GoClaw endpoint
		fmt.Print("GoClaw endpoint [http://localhost:18790]: ")
		ep := readLine(reader, "http://localhost:18790")

		// Provider selection
		fmt.Println("\nProvider type:")
		fmt.Println("  1) anthropic")
		fmt.Println("  2) openai")
		fmt.Println("  3) openrouter")
		fmt.Println("  4) custom")
		fmt.Print("Choose [3]: ")
		choice := readLine(reader, "3")

		var prov providerTemplate
		var provName, envVars string

		switch choice {
		case "1":
			prov = providers["anthropic"]
			provName = "anthropic"
		case "2":
			prov = providers["openai"]
			provName = "openai"
		case "4":
			fmt.Print("Provider name: ")
			provName = readLine(reader, "custom")
			fmt.Print("API base URL: ")
			prov.apiBase = readLine(reader, "https://api.example.com/v1")
			prov.envVar = strings.ToUpper(provName) + "_API_KEY"
			prov.displayName = provName
			prov.providerType = "custom"
			fmt.Print("Default model: ")
			prov.defaultModel = readLine(reader, "default")
		default: // 3 or anything else
			prov = providers["openrouter"]
			provName = "openrouter"
		}

		// Model
		fmt.Printf("Model [%s]: ", prov.defaultModel)
		model := readLine(reader, prov.defaultModel)

		// Agent name
		fmt.Print("Agent name [assistant]: ")
		agentName := readLine(reader, "assistant")

		// Generate manifest
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
    name: %s
    spec:
      displayName: "%s"
      providerType: %s
      apiBase: %s
      apiKey: ${%s}
      enabled: true

  - kind: Agent
    name: %s
    spec:
      displayName: "%s"
      provider: %s
      model: %s
      agentType: open
      status: active
      isDefault: true
`, name, ep, provName, prov.displayName, prov.providerType, prov.apiBase, prov.envVar,
			agentName, strings.Title(agentName), provName, model)

		if err := os.WriteFile("gcplane.yaml", []byte(content), 0644); err != nil {
			return fmt.Errorf("write gcplane.yaml: %w", err)
		}
		fmt.Println("\nCreated gcplane.yaml")

		// Generate .env.example
		envVars = fmt.Sprintf("GOCLAW_TOKEN=\n%s=\n\n# Drift notifications (optional)\nGCPLANE_WEBHOOK_URL=\nGCPLANE_WEBHOOK_FORMAT=slack\n", prov.envVar)
		if _, err := os.Stat(".env.example"); err != nil {
			_ = os.WriteFile(".env.example", []byte(envVars), 0644)
			fmt.Println("Created .env.example")
		}

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
