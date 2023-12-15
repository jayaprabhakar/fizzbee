// Package modelchecker implements the model checker for the FizzBuzz program.
// It is based on the Starlark interpreter for the python part of the code.
// For the interpreter to implement the model checker, we need to simulate
// parallel universe.
// Every time, there is a non-deterministic choice, we need to fork the universe
// and continue the execution in both the universes with the different choices.
// Each universe is represented by a process.
// Each process has a heap and multiple threads.
// Each thread has a stack of call frames.
// Each call frame has a program counter and scope (with nesting).
// The heap is shared across all the threads in the process.
// Duplicate detection: Two threads are same if they have the same stack of call frames
// Two processes are same if they have the same heap and same threads.
package modelchecker

import (
	"github.com/huandu/go-clone"
	"go.starlark.net/starlark"
)

type Process struct {
	Heap    *Heap
	Threads []*Thread
	current int
	Name    string
	Parent  *Process
}

func NewProcess(name string, parent *Process) *Process {
	thread := NewThread()
	return &Process{
		Name:    name,
		Heap:    &Heap{starlark.StringDict{}},
		Threads: []*Thread{thread},
		current: 0,
		Parent:  parent,
	}
}

func (p *Process) Fork() *Process {
	clonedThreads := clone.Clone(p.Threads).([]*Thread)
	return &Process{
		Name:    p.Name,
		Heap:    p.Heap.Clone(),
		Threads: clonedThreads,
		current: p.current,
		Parent:  p,
	}
}

func (p *Process) currentThread() *Thread {
	return p.Threads[p.current]
}

// GetAllVariables returns all variables visible in the current thread.
// This includes state variables and variables from the current thread's variables in the top call frame
func (p *Process) GetAllVariables() starlark.StringDict {
	dict := CloneDict(p.Heap.globals)
	frame := p.currentThread().currentFrame()
	frame.scope.getAllVisibleVariablesToDict(dict)
	return dict
}

func (p *Process) updateAllVariablesInScope(dict starlark.StringDict) {
	frame := p.currentThread().currentFrame()
	for k, v := range dict {
		if p.updateScopedVariable(frame.scope, k, v) {
			// Check local variables in the scope, starting from
			// deepest to its parent. If present, update that
			// and continue
			continue
		}
		if p.Heap.update(k, v) {
			// if no scoped variable exists, check if it is state
			// variable, then update the state variable
			continue
		}
		// Declare the variable to the current scope
		frame.scope.vars[k] = v
	}
}

func (p *Process) updateScopedVariable(scope *Scope, key string, val starlark.Value) bool {
	if scope == nil {
		return false
	}
	if _, ok := scope.vars[key]; ok {
		scope.vars[key] = val
		return true
	}
	return p.updateScopedVariable(scope.parent, key, val)
}
