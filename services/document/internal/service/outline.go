package service

import "strconv"

// RenumberOutline recomputes Level (depth-based) and Numbering (dotted,
// e.g. "1", "1.1", "1.2", "2") for every node in the tree, depth-first,
// in sibling order. It returns a new slice; the input is not mutated.
func RenumberOutline(nodes []ReportOutlineNode) []ReportOutlineNode {
	return renumberSiblings(nodes, 1, "")
}

func renumberSiblings(nodes []ReportOutlineNode, level int, prefix string) []ReportOutlineNode {
	result := make([]ReportOutlineNode, len(nodes))
	for i, node := range nodes {
		numbering := strconv.Itoa(i + 1)
		if prefix != "" {
			numbering = prefix + "." + numbering
		}
		node.Level = level
		node.Numbering = numbering
		if len(node.Children) > 0 {
			node.Children = renumberSiblings(node.Children, level+1, numbering)
		}
		result[i] = node
	}
	return result
}

// RemoveOutlineNode removes the node matching targetID (and its subtree)
// from the tree, wherever it appears, returning the updated tree and
// whether a node was actually removed.
func RemoveOutlineNode(nodes []ReportOutlineNode, targetID string) ([]ReportOutlineNode, bool) {
	result := make([]ReportOutlineNode, 0, len(nodes))
	removed := false
	for _, node := range nodes {
		if node.ID == targetID {
			removed = true
			continue
		}
		if len(node.Children) > 0 {
			children, childRemoved := RemoveOutlineNode(node.Children, targetID)
			if childRemoved {
				removed = true
				node.Children = children
			}
		}
		result = append(result, node)
	}
	return result, removed
}

// CountOutlineNodes returns the total number of nodes in the tree,
// including nested children.
func CountOutlineNodes(nodes []ReportOutlineNode) int {
	count := 0
	for _, node := range nodes {
		count++
		count += CountOutlineNodes(node.Children)
	}
	return count
}
