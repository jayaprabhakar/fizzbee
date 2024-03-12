package modelchecker

import (
	ast "fizz/proto"
	"fmt"
	"github.com/jayaprabhakar/fizzbee/lib"
	"go.starlark.net/starlark"
	"maps"
	"slices"
)

type InvariantPosition struct {
	FileIndex int
	InvariantIndex int
}

func NewInvariantPosition(fileIndex, invariantIndex int) *InvariantPosition {
	return &InvariantPosition{
		FileIndex: fileIndex,
		InvariantIndex: invariantIndex,
	}
}

func CheckInvariants(process *Process) map[int][]int {
	if len(process.Files) > 1 {
		panic("Invariant checking not supported for multiple files")
	}
	results := make(map[int][]int)
	for i, file := range process.Files {
		results[i] = make([]int, 0)
		for j, invariant := range file.Invariants {
			passed := false
			if invariant.Block == nil {
				passed = CheckInvariant(process, invariant)
				if invariant.Eventually && passed && len(process.Threads) == 0 {
					process.Witness[i][j] = true
				} else if !invariant.Eventually && !passed {
					results[i] = append(results[i], j)
				}
			} else {
				passed = CheckAssertion(process, invariant)
				if slices.Contains(invariant.TemporalOperators, "eventually") && passed && len(process.Threads) == 0  {
					process.Witness[i][j] = true
				} else if !slices.Contains(invariant.TemporalOperators, "eventually") && !passed {
					results[i] = append(results[i], j)
				}
			}
		}
	}
	return results
}

func CheckInvariant(process *Process, invariant *ast.Invariant) bool {
	eventuallyAlways := invariant.Eventually && invariant.GetNested().GetAlways()
	if !invariant.Always && !(eventuallyAlways){
		panic("Invariant checking not supported for non-always invariants")
	}
	if !eventuallyAlways && invariant.Nested != nil {
		panic("Invariant checking not supported for nested invariants")
	}
	pyExpr := invariant.PyExpr
	if eventuallyAlways && invariant.Nested != nil {
		pyExpr = invariant.Nested.PyExpr
	}
	vars := CloneDict(process.Heap.globals)
	vars["__returns__"] = NewDictFromStringDict(process.Returns)
	cond, err := process.Evaluator.EvalPyExpr("filename.fizz", pyExpr, vars)
	PanicOnError(err)
	return bool(cond.Truth())
}

func CheckAssertion(process *Process, invariant *ast.Invariant) bool {
	if !slices.Contains(invariant.TemporalOperators, "always") {
		panic("Invariant checking supported only for always/always-eventually/eventually-always invariants")
	}

	vars := CloneDict(process.Heap.globals)
	vars["__returns__"] = NewDictFromStringDict(process.Returns)
	pyStmt := &ast.PyStmt{
		Code: invariant.PyCode + "\n" + "__retval__ = " + invariant.Name + "()\n",
	}
	_, err := process.Evaluator.ExecPyStmt("filename.fizz", pyStmt, vars)
	PanicOnError(err)
	return bool(vars["__retval__"].Truth())
}

func CheckStrictLiveness(node *Node) ([]*Node, *InvariantPosition) {
	fmt.Println("Checking strict liveness")
	process := node.Process
	if len(process.Files) > 1 {
		panic("Invariant checking not supported for multiple files yet")
	}
	for i, file := range process.Files {
		for j, invariant := range file.Invariants {
			predicate := func(n *Node) (bool, bool) {
				return len(n.Process.Threads) == 0, n.Process.Witness[i][j]
			}
			eventuallyAlways := false
			alwaysEventually := false
			if invariant.Block == nil {
				if invariant.Always && invariant.Eventually {
					alwaysEventually = true
				} else if invariant.Eventually && invariant.GetNested().GetAlways() {
					eventuallyAlways = true
				}
			} else {
				if slices.Contains(invariant.TemporalOperators, "eventually") &&
					invariant.TemporalOperators[0] == "eventually" && invariant.TemporalOperators[1] == "always" {
					eventuallyAlways = true
				} else if slices.Contains(invariant.TemporalOperators, "eventually") &&
					invariant.TemporalOperators[0] == "always" && invariant.TemporalOperators[1] == "eventually" {
					alwaysEventually = true
				}
			}
			if eventuallyAlways {
				fmt.Println("Checking eventually always", invariant.Name)
				failurePath, isLive := EventuallyAlwaysFinal(node, predicate)
				if !isLive {
					return failurePath, NewInvariantPosition(i,j)
				}
			} else if alwaysEventually {
				fmt.Println("Checking always eventually", invariant.Name)
				// Always Eventually
				failurePath, isLive := AlwaysEventuallyFinal(node, predicate)
				if !isLive {
					return failurePath, NewInvariantPosition(i,j)
				}
			}
		}

	}
	return nil, nil
}

func CheckFastLiveness(allNodes []*Node) ([]*Node, *InvariantPosition) {
	fmt.Println("Checking strict liveness fast approach")
	node := allNodes[0]
	process := node.Process
	if len(process.Files) > 1 {
		panic("Invariant checking not supported for multiple files yet")
	}
	for i, file := range process.Files {
		for j, invariant := range file.Invariants {
			predicate := func(n *Node) (bool, bool) {
				return len(n.Process.Threads) == 0, n.Process.Witness[i][j]
			}
			eventuallyAlways := false
			alwaysEventually := false
			if invariant.Block == nil {
				if invariant.Always && invariant.Eventually {
					alwaysEventually = true
				} else if invariant.Eventually && invariant.GetNested().GetAlways() {
					eventuallyAlways = true
				}
			} else {
				if slices.Contains(invariant.TemporalOperators, "eventually") &&
					invariant.TemporalOperators[0] == "eventually" && invariant.TemporalOperators[1] == "always" {
					eventuallyAlways = true
				} else if slices.Contains(invariant.TemporalOperators, "eventually") &&
					invariant.TemporalOperators[0] == "always" && invariant.TemporalOperators[1] == "eventually" {
					alwaysEventually = true
				}
			}
			if eventuallyAlways {
				fmt.Println("Checking eventually always", invariant.Name)
				// TODO(jp): Come up with a fast way to check eventually always
				failurePath, isLive := EventuallyAlwaysFinal(node, predicate)
				if !isLive {
					return failurePath, NewInvariantPosition(i,j)
				}
			} else if alwaysEventually {
				fmt.Println("Checking always eventually", invariant.Name)
				// Always Eventually
				failurePath, isLive := AlwaysEventuallyFast(allNodes, predicate)
				if !isLive {
					return failurePath, NewInvariantPosition(i,j)
				}
			}
		}

	}
	return nil, nil
}

func AlwaysEventuallyFast(nodes []*Node, predicate Predicate) ([]*Node, bool) {
	// For strong fairness.
	// For each good node, walk up the Strongly Fair inbound links, and mark them good as well. Eventually, you will
	// end up with nodes that cannot reach a good node either because of a cycle or because of stuttering

	falseNodes := make(map[*Node]bool)
	visited := make(map[*Node]bool)
	queue := lib.NewQueue[*Node]()
	for _, node := range nodes {
		if len(node.Outbound) == 0 {
			fmt.Println("Deadlock detected, at node: ", node.String())
			panic("Deadlock detected, at node: " + node.String())
		}
		relevant, value := predicate(node)
		if relevant && value {
			queue.Enqueue(node)
		} else {
			falseNodes[node] = true
		}
	}
	for queue.Count() > 0 {
		node, _ := queue.Dequeue()
		if visited[node] {
			continue
		}
		visited[node] = true
		for _, link := range node.Inbound {
			if visited[link.Node] || link.Node == node ||
				link.Fairness != ast.FairnessLevel_FAIRNESS_LEVEL_STRONG {
                continue
			}
			delete(falseNodes, link.Node)
			queue.Enqueue(link.Node)
		}
	}
	if len(falseNodes) > 0 {
		var closestDeadNode *Node

		for node, _ := range falseNodes {
			//fmt.Println("-\n",node.String(), count)
			if closestDeadNode == nil || (len(closestDeadNode.Threads) > 0 && len(node.Threads) == 0) {
				closestDeadNode = node
				continue
			}
			if node.actionDepth > closestDeadNode.actionDepth {
				continue
			} else if node.actionDepth < closestDeadNode.actionDepth {
				closestDeadNode = node
			} else if node.forkDepth < closestDeadNode.forkDepth {
				closestDeadNode = node
			}
		}
		//fmt.Println("Closest dead node:", closestDeadNode.String())
		failurePath := pathToInit(nodes, closestDeadNode)
		path := findCyclePath(closestDeadNode, falseNodes)
		path = append(failurePath, path...)
		return path, false
	} else {
		fmt.Println("Always eventually  invariant passed")
	}
	return nil, true
}

func pathToInit(nodes []*Node, closestDeadNode *Node) []*Node {
	failurePath := make([]*Node, 0)

	node := closestDeadNode
	for node != nil {
		failurePath = append(failurePath, node)
		if len(node.Inbound) == 0 || node.Name == "init" || node == nodes[0] {
			break
		}
		node = node.Inbound[0].Node
	}
	slices.Reverse(failurePath)
	return failurePath
}

func findCyclePath(startNode *Node, nodes map[*Node]bool) []*Node {
	type Wrapper struct {
		node *Node
		path []*Node
		visited map[*Node]bool
	}
	queue := lib.NewQueue[*Wrapper]()
	queue.Enqueue(&Wrapper{node: startNode, path: make([]*Node, 0), visited: make(map[*Node]bool)})

	for queue.Count() > 0 {
		element, _ := queue.Dequeue()
		node := element.node
		path := element.path
		visited := element.visited
		fairCount := 0
		for _, link := range node.Outbound {
			if link.Fairness != ast.FairnessLevel_FAIRNESS_LEVEL_STRONG {
				continue
			}
			if !nodes[link.Node] {
				continue
			}
			fairCount++
			pathCopy := slices.Clone(path)
			visitedCopy := maps.Clone(visited)

			pathCopy = append(pathCopy, link.Node)
			if visitedCopy[node] {
				return path
			}
			visitedCopy[node] = true
			queue.Enqueue(&Wrapper{node: link.Node, path: pathCopy, visited: visitedCopy})
		}
		if fairCount == 0 {
			pathCopy := slices.Clone(path)
			pathCopy = append(pathCopy, node)
			return pathCopy
		}
	}
	// TODO: Should this panic?
	panic("Cycle not found")
	//return nil
}

func EventuallyAlwaysFast(nodes []*Node, predicate Predicate) ([]*Node, bool) {
	panic("Not implemented")
}


type Predicate func(n *Node) (bool, bool)

type CycleCallback func(path []*Node) bool

func AlwaysEventuallyFinal(root *Node, predicate Predicate) ([]*Node, bool) {
	f := func(path []*Node) bool {
		mergeNode := path[len(path)-1]
		// iterate over the path in reverse order and check if the property holds
		for i := len(path) - 1; i >= 0; i-- {
			relevant, value := predicate(path[i])
			//fmt.Printf("Node: %s, Relevant: %t, Value: %t\n", path[i].String(), relevant, value)
			if relevant && value {
				//fmt.Println("Live node FOUND in the path")
				return true
			}
			if i < len(path) - 1 && path[i] == mergeNode {
				break
			}
		}
		//fmt.Println("Live node NOT FOUND in the path")
		return false
	}
	return CycleFinderFinalBfs(root, f)
}

func EventuallyAlwaysFinal(root *Node, predicate Predicate) ([]*Node, bool) {
	f := func(path []*Node) bool {
		mergeNode := path[len(path)-1]
		// iterate over the path in reverse order and check if the property holds
		for i := len(path) - 1; i >= 0; i-- {
			relevant, value := predicate(path[i])
			//fmt.Printf("Node: %s, Relevant: %t, Value: %t\n", path[i].String(), relevant, value)
			if relevant && !value {
				//fmt.Println("Dead node FOUND in the path")
				return false
			}
			if i < len(path) - 1 && path[i] == mergeNode {
				break
			}
		}
		//fmt.Println("Dead node NOT FOUND in the path")
		return true
	}
	return CycleFinderFinal(root, f)
}

func CycleFinderFinal(node *Node, callback CycleCallback) ([]*Node, bool) {
	visited := make(map[*Node]bool)
	globalVisited := make(map[*Node]bool)
	path := make([]*Node, 0)
	return cycleFinderHelper(node, callback, visited, path, globalVisited)
}

func cycleFinderHelper(node *Node, callback CycleCallback, visited map[*Node]bool, path []*Node, globalVisited map[*Node]bool) ([]*Node, bool) {
	if visited[node] {
		path = append(path, node)

		//fmt.Println("\n\nCycle detected in the path:")
		//fmt.Println("Path:", path)
		return path, callback(path)
	}

	visited[node] = true
	path = append(path, node)
	if globalVisited[node] {
		//fmt.Println("Skipping node", node.String())
		return nil, true
	}
	globalVisited[node] = true

	// Traverse outbound links
	for _, link := range node.Outbound {
		pathCopy := slices.Clone(path)
		visitedCopy := maps.Clone(visited)
		failedPath, success := cycleFinderHelper(link.Node, callback, visitedCopy, pathCopy, globalVisited)
		if !success {
			return failedPath,false
		}
	}
	return nil, true
}

func CycleFinderFinalBfs(node *Node, callback CycleCallback) ([]*Node, bool) {
	visited := make(map[*Node]bool)
	path := make([]*Node, 0)
	return cycleFinderHelperBfs(node, callback, visited, path)
}

func cycleFinderHelperBfs(node *Node, callback CycleCallback, visited map[*Node]bool, path []*Node) ([]*Node, bool) {
	type Wrapper struct {
		node *Node
		path []*Node
		visited map[*Node]bool
	}
	queue := lib.NewQueue[*Wrapper]()
	queue.Enqueue(&Wrapper{node: node, path: path, visited: visited})
	for queue.Count() > 0 {
		element, _ := queue.Dequeue()
		node = element.node
		path = element.path
		visited = element.visited

		if visited[node] {
			path = append(path, node)
			//fmt.Println("\n\nCycle detected in the path:")
			//fmt.Println("Path:", path)
			live := callback(path)
			if live {
				continue
			}
			return path, false
		}
		visited[node] = true
		path = append(path, node)

		// Traverse outbound links
		for _, link := range node.Outbound {
			pathCopy := slices.Clone(path)
			visitedCopy := maps.Clone(visited)
			queue.Enqueue(&Wrapper{node: link.Node, path: pathCopy, visited: visitedCopy})

		}
	}
	return nil, true
}

func NewDictFromStringDict(vals starlark.StringDict) *starlark.Dict {
	result := starlark.NewDict(len(vals))
	for k, v := range vals {
		err := result.SetKey(starlark.String(k), v)
		// Should not fail
		PanicOnError(err)
	}
	return result
}
