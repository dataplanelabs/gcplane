package manifest

import "testing"

func TestFilterByLabels(t *testing.T) {
	resources := []Resource{
		{Name: "a", Labels: map[string]string{"team": "data"}},
		{Name: "b", Labels: map[string]string{"team": "product"}},
		{Name: "c", Labels: nil},
	}

	// Single label match
	result := FilterByLabels(resources, map[string]string{"team": "data"})
	if len(result) != 1 || result[0].Name != "a" {
		t.Fatalf("expected 1 result 'a', got %v", result)
	}

	// Empty selector returns all
	all := FilterByLabels(resources, nil)
	if len(all) != 3 {
		t.Fatalf("expected 3, got %d", len(all))
	}

	// No match
	none := FilterByLabels(resources, map[string]string{"team": "ops"})
	if len(none) != 0 {
		t.Fatalf("expected 0 results, got %v", none)
	}

	// Multiple labels — only match when all present
	multi := []Resource{
		{Name: "x", Labels: map[string]string{"team": "data", "role": "engineer"}},
		{Name: "y", Labels: map[string]string{"team": "data", "role": "analyst"}},
	}
	got := FilterByLabels(multi, map[string]string{"team": "data", "role": "engineer"})
	if len(got) != 1 || got[0].Name != "x" {
		t.Fatalf("expected 1 result 'x', got %v", got)
	}
}

func TestParseLabelSelector(t *testing.T) {
	cases := []struct {
		input    string
		expected map[string]string
	}{
		{"", nil},
		{"team=data", map[string]string{"team": "data"}},
		{"team=data,role=engineer", map[string]string{"team": "data", "role": "engineer"}},
		{"team = data", map[string]string{"team": "data"}},
	}

	for _, c := range cases {
		got := ParseLabelSelector(c.input)
		if len(got) != len(c.expected) {
			t.Errorf("input %q: expected %v, got %v", c.input, c.expected, got)
			continue
		}
		for k, v := range c.expected {
			if got[k] != v {
				t.Errorf("input %q: key %q expected %q, got %q", c.input, k, v, got[k])
			}
		}
	}
}
