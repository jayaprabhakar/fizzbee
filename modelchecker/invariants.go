package modelchecker

import (
	ast "fizz/proto"
	"fmt"
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
				return len(process.Threads) == 0, n.Process.Witness[i][j]
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
	return CycleFinderFinal(root, f)
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
	path := make([]*Node, 0)
	return cycleFinderHelper(node, callback, visited, path)
}

func cycleFinderHelper(node *Node, callback CycleCallback, visited map[*Node]bool, path []*Node) ([]*Node, bool) {
	if visited[node] {
		path = append(path, node)

		//fmt.Println("\n\nCycle detected in the path:")
		//fmt.Println("Path:", path)
		return path, callback(path)
	}

	visited[node] = true
	path = append(path, node)

	// Traverse outbound links
	for _, link := range node.Outbound {
		pathCopy := slices.Clone(path)
		visitedCopy := maps.Clone(visited)
		failedPath, success := cycleFinderHelper(link.Node, callback, visitedCopy, pathCopy)
		if !success {
			return failedPath,false
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
