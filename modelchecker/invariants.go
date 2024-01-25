package modelchecker

import (
	"fizz/ast"
	"go.starlark.net/starlark"
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

			if invariant.Eventually && passed && len(process.Threads) == 0 {
				process.Witness[i][j] = true
			} else if !invariant.Eventually && !passed {
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
	vars := CloneDict(process.Heap.globals)
	vars["__returns__"] = NewDictFromStringDict(process.Returns)
	cond, err := process.Evaluator.EvalPyExpr("filename.fizz", invariant.PyExpr, vars)
	PanicOnError(err)
	return bool(cond.Truth())
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
