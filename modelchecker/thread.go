package modelchecker

import (
	"crypto/sha256"
	"encoding/json"
	"fizz/ast"
	"fmt"
	"github.com/golang/protobuf/proto"
	"github.com/jayaprabhakar/fizzbee/lib"
	"go.starlark.net/starlark"
	"hash"
	"sort"
	"strings"
)

type Heap struct {
	globals starlark.StringDict
}

func StringDictToMap(stringDict starlark.StringDict) map[string]string {
	m := make(map[string]string)
	for k, v := range stringDict {
		if v.Type() == "set" {
			// Convert set to a list.
			iter := v.(starlark.Iterable).Iterate()
			defer iter.Done()
			var x starlark.Value
			var list []string
			for iter.Next(&x) {
				list = append(list, x.String())
			}
			sort.Strings(list)
			m[k] = fmt.Sprintf("%v", list)
			continue
		}
		m[k] = v.String()
	}
	return m
}

func (h *Heap) ToJson() string {
	bytes, err := StringDictToJson(h.globals)
	if err != nil {
		panic(err)
	}
	return string(bytes)
}

func StringDictToJson(stringDict starlark.StringDict) ([]byte, error) {
	m := StringDictToMap(stringDict)
	bytes, err := json.Marshal(m)
	return bytes, err
}

func (h *Heap) String() string {
	return h.ToJson()
}

// HashCode returns a string hash of the global state.
func (h *Heap) HashCode() string {
	hash := sha256.New()
	hash.Write([]byte(h.ToJson()))
	return fmt.Sprintf("%x", hash.Sum(nil))
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
	flow   ast.Flow
	// vars is the set of variables defined in this scope.
	vars starlark.StringDict
	// On parallel execution, skipstmts contains the list of statements to skip
	// as it is already executed.
	skipstmts []int
}

func (s *Scope) Hash() hash.Hash {
	var h hash.Hash
	if s == nil {
		return sha256.New()
	}
	if s.parent != nil {
		h = s.parent.Hash()
	} else {
		h = sha256.New()
	}
	vars, err := StringDictToJson(s.vars)
	if err != nil {
		panic(err)
	}
	h.Write(vars)
	h.Write([]byte(fmt.Sprintln(sortedCopy(s.skipstmts))))
	return h
}

func (s *Scope) HashCode() string {
	return fmt.Sprintf("%x", s.Hash().Sum(nil))
}

func sortedCopy(slice []int) []int {
	sorted := make([]int, len(slice))
	copy(sorted, slice)
	sort.Ints(sorted)
	return sorted
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
		if v.Type() == "set" {
			// Clone a set.
			//to[k] = v.(*starlark.Set).Clone()
			iter := v.(starlark.Iterable).Iterate()
			defer iter.Done()
			var x starlark.Value
			newSet := starlark.NewSet(10)
			for iter.Next(&x) {
				err := newSet.Insert(x)
				PanicOnError(err)
			}
			to[k] = newSet
			continue
		}
		to[k] = v
	}
	return to
}

type CallFrame struct {
	// FileIndex is the ast.FileIndex that this frame is executing.
	FileIndex int
	// pc is the program counter, pointing at the next instruction to execute.
	pc string
	// scope is the lexical scope of the current frame
	scope *Scope
}

func (c *CallFrame) HashCode() string {
	// Hash the scope and append the pc to it.
	// This is to ensure that the same scoped variables are not treated the same
	// if program counter is at different stmts.
	h := c.scope.Hash()
	h.Write([]byte(c.pc))
	return fmt.Sprintf("%x", h.Sum(nil))
}

type CallStack struct {
	*lib.Stack[*CallFrame]
}

func NewCallStack() *CallStack {
	return &CallStack{lib.NewStack[*CallFrame]()}
}

func (s *CallStack) Clone() *CallStack {
	return &CallStack{s.Stack.Clone()}
}

func (s *CallStack) HashCode() string {
	if s == nil {
		return ""
	}
	arr := s.RawArrayCopy()
	h := sha256.New()

	for _, frame := range arr {
		h.Write([]byte(frame.HashCode()))
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

// Thread represents a thread of execution.
type Thread struct {
	Process *Process
	Files   []*ast.File
	Stack   *CallStack
}

func NewThread(Process *Process, files []*ast.File, fileIndex int, action string) *Thread {
	stack := NewCallStack()
	frame := &CallFrame{FileIndex: fileIndex, pc: action}
	t := &Thread{Process: Process, Files: files, Stack: stack}
	t.pushFrame(frame)
	return t
}

func (t *Thread) HashCode() string {
	h := sha256.New()
	h.Write([]byte(t.Stack.HashCode()))
	return fmt.Sprintf("%x", h.Sum(nil))
}

// InsertNewScope adds a new scope to the current stack frame and returns the newly created scope.
func (t *Thread) InsertNewScope() *Scope {
	scope := &Scope{parent: t.currentFrame().scope, vars: starlark.StringDict{}}
	t.currentFrame().scope = scope
	return scope
}

func (t *Thread) currentFrame() *CallFrame {
	frame, ok := t.Stack.Peek()
	PanicIfFalse(ok, "No frame on the stack")
	return frame
}

func (t *Thread) currentFileAst() *ast.File {
	frame := t.currentFrame()
	return t.Files[frame.FileIndex]
}

func PanicIfFalse(ok bool, msg string) {
	if !ok {
		panic(msg)
	}
}

func PanicOnError(err error) {
	if err != nil {
		panic(err)
	}
}

func (t *Thread) pushFrame(frame *CallFrame) {
	t.Stack.Push(frame)
}

func (t *Thread) popFrame() *CallFrame {
	frame, found := t.Stack.Pop()
	PanicIfFalse(found, "No frame on the stack")
	return frame
}

func (t *Thread) Clone() *Thread {
	return &Thread{Process: t.Process, Files: t.Files, Stack: t.Stack.Clone()}
}

func (t *Thread) Execute() ([]*Process, bool) {
	//fmt.Println(t.Process.Heap.globals)
	var forks []*Process
	yield := false
	for t.Stack.Len() > 0 {
		//fmt.Println(t.Process.Heap.globals)
		//fmt.Println(t.currentPc())
		for t.currentFrame().pc == "" || strings.HasSuffix(t.currentFrame().pc, ".Block.$") {
			yield = t.executeEndOfBlock()
			if yield {
				return forks, yield
			}
		}
		frame := t.currentFrame()
		protobuf := GetProtoFieldByPath(t.currentFileAst(), frame.pc)

		switch protobuf.(type) {
		case *ast.Action:
			t.executeAction()
		case *ast.Block:
			forks = t.executeBlock()
		case *ast.Statement:
			forks, yield = t.executeStatement()
		}
		if len(forks) > 0 || yield {
			break
		}
	}
	return forks, yield
}

func (t *Thread) executeAction() {
	t.currentFrame().pc = t.currentFrame().pc + ".Block"
}

func (t *Thread) executeBlock() []*Process {
	newScope := t.InsertNewScope()
	protobuf := GetProtoFieldByPath(t.currentFileAst(), t.currentPc())
	b := convertToBlock(protobuf)
	newScope.flow = b.Flow
	switch b.Flow {
	case ast.Flow_FLOW_ATOMIC:
		t.currentFrame().pc = t.currentPc() + ".Stmts[0]"
		return nil
	case ast.Flow_FLOW_SERIAL:
		t.currentFrame().pc = t.currentPc() + ".Stmts[0]"
		return nil
	case ast.Flow_FLOW_ONEOF:
		forks := make([]*Process, len(b.Stmts))
		for i, _ := range b.Stmts {
			forks[i] = t.Process.Fork()
			forks[i].currentThread().currentFrame().pc = fmt.Sprintf("%s.Stmts[%d]", t.currentPc(), i)
		}
		//t.currentFrame().pc = ""
		return forks
	case ast.Flow_FLOW_PARALLEL:
		forks := make([]*Process, len(b.Stmts))
		for i, _ := range b.Stmts {
			forks[i] = t.Process.Fork()
			forks[i].currentThread().currentFrame().pc = fmt.Sprintf("%s.Stmts[%d]", t.currentPc(), i)
			forks[i].currentThread().currentFrame().scope.skipstmts = append(forks[i].currentThread().currentFrame().scope.skipstmts, i)
		}
		//t.currentFrame().pc = ""
		return forks
	default:
		panic("Unknown flow type")
	}

	return nil
}

func (t *Thread) executeStatement() ([]*Process, bool) {
	protobuf := GetProtoFieldByPath(t.currentFileAst(), t.currentPc())
	stmt := convertToStatement(protobuf)
	if stmt.PyStmt != nil {
		vars := t.Process.GetAllVariables()
		_, err := t.Process.Evaluator.ExecPyStmt("filename.fizz", stmt.PyStmt, vars)
		PanicOnError(err)
		t.Process.updateAllVariablesInScope(vars)
	} else if stmt.Block != nil {
		t.currentFrame().pc = t.currentFrame().pc + ".Block"
		forks := t.executeBlock()
		return forks, false
	} else if stmt.IfStmt != nil {
		if stmt.IfStmt.Flow != ast.Flow_FLOW_ATOMIC {
			panic("Only atomic flow is supported for if statements")
		}
		for i, branch := range stmt.IfStmt.Branches {
			vars := t.Process.GetAllVariables()
			cond, err := t.Process.Evaluator.EvalPyExpr("filename.fizz", branch.Condition, vars)
			PanicOnError(err)
			t.Process.updateAllVariablesInScope(vars)
			if cond.Truth() {
				t.currentFrame().pc = fmt.Sprintf("%s.IfStmt.Branches[%d].Block", t.currentPc(), i)
				return nil, false
			}
		}

		//t.currentFrame().pc = t.currentFrame().pc + ".Block"
		//forks := t.executeBlock()
		//return forks, false
	} else if stmt.AnyStmt != nil {
		if stmt.AnyStmt.Flow != ast.Flow_FLOW_ATOMIC {
			panic("Only atomic flow is supported for any statements")
		}
		if len(stmt.AnyStmt.LoopVars) != 1 {
			panic("Loop variables must be exactly one")
		}
		vars := t.Process.GetAllVariables()
		val, err := t.Process.Evaluator.EvalPyExpr("filename.fizz", stmt.AnyStmt.PyExpr, vars)
		PanicOnError(err)
		rangeVal, _ := val.(starlark.Iterable)
		iter := rangeVal.Iterate()
		defer iter.Done()

		//scope := t.InsertNewScope()
		//scope.flow = stmt.AnyStmt.Flow
		forks := make([]*Process, 0)
		var x starlark.Value
		for iter.Next(&x) {
			//fmt.Printf("anyVariable: x: %s\n", x.String())
			fork := t.Process.Fork()
			fork.currentThread().currentFrame().pc = fmt.Sprintf("%s.AnyStmt.Block", t.currentPc())
			fork.currentThread().currentFrame().scope.vars[stmt.AnyStmt.LoopVars[0]] = x
			forks = append(forks, fork)

		}
		if len(forks) > 0 {
			return forks, false
		}

		//scope.vars[stmt.AnyStmt.LoopVars[0]] = val
		//t.currentFrame().pc = fmt.Sprintf("%s.AnyStmt.Block", t.currentPc())
	} else {
		panic(fmt.Sprintf("Unknown statement type: %v", stmt))
	}
	return t.executeEndOfStatement()
}

func (t *Thread) executeEndOfStatement() ([]*Process, bool) {
	switch t.currentFrame().scope.flow {
	case ast.Flow_FLOW_ATOMIC:
		t.currentFrame().pc = t.FindNextProgramCounter()
		return nil, false
	case ast.Flow_FLOW_SERIAL:
		t.currentFrame().pc = t.FindNextProgramCounter()
		return nil, true
	case ast.Flow_FLOW_ONEOF:
		t.currentFrame().pc = EndOfBlock(t.currentPc())
		return nil, false
	case ast.Flow_FLOW_PARALLEL:
		blockPath := ParentBlockPath(t.currentPc())
		if blockPath == "" {
			//return nil, t.executeEndOfBlock()
		}
		protobuf := GetProtoFieldByPath(t.currentFileAst(), blockPath)
		b := convertToBlock(protobuf)
		skipstmts := t.currentFrame().scope.skipstmts
		if len(skipstmts) == len(b.Stmts) {
			t.currentFrame().pc = EndOfBlock(t.currentPc())
			return nil, true
		}
		forks := make([]*Process, 0, len(b.Stmts)-len(skipstmts))
		for i, _ := range b.Stmts {
			if ContainsInt(skipstmts, i) {
				continue
			}
			fork := t.Process.Fork()
			fork.currentThread().currentFrame().pc = fmt.Sprintf("%s.Stmts[%d]", blockPath, i)
			fork.currentThread().currentFrame().scope.skipstmts = append(fork.currentThread().currentFrame().scope.skipstmts, i)
			forks = append(forks, fork)
		}
		t.currentFrame().pc = ""
		return forks, true
	default:
		panic(fmt.Sprintf("Unknown flow type at %s", t.currentPc()))
	}
}

func (t *Thread) executeEndOfBlock() bool {
	frame := t.currentFrame()
	if frame == nil {
		return false
	}
	for {
		frame.scope = frame.scope.parent
		if frame.scope == nil {
			t.popFrame()

			if t.Stack.Len() == 0 {
				t.Process.removeCurrentThread()
				return true
			}
		}
		t.currentFrame().pc = RemoveLastBlock(t.currentPc())
		forks, yield := t.executeEndOfStatement()
		if len(forks) > 0 || yield {
			return yield
		}

		if t.currentPc() != "" {
			break
		}
	}
	if frame.scope.flow == ast.Flow_FLOW_SERIAL ||
		frame.scope.flow == ast.Flow_FLOW_PARALLEL {
		return true
	}
	return false
}

func ContainsInt(skipstmts []int, i int) bool {
	for _, s := range skipstmts {
		if s == i {
			return true
		}
	}
	return false
}

func (t *Thread) currentPc() string {
	return t.currentFrame().pc
}

func (t *Thread) FindNextProgramCounter() string {
	frame := t.currentFrame()
	protobuf := GetProtoFieldByPath(t.currentFileAst(), frame.pc)
	switch protobuf.(type) {
	case *ast.Action:
		return frame.pc + ".Block"
	case *ast.Block:
		convertToBlock(protobuf)
		return frame.pc + ".Stmts[0]"
	case *ast.Statement:
		path, _ := GetNextFieldPath(t.currentFileAst(), frame.pc)
		return path
	case *ast.AnyStmt:
		path, _ := GetNextFieldPath(t.currentFileAst(), frame.pc)
		return path
	case *ast.Branch:
		path, _ := GetNextFieldPath(t.currentFileAst(), frame.pc)
		return path
	}
	return ""
}

func convertToBlock(message proto.Message) *ast.Block {
	return message.(*ast.Block)
}

func convertToStatement(message proto.Message) *ast.Statement {
	return message.(*ast.Statement)
}
