package tui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/dataplanelabs/gcplane/internal/manifest"
	"github.com/dataplanelabs/gcplane/internal/reconciler"
	"github.com/dataplanelabs/gcplane/internal/tui/views"
	"gopkg.in/yaml.v3"
)

// applyAll triggers a real reconciliation (non-dry-run) with confirmation.
func (a *App) applyAll() {
	plan := a.model.GetPlan()
	if plan == nil {
		return
	}
	pending := plan.Creates + plan.Updates
	if pending == 0 {
		a.showStatus(views.Tag(views.HexGreen, "All resources in sync. Nothing to apply."))
		return
	}

	msg := fmt.Sprintf("Apply %d change(s)?\n(%d create, %d update)",
		pending, plan.Creates, plan.Updates)

	a.confirm.Show(msg, func(confirmed bool) {
		a.popView() // return from confirm to previous view
		if !confirmed {
			return
		}
		a.showStatus(views.Tag(views.HexYellow, "Applying..."))
		go a.doApply()
	})
	a.pushView("confirm")
	a.tapp.SetFocus(a.confirm.Modal)
}

// doApply runs the actual reconciliation in a goroutine.
func (a *App) doApply() {
	plan, result := a.Engine.Reconcile(context.Background(), a.Manifest, reconciler.ReconcileOpts{DryRun: false})

	// Refresh to show updated state
	a.refresh()

	a.tapp.QueueUpdateDraw(func() {
		summary := fmt.Sprintf("Applied: %d, Failed: %d, Creates: %d, Updates: %d",
			result.Applied, result.Failed, plan.Creates, plan.Updates)
		if len(result.Errors) > 0 {
			summary += " | Errors: " + result.Errors[0]
		}
		if result.Failed > 0 || result.Applied == 0 {
			a.showStatus(views.Tag(views.HexRed, summary))
		} else {
			a.showStatus(views.Tag(views.HexGreen, summary))
		}
	})
}

// deleteResource deletes the selected resource with confirmation.
func (a *App) deleteResource() {
	c := a.table.GetSelectedChange()
	if c == nil {
		return
	}

	msg := fmt.Sprintf("Delete %s/%s from GoClaw?", c.Kind, c.Name)

	a.confirm.Show(msg, func(confirmed bool) {
		a.popView() // return from confirm to previous view
		if !confirmed {
			return
		}
		a.showStatus(views.Tag(views.HexYellow, fmt.Sprintf("Deleting %s/%s...", c.Kind, c.Name)))
		go a.doDelete(c.Kind, c.Name)
	})
	a.pushView("confirm")
	a.tapp.SetFocus(a.confirm.Modal)
}

// doDelete runs the delete operation in a goroutine.
func (a *App) doDelete(kind manifest.ResourceKind, name string) {
	err := a.Provider.Delete(context.Background(), kind, name)

	// Refresh to show updated state
	a.refresh()

	a.tapp.QueueUpdateDraw(func() {
		if err != nil {
			a.showStatus(views.Tag(views.HexRed, fmt.Sprintf("Delete failed: %s", err)))
		} else {
			a.showStatus(views.Tag(views.HexGreen, fmt.Sprintf("Deleted %s/%s", kind, name)))
		}
	})
}

// editResource opens $EDITOR with the resource YAML, then applies changes.
func (a *App) editResource() {
	c := a.table.GetSelectedChange()
	if c == nil {
		return
	}

	// Observe current spec from GoClaw
	observed, err := a.Provider.Observe(context.Background(), c.Kind, c.Name)
	if err != nil {
		a.showStatus(views.Tag(views.HexRed, fmt.Sprintf("Cannot edit: %s", err)))
		return
	}

	// Build editable YAML document with kind/name header
	doc := map[string]any{
		"kind": string(c.Kind),
		"name": c.Name,
		"spec": observed,
	}

	yamlBytes, err := yaml.Marshal(doc)
	if err != nil {
		a.showStatus(views.Tag(views.HexRed, fmt.Sprintf("Marshal error: %s", err)))
		return
	}

	// Write to temp file
	tmpFile, err := os.CreateTemp("", fmt.Sprintf("gcplane-%s-%s-*.yaml", c.Kind, c.Name))
	if err != nil {
		a.showStatus(views.Tag(views.HexRed, fmt.Sprintf("Temp file error: %s", err)))
		return
	}
	tmpPath := tmpFile.Name()
	defer func() { _ = os.Remove(tmpPath) }()

	if _, err := tmpFile.Write(yamlBytes); err != nil {
		_ = tmpFile.Close()
		a.showStatus(views.Tag(views.HexRed, fmt.Sprintf("Write error: %s", err)))
		return
	}
	_ = tmpFile.Close()

	// Determine editor
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}

	// Suspend TUI, open editor, resume TUI
	a.tapp.Suspend(func() {
		cmd := exec.Command(editor, tmpPath)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		_ = cmd.Run()
	})

	// Read edited file
	edited, err := os.ReadFile(tmpPath)
	if err != nil {
		a.showStatus(views.Tag(views.HexRed, fmt.Sprintf("Read error: %s", err)))
		return
	}

	// Parse edited YAML
	var result map[string]any
	if err := yaml.Unmarshal(edited, &result); err != nil {
		a.showStatus(views.Tag(views.HexRed, fmt.Sprintf("Invalid YAML: %s", err)))
		return
	}

	spec, ok := result["spec"].(map[string]any)
	if !ok {
		a.showStatus(views.Tag(views.HexRed, "Missing or invalid 'spec' in edited YAML"))
		return
	}

	// Apply the update
	go func() {
		err := a.Provider.Update(context.Background(), c.Kind, c.Name, spec)
		a.refresh()

		a.tapp.QueueUpdateDraw(func() {
			if err != nil {
				a.showStatus(views.Tag(views.HexRed, fmt.Sprintf("Update failed: %s", err)))
			} else {
				a.showStatus(views.Tag(views.HexGreen, fmt.Sprintf("Updated %s/%s", c.Kind, c.Name)))
			}
		})
	}()
}

// showStatus displays a temporary message in the command bar, auto-clearing after 5 seconds.
func (a *App) showStatus(msg string) {
	a.cmdBar.SetLabel(msg)
	a.cmdBar.SetText("")
	go func() {
		time.Sleep(5 * time.Second)
		a.tapp.QueueUpdateDraw(func() {
			// Only clear if the label still matches (hasn't been replaced by a newer message)
			if a.cmdBar.GetLabel() == msg {
				a.cmdBar.SetLabel(":")
			}
		})
	}()
}
