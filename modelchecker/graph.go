package modelchecker

import (
	"fmt"
	"regexp"
)

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
	for _, child := range currentNode.Outbound {
		if child.Node.Process == nil && len(child.Node.Outbound) == 1 {
			for j, p := range parentNode.Outbound {
				if p.Node == currentNode {
					parentNode.Outbound[j].Node = child.Node.Outbound[0].Node
				}
			}
			child.Node.Outbound[0].Node.Inbound = append(child.Node.Outbound[0].Node.Inbound, &Link{Node: parentNode})
			//if parentNode == nil || len(parentNode.Outbound) != 1 {
			//	fmt.Printf("parentNode: %p, %s\n", parentNode, parentNode.String())
			//	fmt.Printf("currentNode: %p, %s\n", currentNode, currentNode.String())
			//	fmt.Printf("childNode: %p, %s\n", child, child.String())
			//	panic(fmt.Sprintf("Expecting only one Outbound node for the parent node %p, %s", parentNode, parentNode.String()))
			//}

			//child = child.Outbound[0]
			removed = true
			removeMergeNodes(child.Node.Outbound[0].Node, parentNode, visited)
			continue
		} else if child.Node.Process == nil {
			panic(fmt.Sprintf("Expecting only one Outbound node for the parent node %p, %s", parentNode, parentNode.String()))
		} else {
			removed = removed || removeMergeNodes(child.Node, currentNode, visited)
		}
		//removeMergeNodes(child, currentNode, visited)
	}
	return removed
}

func generateDotFile(node *Node, visited map[*Node]bool) string {
	re := regexp.MustCompile(`\\+`)
	dotGraph := "digraph G {\n"

	var dfs func(n *Node)
	dfs = func(n *Node) {
		if visited[n] {
			return
		}
		visited[n] = true

		nodeID := fmt.Sprintf("\"%p\"", n)

		color := "black"
		if n.Process.HasFailedInvariants() {
			color = "red"
		}
		if n.Process != nil && n.Process.Witness != nil {
			for _, w := range n.Process.Witness {
				for _, pass := range w {
					if pass {
						color = "green"
						// Ideally this should break from outerloop, for now okay. not sure if go has labelled stmts
						break
					}
				}
			}
		}
		penwidth := 1
		if len(n.Threads) == 0 {
			penwidth = 2
		}
		stateString := re.ReplaceAllString(n.String(), "\\")
		dotGraph += fmt.Sprintf("  %s [label=\"%s\", color=\"%s\" penwidth=\"%d\" ];\n", nodeID, stateString, color, penwidth)

		// Recursively visit Outbound nodes
		for _, child := range n.Outbound {
			//for child.Process == nil && len(child.Outbound) == 1 {
			//	child = child.Outbound[0]
			//}
			childID := fmt.Sprintf("\"%p\"", child.Node)
			//if color != "green" {
			dotGraph += fmt.Sprintf("  %s -> %s [label=\"%s\"];\n", nodeID, childID, child.Name)
			//}

			dfs(child.Node)
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
	for _, outbound := range node.Outbound {
		fmt.Printf("  -> ")
		printGraph(outbound.Node)
	}
}
