package goclaw

import "testing"

func TestIsSystemSourceDir(t *testing.T) {
	cases := map[string]bool{
		"_system":                                 true,
		"_system/skills/gh-read":                  true,
		"_system/skills/gws-calendar-read":        true,
		"./_system/skills/foo":                    true,
		"/abs/path/_system/skills/bar":            true,
		"goclaw-config/_system/skills/baz":        true,

		"annhien/skills/sales-of-day":             false,
		"tenant/skills/_system_lookalike":         false,
		"./skills/_system-fake/x":                 false,
		"":                                        false,
		"skills/something":                        false,
	}
	for in, want := range cases {
		if got := isSystemSourceDir(in); got != want {
			t.Errorf("isSystemSourceDir(%q) = %v, want %v", in, got, want)
		}
	}
}
