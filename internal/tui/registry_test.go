package tui

import (
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

	r.Push("main")
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

	// Pop past empty returns "main"
	page = r.Pop()
	if page != "main" {
		t.Errorf("pop past empty should return main, got %s", page)
	}
}
