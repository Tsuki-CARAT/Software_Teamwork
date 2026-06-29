package service

import "testing"

func TestRenumberOutlineAssignsDottedNumbering(t *testing.T) {
	nodes := []ReportOutlineNode{
		{ID: "a", Title: "Intro"},
		{
			ID:    "b",
			Title: "Body",
			Children: []ReportOutlineNode{
				{ID: "b1", Title: "Background"},
				{ID: "b2", Title: "Findings"},
			},
		},
		{ID: "c", Title: "Conclusion"},
	}

	got := RenumberOutline(nodes)

	want := map[string]string{"a": "1", "b": "2", "b1": "2.1", "b2": "2.2", "c": "3"}
	flat := flattenNumbering(got)
	for id, numbering := range want {
		if flat[id] != numbering {
			t.Fatalf("numbering[%s] = %q, want %q", id, flat[id], numbering)
		}
	}
	if got[1].Level != 1 || got[1].Children[0].Level != 2 {
		t.Fatalf("levels not recomputed: top=%d child=%d", got[1].Level, got[1].Children[0].Level)
	}
}

func TestRemoveOutlineNodeDropsSubtreeAndRenumbers(t *testing.T) {
	nodes := []ReportOutlineNode{
		{ID: "a", Title: "Intro"},
		{
			ID:    "b",
			Title: "Body",
			Children: []ReportOutlineNode{
				{ID: "b1", Title: "Background"},
				{ID: "b2", Title: "Findings"},
			},
		},
		{ID: "c", Title: "Conclusion"},
	}

	remaining, removed := RemoveOutlineNode(nodes, "b1")
	if !removed {
		t.Fatalf("expected node to be removed")
	}
	if CountOutlineNodes(remaining) != 4 {
		t.Fatalf("expected 4 remaining nodes, got %d", CountOutlineNodes(remaining))
	}

	renumbered := RenumberOutline(remaining)
	flat := flattenNumbering(renumbered)
	if flat["b2"] != "2.1" {
		t.Fatalf("numbering[b2] = %q, want 2.1 after sibling removed", flat["b2"])
	}
	if flat["c"] != "3" {
		t.Fatalf("numbering[c] = %q, want 3", flat["c"])
	}
}

func TestRemoveOutlineNodeNotFound(t *testing.T) {
	nodes := []ReportOutlineNode{{ID: "a", Title: "Intro"}}
	_, removed := RemoveOutlineNode(nodes, "missing")
	if removed {
		t.Fatalf("expected removed = false for missing id")
	}
}

func flattenNumbering(nodes []ReportOutlineNode) map[string]string {
	result := map[string]string{}
	var walk func([]ReportOutlineNode)
	walk = func(nodes []ReportOutlineNode) {
		for _, node := range nodes {
			result[node.ID] = node.Numbering
			walk(node.Children)
		}
	}
	walk(nodes)
	return result
}
