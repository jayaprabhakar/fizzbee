package modelchecker

import "fmt"

func RemoveMergeNodes(root *Node) {
	removed := true
	for removed {
		// The implementation of removeMergeNodes is buggy. It does not remove all the merge nodes.
		// Especially when there are multiple merge nodes pointing to the same node.
		// Temporary hack to calll this multiple times until no more is left
		removed = removeMergeNodes(root, nil, make(map[*Node]bool))
	}
}

func removeMergeNodes(currentNode *Node, parentNode *Node, visited map[*Node]bool) bool {

	if currentNode == nil {
		return false
	}
	if visited[currentNode] {
		return false
	}
	removed := false
	visited[currentNode] = true
	for _, child := range currentNode.outbound {
		if child.Process == nil && len(child.outbound) == 1 {
			for j, p := range parentNode.outbound {
				if p == currentNode {
					parentNode.outbound[j] = child.outbound[0]
				}
			}
			child.outbound[0].inbound = append(child.outbound[0].inbound, parentNode)
			//if parentNode == nil || len(parentNode.outbound) != 1 {
			//	fmt.Printf("parentNode: %p, %s\n", parentNode, parentNode.String())
			//	fmt.Printf("currentNode: %p, %s\n", currentNode, currentNode.String())
			//	fmt.Printf("childNode: %p, %s\n", child, child.String())
			//	panic(fmt.Sprintf("Expecting only one outbound node for the parent node %p, %s", parentNode, parentNode.String()))
			//}

			//child = child.outbound[0]
			removed = true
			removeMergeNodes(child.outbound[0], parentNode, visited)
			continue
		} else if child.Process == nil {
			panic(fmt.Sprintf("Expecting only one outbound node for the parent node %p, %s", parentNode, parentNode.String()))
		} else {
			removed = removed || removeMergeNodes(child, currentNode, visited)
		}
		//removeMergeNodes(child, currentNode, visited)
	}
	return removed
}

func generateDotFile(node *Node, visited map[*Node]bool) string {
	dotGraph := "digraph G {\n"

	var dfs func(n *Node)
	dfs = func(n *Node) {
		if visited[n] {
			return
		}
		visited[n] = true

		nodeID := fmt.Sprintf("\"%p\"", n)

		if n.Process.HasFailedInvariants() {
			// Add node with label and color
			dotGraph += fmt.Sprintf("  %s [label=\"%s\", color=\"red\"];\n", nodeID, n.String())
		} else {
			// Add node with label
			dotGraph += fmt.Sprintf("  %s [label=\"%s\", color=\"black\"];\n", nodeID, n.String())
		}

		// Recursively visit outbound nodes
		for _, child := range n.outbound {
			//for child.Process == nil && len(child.outbound) == 1 {
			//	child = child.outbound[0]
			//}
			childID := fmt.Sprintf("\"%p\"", child)
			dotGraph += fmt.Sprintf("  %s -> %s;\n", nodeID, childID)
			dfs(child)
		}
	}

	dfs(node)
	dotGraph += "}\n"

	return dotGraph
}

// Helper function to print the graph
func printGraph(node *Node) {
	if node == nil {
		return
	}

	name := ""
	if node.Process != nil {
		name = node.Process.Name
	}
	fmt.Printf("Node: %p, Process: %p (%s)\n", node, node.Process, name)
	for _, outbound := range node.outbound {
		fmt.Printf("  -> ")
		printGraph(outbound)
	}
}
