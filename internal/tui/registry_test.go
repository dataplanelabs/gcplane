package tui

import (
	"strings"
	"testing"

	"github.com/rivo/tview"
)

type stubView struct {
	name string
}

func (v *stubView) Name() string              { return v.name }
func (v *stubView) Primitive() tview.Primitive { return tview.NewTextView() }
func (v *stubView) Activate()                  {}

func TestViewRegistryRegisterAndGet(t *testing.T) {
	pages := tview.NewPages()
	app := tview.NewApplication()
	r := NewViewRegistry(pages, app)

	r.Register(&stubView{name: "alpha"})
	r.Register(&stubView{name: "beta"})

	if r.Count() != 2 {
		t.Errorf("want 2 views, got %d", r.Count())
	}

	if _, ok := r.Get("alpha"); !ok {
		t.Error("alpha should be registered")
	}
	if _, ok := r.Get("missing"); ok {
		t.Error("missing should not be registered")
	}
}

func TestViewRegistryPushPop(t *testing.T) {
	pages := tview.NewPages()
	app := tview.NewApplication()
	r := NewViewRegistry(pages, app)

	r.Register(&stubView{name: "main"})
	r.Register(&stubView{name: "detail"})
	r.Register(&stubView{name: "drift"})

	// Default tab is State, page "main"
	if r.Current() != "main" {
		t.Errorf("want main, got %s", r.Current())
	}

	r.Push("detail")
	if r.Current() != "detail" {
		t.Errorf("want detail, got %s", r.Current())
	}

	r.Push("drift")
	if r.Current() != "drift" {
		t.Errorf("want drift, got %s", r.Current())
	}

	page := r.Pop()
	if page != "detail" {
		t.Errorf("pop should return detail, got %s", page)
	}

	page = r.Pop()
	if page != "main" {
		t.Errorf("pop should return main, got %s", page)
	}

	// Pop past empty returns active tab ("main" for TabState)
	page = r.Pop()
	if page != "main" {
		t.Errorf("pop past empty should return main, got %s", page)
	}
}

func TestViewRegistrySwitchTab(t *testing.T) {
	pages := tview.NewPages()
	app := tview.NewApplication()
	r := NewViewRegistry(pages, app)

	r.Register(&stubView{name: "main"})
	r.Register(&stubView{name: "logs"})
	r.Register(&stubView{name: "events"})
	r.Register(&stubView{name: "trace"})
	r.Register(&stubView{name: "detail"})

	// Default is TabState
	if r.ActiveTab() != TabState {
		t.Fatalf("default tab should be TabState")
	}
	if r.Current() != "main" {
		t.Fatalf("default current should be main, got %s", r.Current())
	}

	// Switch tab
	r.SwitchTab(TabLogs)
	if r.ActiveTab() != TabLogs {
		t.Errorf("want TabLogs")
	}
	if r.Current() != "logs" {
		t.Errorf("want logs, got %s", r.Current())
	}
	if r.HasOverlay() {
		t.Error("should not have overlay after tab switch")
	}

	// Push overlay on Logs tab
	r.Push("detail")
	if !r.HasOverlay() {
		t.Error("want overlay")
	}
	if r.Current() != "detail" {
		t.Errorf("want detail, got %s", r.Current())
	}

	// Pop returns to Logs
	page := r.Pop()
	if page != "logs" {
		t.Errorf("want logs after pop, got %s", page)
	}
	if r.HasOverlay() {
		t.Error("should not have overlay after pop")
	}

	// Switch tab clears overlay
	r.Push("detail")
	r.SwitchTab(TabTrace)
	if r.HasOverlay() {
		t.Error("tab switch should clear overlay")
	}
	if r.Current() != "trace" {
		t.Errorf("want trace, got %s", r.Current())
	}

	// Pop on empty overlay returns to active tab
	page = r.Pop()
	if page != "trace" {
		t.Errorf("pop on empty overlay should return trace, got %s", page)
	}
}

func TestViewRegistryTabBar(t *testing.T) {
	pages := tview.NewPages()
	app := tview.NewApplication()
	r := NewViewRegistry(pages, app)

	r.Register(&stubView{name: "main"})

	bar := r.TabBar()
	if bar == "" {
		t.Error("TabBar should not be empty")
	}
	// Active tab (State) should have filled circle
	if !strings.Contains(bar, "\u25cf") {
		t.Error("TabBar should contain filled circle for active tab")
	}
	// Inactive tabs should have open circle
	if !strings.Contains(bar, "\u25cb") {
		t.Error("TabBar should contain open circle for inactive tabs")
	}
}
