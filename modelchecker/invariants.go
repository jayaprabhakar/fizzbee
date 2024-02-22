package modelchecker

import (
	ast "fizz/proto"
	"go.starlark.net/starlark"
	"slices"
)

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

func NewDictFromStringDict(vals starlark.StringDict) *starlark.Dict {
	result := starlark.NewDict(len(vals))
	for k, v := range vals {
		err := result.SetKey(starlark.String(k), v)
		// Should not fail
		PanicOnError(err)
	}
	return result
}
