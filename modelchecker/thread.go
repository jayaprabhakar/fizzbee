package modelchecker

import (
	"github.com/jayaprabhakar/fizzbee/lib"
	"go.starlark.net/starlark"
)

type Heap struct {
	globals starlark.StringDict
}

func (h *Heap) update(k string, v starlark.Value) bool {
	if _, ok := h.globals[k]; ok {
		h.globals[k] = v
		return true
	}
	return false
}

func (h *Heap) Clone() *Heap {
	return &Heap{CloneDict(h.globals)}
}

type Scope struct {
	// parent is the parent scope, or nil if this is the global scope.
	parent *Scope
	// vars is the set of variables defined in this scope.
	vars starlark.StringDict
}

func (s *Scope) Lookup(name string) (starlark.Value, bool) {
	v, ok := s.vars[name]
	if !ok && s.parent != nil {
		return s.parent.Lookup(name)
	}
	return v, ok
}

// GetAllVisibleVariables returns all variables visible in this scope.
func (s *Scope) GetAllVisibleVariables() starlark.StringDict {
	dict := starlark.StringDict{}
	s.getAllVisibleVariablesToDict(dict)
	return dict
}

func (s *Scope) getAllVisibleVariablesToDict(dict starlark.StringDict) {
	if s.parent != nil {
		s.parent.getAllVisibleVariablesToDict(dict)
	}
	CopyDict(s.vars, dict)
}

func CloneDict(oldDict starlark.StringDict) starlark.StringDict {
	return CopyDict(oldDict, nil)
}

// CopyDict copies values `from` to `to` overriding existing values. If the `to` is nil, creates a new dict.
func CopyDict(from starlark.StringDict, to starlark.StringDict) starlark.StringDict {
	if to == nil {
		to = make(starlark.StringDict)
	}
	for k, v := range from {
		to[k] = v
	}
	return to
}

type CallFrame struct {
	// pc is the program counter, pointing at the next instruction to execute.
	pc int
	// scope is the lexical scope of the current frame
	scope *Scope
}

// Thread represents a thread of execution.
type Thread struct {
	Stack *lib.Stack[*CallFrame]
}

func NewThread() *Thread {
	return &Thread{lib.NewStack[*CallFrame]()}
}

func (t *Thread) currentFrame() *CallFrame {
	frame, err := t.Stack.Peek()
	panic(err)
	return frame
}

func (t *Thread) pushFrame(frame *CallFrame) {
	t.Stack.Push(frame)
}
