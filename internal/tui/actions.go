package tui

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/dataplanelabs/gcplane/internal/manifest"
	"github.com/dataplanelabs/gcplane/internal/reconciler"
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
		a.showStatus("[green]All resources in sync. Nothing to apply.[-]")
		return
	}

	msg := fmt.Sprintf("Apply %d change(s)?\n(%d create, %d update)",
		pending, plan.Creates, plan.Updates)

	a.confirm.Show(msg, func(confirmed bool) {
		a.pages.SwitchToPage("main")
		a.tapp.SetFocus(a.table.Table)
		if !confirmed {
			return
		}
		a.showStatus("[yellow]Applying...[-]")
		go a.doApply()
	})
	a.pages.SwitchToPage("confirm")
	a.tapp.SetFocus(a.confirm.Modal)
}

// doApply runs the actual reconciliation in a goroutine.
func (a *App) doApply() {
	_, result := a.Engine.Reconcile(a.Manifest, reconciler.ReconcileOpts{DryRun: false})

	// Refresh to show updated state
	a.refresh()

	a.tapp.QueueUpdateDraw(func() {
		if result.Failed > 0 {
			a.showStatus(fmt.Sprintf("[red]Applied: %d, Failed: %d[-]", result.Applied, result.Failed))
		} else {
			a.showStatus(fmt.Sprintf("[green]Applied %d change(s) successfully[-]", result.Applied))
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
		a.pages.SwitchToPage("main")
		a.tapp.SetFocus(a.table.Table)
		if !confirmed {
			return
		}
		a.showStatus(fmt.Sprintf("[yellow]Deleting %s/%s...[-]", c.Kind, c.Name))
		go a.doDelete(c.Kind, c.Name)
	})
	a.pages.SwitchToPage("confirm")
	a.tapp.SetFocus(a.confirm.Modal)
}

// doDelete runs the delete operation in a goroutine.
func (a *App) doDelete(kind manifest.ResourceKind, name string) {
	err := a.Provider.Delete(kind, name)

	// Refresh to show updated state
	a.refresh()

	a.tapp.QueueUpdateDraw(func() {
		if err != nil {
			a.showStatus(fmt.Sprintf("[red]Delete failed: %s[-]", err))
		} else {
			a.showStatus(fmt.Sprintf("[green]Deleted %s/%s[-]", kind, name))
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
	observed, err := a.Provider.Observe(c.Kind, c.Name)
	if err != nil {
		a.showStatus(fmt.Sprintf("[red]Cannot edit: %s[-]", err))
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
		a.showStatus(fmt.Sprintf("[red]Marshal error: %s[-]", err))
		return
	}

	// Write to temp file
	tmpFile, err := os.CreateTemp("", fmt.Sprintf("gcplane-%s-%s-*.yaml", c.Kind, c.Name))
	if err != nil {
		a.showStatus(fmt.Sprintf("[red]Temp file error: %s[-]", err))
		return
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := tmpFile.Write(yamlBytes); err != nil {
		tmpFile.Close()
		a.showStatus(fmt.Sprintf("[red]Write error: %s[-]", err))
		return
	}
	tmpFile.Close()

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
		cmd.Run()
	})

	// Read edited file
	edited, err := os.ReadFile(tmpPath)
	if err != nil {
		a.showStatus(fmt.Sprintf("[red]Read error: %s[-]", err))
		return
	}

	// Parse edited YAML
	var result map[string]any
	if err := yaml.Unmarshal(edited, &result); err != nil {
		a.showStatus(fmt.Sprintf("[red]Invalid YAML: %s[-]", err))
		return
	}

	spec, ok := result["spec"].(map[string]any)
	if !ok {
		a.showStatus("[red]Missing or invalid 'spec' in edited YAML[-]")
		return
	}

	// Apply the update
	go func() {
		err := a.Provider.Update(c.Kind, c.Name, spec)
		a.refresh()

		a.tapp.QueueUpdateDraw(func() {
			if err != nil {
				a.showStatus(fmt.Sprintf("[red]Update failed: %s[-]", err))
			} else {
				a.showStatus(fmt.Sprintf("[green]Updated %s/%s[-]", c.Kind, c.Name))
			}
		})
	}()
}

// showStatus displays a temporary message in the command bar.
func (a *App) showStatus(msg string) {
	a.cmdBar.SetLabel(msg)
	a.cmdBar.SetText("")
}
