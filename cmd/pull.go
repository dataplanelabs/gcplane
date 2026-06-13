package cmd

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/dataplanelabs/gcplane/internal/manifest"
	"github.com/dataplanelabs/gcplane/internal/provider/goclaw"
	"github.com/dataplanelabs/gcplane/internal/reconciler"
	"github.com/dataplanelabs/gcplane/internal/skillpkg"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	pullKinds      []string
	pullAll        bool
	pullDryRun     bool
	pullPruneFiles bool
)

var pullCmd = &cobra.Command{
	Use:   "pull",
	Short: "Reverse-sync evolved skills & agent context files from GoClaw into the repo",
	Long: `Pulls the current state of skills and agent context files from a live GoClaw
instance into the local manifest directory. Intended for reviewing self-evolved
artifacts before committing them back to git.

Default behavior (without --all) pulls only skills with source=gcplane or
source=evolution, and only agents already present in the local manifest.
Use --dry-run to preview changes without writing anything.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		m, err := loadAndValidateManifest()
		if err != nil {
			return err
		}

		ep, tok, err := resolveConnection(m)
		if err != nil {
			return err
		}
		provOpts, err := resolveProviderOpts(m)
		if err != nil {
			return err
		}

		p := goclaw.New(ep, tok, provOpts...)
		defer p.Close()

		ctx := cmd.Context()

		if wantKind(pullKinds, "skill") {
			if err := pullSkills(ctx, p, m, pullAll, pullDryRun, pullPruneFiles); err != nil {
				return err
			}
		}
		if wantKind(pullKinds, "context-files") {
			if err := pullContextFiles(ctx, p, m, pullDryRun); err != nil {
				return err
			}
		}
		return nil
	},
}

func init() {
	pullCmd.Flags().StringArrayVar(&pullKinds, "kind", nil, "restrict to kind(s): skill, context-files (default: both)")
	pullCmd.Flags().BoolVar(&pullAll, "all", false, "pull all skills regardless of source (default: gcplane+evolution only)")
	pullCmd.Flags().BoolVar(&pullDryRun, "dry-run", false, "print what would change without writing")
	pullCmd.Flags().BoolVar(&pullPruneFiles, "prune-skill-files", false, "delete local skill files absent from server (default: off)")
}

// wantKind returns true when kinds is empty (= all) or contains k.
func wantKind(kinds []string, k string) bool {
	if len(kinds) == 0 {
		return true
	}
	for _, v := range kinds {
		if strings.EqualFold(v, k) {
			return true
		}
	}
	return false
}

// shouldPullSkill decides whether a server-side skill is reverse-synced.
// Default: only source=gcplane|evolution skills already declared locally.
// --all: everything the token can see except system/bundled skills.
func shouldPullSkill(info reconciler.ResourceInfo, all, localKnown bool) bool {
	if all {
		return !info.IsSystem && info.Source != "bundled"
	}
	if info.Source != "gcplane" && info.Source != "evolution" {
		return false
	}
	return localKnown
}

// pullSkills downloads skill file trees for gcplane/evolution-sourced skills
// and writes them into the manifest's sourceDir layout.
func pullSkills(ctx context.Context, p *goclaw.Provider, m *manifest.Manifest, all, dryRun, pruneFiles bool) error {
	infos, err := p.ListAll(ctx, manifest.KindSkill)
	if err != nil {
		return fmt.Errorf("list skills: %w", err)
	}

	// Build set of slugs declared in local manifest so we don't invent new skills.
	localSlugs := make(map[string]string) // slug → sourceDir
	for _, r := range m.Resources {
		if r.Kind != manifest.KindSkill {
			continue
		}
		if sd, _ := r.Spec["sourceDir"].(string); sd != "" {
			localSlugs[r.Name] = sd
		}
	}

	pulled := 0
	for _, info := range infos {
		slug := info.Name

		_, localKnown := localSlugs[slug]
		if !shouldPullSkill(info, all, localKnown) {
			continue
		}

		sourceDir, known := localSlugs[slug]
		if !known {
			// --all mode: infer sourceDir from the manifest config path.
			sourceDir = inferSourceDir(configFile, slug)
		}

		files, grantees, err := p.DownloadSkillSource(ctx, slug)
		if err != nil {
			return fmt.Errorf("download skill %s: %w", slug, err)
		}

		if dryRun {
			printSkillDiff(slug, sourceDir, files)
		} else {
			changed, err := skillpkg.UnpackTo(sourceDir, files, pruneFiles)
			if err != nil {
				return fmt.Errorf("unpack skill %s: %w", slug, err)
			}
			if err := writeFrontmatter(sourceDir, grantees); err != nil {
				return fmt.Errorf("write frontmatter for %s: %w", slug, err)
			}
			if len(changed) > 0 {
				fmt.Printf("skill %s: wrote %d file(s)\n", slug, len(changed))
				for _, f := range changed {
					fmt.Printf("  %s\n", f)
				}
			} else {
				fmt.Printf("skill %s: no changes\n", slug)
			}
		}
		pulled++
	}

	if pulled == 0 {
		fmt.Println("pull skills: no matching skills found")
	}
	return nil
}

// pullContextFiles downloads context files for all Agent resources in the manifest
// and patches them in-place in the agents.yaml files.
func pullContextFiles(ctx context.Context, p *goclaw.Provider, m *manifest.Manifest, dryRun bool) error {
	// Collect agent keys from manifest.
	var agentKeys []string
	for _, r := range m.Resources {
		if r.Kind == manifest.KindAgent {
			agentKeys = append(agentKeys, r.Name)
		}
	}
	if len(agentKeys) == 0 {
		fmt.Println("pull context-files: no Agent resources in manifest")
		return nil
	}

	// Group agents by the YAML file they live in so we can patch per-file.
	// When the manifest is a directory, agents are in agents.yaml inside that dir.
	agentsFile := resolveAgentsFile(configFile)

	for _, agentKey := range agentKeys {
		files, err := p.DownloadAgentContextFiles(ctx, agentKey)
		if err != nil {
			return fmt.Errorf("download context files for %s: %w", agentKey, err)
		}
		if len(files) == 0 {
			fmt.Printf("agent %s: no context files\n", agentKey)
			continue
		}

		if dryRun {
			printContextFileDiff(agentKey, agentsFile, files)
		} else {
			changed, err := patchAgentsYAML(agentsFile, agentKey, files)
			if err != nil {
				return fmt.Errorf("patch agents.yaml for %s: %w", agentKey, err)
			}
			if changed {
				fmt.Printf("agent %s: context files updated in %s\n", agentKey, agentsFile)
			} else {
				fmt.Printf("agent %s: context files unchanged\n", agentKey)
			}
		}
	}
	return nil
}

// resolveAgentsFile returns the agents.yaml path relative to the manifest dir.
func resolveAgentsFile(cfgPath string) string {
	if cfgPath == "" {
		return "agents.yaml"
	}
	info, err := os.Stat(cfgPath)
	if err == nil && info.IsDir() {
		return filepath.Join(cfgPath, "agents.yaml")
	}
	return filepath.Join(filepath.Dir(cfgPath), "agents.yaml")
}

// inferSourceDir builds a sourceDir for a skill not already in the local manifest.
// Falls back to <manifestdir>/skills/<slug>/.
func inferSourceDir(cfgPath, slug string) string {
	base := cfgPath
	if cfgPath != "" {
		info, err := os.Stat(cfgPath)
		if err == nil && !info.IsDir() {
			base = filepath.Dir(cfgPath)
		}
	}
	return filepath.Join(base, "skills", slug)
}

// writeFrontmatter writes a frontmatter.yaml in sourceDir with grants.agents sorted.
// Skips writing if grantees is empty (avoids creating an empty file).
func writeFrontmatter(sourceDir string, grantees []string) error {
	outPath := filepath.Join(sourceDir, "frontmatter.yaml")
	if len(grantees) == 0 {
		// Grants revoked server-side: drop stale frontmatter so apply won't re-grant.
		if err := os.Remove(outPath); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	sort.Strings(grantees)

	type grantsOverlay struct {
		Grants struct {
			Agents []string `yaml:"agents"`
		} `yaml:"grants"`
	}
	var doc grantsOverlay
	doc.Grants.Agents = grantees

	data, err := yaml.Marshal(&doc)
	if err != nil {
		return err
	}

	existing, _ := os.ReadFile(outPath)
	if bytes.Equal(existing, data) {
		return nil
	}
	return os.WriteFile(outPath, data, 0o644)
}

// patchAgentsYAML updates the contextFiles for a named agent in agents.yaml in-place.
// Only rewrites the file when content actually differs to avoid comment churn.
// Returns true if the file was modified.
func patchAgentsYAML(path, agentKey string, files []map[string]string) (bool, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return false, fmt.Errorf("read %s: %w", path, err)
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return false, fmt.Errorf("parse %s: %w", path, err)
	}

	// Navigate: doc is a document node whose Content[0] is the mapping.
	root := docRoot(&doc)
	if root == nil {
		return false, fmt.Errorf("empty or invalid YAML in %s", path)
	}

	// Find the resources sequence.
	resources := mappingValue(root, "resources")
	if resources == nil || resources.Kind != yaml.SequenceNode {
		return false, fmt.Errorf("no 'resources' sequence in %s", path)
	}

	// Find the Agent node matching agentKey.
	agentNode := findAgent(resources, agentKey)
	if agentNode == nil {
		// Agent not in this file — silently skip.
		return false, nil
	}

	// Check if existing content already matches (hash compare).
	if !contextFilesChanged(agentNode, files) {
		return false, nil
	}

	// Build the new contextFiles YAML node.
	newCF := buildContextFilesNode(files)

	// Replace or insert contextFiles in the agent's spec.
	specNode := mappingValue(agentNode, "spec")
	if specNode == nil {
		return false, fmt.Errorf("agent %s has no spec in %s", agentKey, path)
	}
	setMappingValue(specNode, "contextFiles", newCF)

	out, err := yaml.Marshal(&doc)
	if err != nil {
		return false, fmt.Errorf("marshal %s: %w", path, err)
	}
	if err := os.WriteFile(path, out, 0o644); err != nil {
		return false, fmt.Errorf("write %s: %w", path, err)
	}
	return true, nil
}

// contextFilesChanged returns true when the agent's current contextFiles in the
// YAML node differ from the server-fetched files (by SHA-256 of content).
func contextFilesChanged(agentNode *yaml.Node, files []map[string]string) bool {
	specNode := mappingValue(agentNode, "spec")
	if specNode == nil {
		return true
	}
	cfNode := mappingValue(specNode, "contextFiles")
	if cfNode == nil || cfNode.Kind != yaml.SequenceNode {
		return true
	}

	// Extract current name→contentHash from YAML.
	current := make(map[string][32]byte)
	for _, item := range cfNode.Content {
		if item.Kind != yaml.MappingNode {
			continue
		}
		name := mappingStringValue(item, "name")
		content := mappingStringValue(item, "content")
		if name != "" {
			current[name] = sha256.Sum256([]byte(content))
		}
	}

	if len(current) != len(files) {
		return true
	}
	for _, f := range files {
		want := sha256.Sum256([]byte(f["content"]))
		if got, ok := current[f["name"]]; !ok || got != want {
			return true
		}
	}
	return false
}

// buildContextFilesNode creates a YAML sequence node for contextFiles using
// literal block scalars for content to match the repo's authoring style.
func buildContextFilesNode(files []map[string]string) *yaml.Node {
	seq := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
	for _, f := range files {
		item := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		nameKey := &yaml.Node{Kind: yaml.ScalarNode, Value: "name"}
		nameVal := &yaml.Node{Kind: yaml.ScalarNode, Value: f["name"]}
		contentKey := &yaml.Node{Kind: yaml.ScalarNode, Value: "content"}
		contentVal := &yaml.Node{
			Kind:  yaml.ScalarNode,
			Value: f["content"],
			Style: yaml.LiteralStyle,
		}
		item.Content = append(item.Content, nameKey, nameVal, contentKey, contentVal)
		seq.Content = append(seq.Content, item)
	}
	return seq
}

// docRoot unwraps a yaml.Node document to its root mapping.
func docRoot(n *yaml.Node) *yaml.Node {
	if n == nil {
		return nil
	}
	if n.Kind == yaml.DocumentNode && len(n.Content) > 0 {
		return n.Content[0]
	}
	return n
}

// mappingValue returns the value node for key in a mapping node.
func mappingValue(m *yaml.Node, key string) *yaml.Node {
	if m == nil || m.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1]
		}
	}
	return nil
}

// mappingStringValue returns the string value of key in a mapping node, or "".
func mappingStringValue(m *yaml.Node, key string) string {
	v := mappingValue(m, key)
	if v == nil {
		return ""
	}
	return v.Value
}

// setMappingValue sets (or replaces) key→val in a mapping node.
func setMappingValue(m *yaml.Node, key string, val *yaml.Node) {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			m.Content[i+1] = val
			return
		}
	}
	keyNode := &yaml.Node{Kind: yaml.ScalarNode, Value: key}
	m.Content = append(m.Content, keyNode, val)
}

// findAgent returns the mapping node for the Agent resource with agentKey.
func findAgent(resources *yaml.Node, agentKey string) *yaml.Node {
	for _, item := range resources.Content {
		if item.Kind != yaml.MappingNode {
			continue
		}
		kind := mappingStringValue(item, "kind")
		name := mappingStringValue(item, "name")
		if kind == "Agent" && name == agentKey {
			return item
		}
	}
	return nil
}

// printSkillDiff prints a summary of what would change for a skill.
func printSkillDiff(slug, sourceDir string, files []skillpkg.SkillFile) {
	fmt.Printf("[dry-run] skill %s → %s (%d files)\n", slug, sourceDir, len(files))
	for _, f := range files {
		abs := filepath.Join(sourceDir, filepath.FromSlash(f.Path))
		existing, err := os.ReadFile(abs)
		if err != nil || !bytes.Equal(existing, f.Data) {
			fmt.Printf("  ~ %s\n", f.Path)
		}
	}
}

// printContextFileDiff prints a summary of what would change for an agent's context files.
func printContextFileDiff(agentKey, agentsFile string, files []map[string]string) {
	fmt.Printf("[dry-run] agent %s context-files in %s:\n", agentKey, agentsFile)
	for _, f := range files {
		fmt.Printf("  ~ %s (%d bytes)\n", f["name"], len(f["content"]))
	}
}
