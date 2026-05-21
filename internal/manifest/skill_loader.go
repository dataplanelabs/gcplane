package manifest

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// systemTenant is the directory name that marks global (cross-tenant) skills.
// _system/skills/<slug>/ becomes visibility=public; any other tenant directory
// produces visibility=tenant skills.
const systemTenant = "_system"

// skillsSubdir is the conventional subdirectory under a tenant root where skill
// directories live.
const skillsSubdir = "skills"

// skillOverridesFile is the conventional file under a tenant root that toggles
// global skills on/off per tenant.
const skillOverridesFile = "skill-overrides.yaml"

// skillFrontmatter mirrors the SKILL.md frontmatter goclaw parses on upload.
// Only `name` is strictly required by goclaw; `description` is strongly
// recommended for searchability.
type skillFrontmatter struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

// skillOverlay is loaded from an optional frontmatter.yaml co-located with
// SKILL.md. It carries fields gcplane needs at apply time but that don't fit
// inside SKILL.md's frontmatter (e.g. tags, status, per-agent grants).
type skillOverlay struct {
	Tags   []string             `yaml:"tags,omitempty"`
	Status string               `yaml:"status,omitempty"`
	Grants *skillGrantsOverlay  `yaml:"grants,omitempty"`
}

// skillGrantsOverlay mirrors the MCPServer `grants.agents` shape so per-agent
// skill enablement is authored the same way across the manifest.
type skillGrantsOverlay struct {
	Agents []string `yaml:"agents,omitempty"`
}

// skillOverridesDoc is the schema of <tenant>/skill-overrides.yaml.
type skillOverridesDoc struct {
	Overrides map[string]struct {
		Enabled bool `yaml:"enabled"`
	} `yaml:"overrides"`
}

// LoadSkillResources walks dir for skill directories and a skill-overrides.yaml
// file, returning synthesized KindSkill + KindSkillConfig resources.
//
// Layout:
//
//	<dir>/skills/<slug>/SKILL.md          → KindSkill (visibility derived from dir basename)
//	<dir>/skills/<slug>/frontmatter.yaml  → optional overlay (tags, status)
//	<dir>/skill-overrides.yaml            → KindSkillConfig per entry
//
// When the dir basename equals `_system`, synthesized skills are tagged
// visibility=public; otherwise visibility=tenant.
// Returns an empty slice (not nil) when no skills are found, for backward
// compatibility with existing YAML-only deployments.
func LoadSkillResources(dir string) ([]Resource, error) {
	out := make([]Resource, 0)

	scope := "tenant"
	if filepath.Base(dir) == systemTenant {
		scope = "global"
	}

	skillsDir := filepath.Join(dir, skillsSubdir)
	if info, err := os.Stat(skillsDir); err == nil && info.IsDir() {
		entries, err := os.ReadDir(skillsDir)
		if err != nil {
			return nil, fmt.Errorf("readdir %s: %w", skillsDir, err)
		}
		seen := make(map[string]string, len(entries))
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			if strings.HasPrefix(e.Name(), ".") {
				continue
			}
			skillDir := filepath.Join(skillsDir, e.Name())
			res, err := loadSkillDir(skillDir, e.Name(), scope)
			if err != nil {
				return nil, err
			}
			if prior, dup := seen[res.Name]; dup {
				return nil, fmt.Errorf("duplicate skill slug %q at %s and %s", res.Name, prior, skillDir)
			}
			seen[res.Name] = skillDir
			out = append(out, res)
		}
	}

	overridesPath := filepath.Join(dir, skillOverridesFile)
	if data, err := os.ReadFile(overridesPath); err == nil {
		var doc skillOverridesDoc
		if err := yaml.Unmarshal(data, &doc); err != nil {
			return nil, fmt.Errorf("parse %s: %w", overridesPath, err)
		}
		for slug, ov := range doc.Overrides {
			out = append(out, Resource{
				Kind: KindSkillConfig,
				Name: slug,
				Spec: map[string]any{"enabled": ov.Enabled},
			})
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("read %s: %w", overridesPath, err)
	}

	return out, nil
}

func loadSkillDir(skillDir, slug, scope string) (Resource, error) {
	skillMDPath := filepath.Join(skillDir, "SKILL.md")
	mdBytes, err := os.ReadFile(skillMDPath)
	if err != nil {
		return Resource{}, fmt.Errorf("read %s: %w", skillMDPath, err)
	}

	fm, err := parseFrontmatter(mdBytes)
	if err != nil {
		return Resource{}, fmt.Errorf("%s: %w", skillMDPath, err)
	}
	if fm.Name == "" {
		return Resource{}, fmt.Errorf("%s: frontmatter is missing required field 'name'", skillMDPath)
	}

	visibility := "tenant"
	if scope == "global" {
		visibility = "public"
	}

	spec := map[string]any{
		"description": fm.Description,
		"visibility":  visibility,
		"sourceDir":   skillDir,
	}

	overlayPath := filepath.Join(skillDir, "frontmatter.yaml")
	if data, err := os.ReadFile(overlayPath); err == nil {
		var overlay skillOverlay
		if err := yaml.Unmarshal(data, &overlay); err != nil {
			return Resource{}, fmt.Errorf("%s: %w", overlayPath, err)
		}
		if overlay.Status != "" {
			spec["status"] = overlay.Status
		}
		if len(overlay.Tags) > 0 {
			tags := make([]any, len(overlay.Tags))
			for i, t := range overlay.Tags {
				tags[i] = t
			}
			spec["tags"] = tags
		}
		if overlay.Grants != nil && len(overlay.Grants.Agents) > 0 {
			agents := make([]any, len(overlay.Grants.Agents))
			for i, a := range overlay.Grants.Agents {
				agents[i] = a
			}
			spec["grants"] = map[string]any{"agents": agents}
		}
	} else if !os.IsNotExist(err) {
		return Resource{}, fmt.Errorf("read %s: %w", overlayPath, err)
	}

	return Resource{
		Kind: KindSkill,
		Name: slug,
		Spec: spec,
	}, nil
}

// parseFrontmatter extracts the YAML block delimited by leading and closing
// `---` lines. The remaining body is discarded — gcplane doesn't inspect it,
// goclaw re-parses the full SKILL.md on upload.
func parseFrontmatter(content []byte) (skillFrontmatter, error) {
	var fm skillFrontmatter

	trimmed := bytes.TrimPrefix(content, []byte{0xEF, 0xBB, 0xBF}) // strip UTF-8 BOM
	trimmed = bytes.TrimLeft(trimmed, " \t\r\n")
	// Normalize CRLF → LF so fence detection is line-oriented on Windows files.
	normalized := bytes.ReplaceAll(trimmed, []byte("\r\n"), []byte("\n"))

	// Require the opening fence to occupy its own line: `---\n` (or `---` then EOF).
	// Otherwise content like `---foo\n` would be mis-parsed.
	if !bytes.HasPrefix(normalized, []byte("---\n")) && !bytes.Equal(normalized, []byte("---")) {
		return fm, fmt.Errorf("SKILL.md must start with a '---' frontmatter fence on its own line")
	}
	rest := normalized[4:] // skip "---\n"

	// Closing fence: a line containing exactly `---`.
	closeIdx := bytes.Index(rest, []byte("\n---\n"))
	if closeIdx < 0 {
		// Allow trailing `---` with no newline (file ends right after fence).
		if i := bytes.LastIndex(rest, []byte("\n---")); i >= 0 && i+4 == len(rest) {
			closeIdx = i
		} else {
			return fm, fmt.Errorf("SKILL.md frontmatter is missing closing '---' fence")
		}
	}
	block := rest[:closeIdx]

	if err := yaml.Unmarshal(block, &fm); err != nil {
		return fm, fmt.Errorf("parse frontmatter: %w", err)
	}
	return fm, nil
}
