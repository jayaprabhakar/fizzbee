package modelchecker

import (
	"fizz/ast"
)

func CheckInvariants(process *Process) map[int][]int {
	if len(process.Files) > 1 {
		panic("Invariant checking not supported for multiple files")
	}
	results := make(map[int][]int)
	for i, file := range process.Files {
		results[i] = make([]int, 0)
		for j, invariant := range file.Invariants {
			passed := CheckInvariant(process, invariant)
			if !passed {
				results[i] = append(results[i], j)
			}
		}
	}
	return results
}

func CheckInvariant(process *Process, invariant *ast.Invariant) bool {
	if !invariant.Always {
		panic("Invariant checking not supported for non-always invariants")
	}
	if invariant.Nested != nil {
		panic("Invariant checking not supported for nested invariants")
	}
	vars := process.Heap.globals
	cond, err := process.Evaluator.EvalPyExpr("filename.fizz", invariant.PyExpr, vars)
	PanicOnError(err)
	return bool(cond.Truth())

}
