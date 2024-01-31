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
	"crypto/sha256"
	ast "fizz/proto"
	"fmt"
	"github.com/zeroflucs-given/generics/collections"
	_ "github.com/zeroflucs-given/generics/collections"
	"github.com/zeroflucs-given/generics/collections/linkedlist"
	_ "github.com/zeroflucs-given/generics/collections/linkedlist"
	"go.starlark.net/starlark"
	"runtime"
	"sort"
	"strings"
	"time"
)

// DefType is a custom enum-like type
type DefType string

const (
	Function DefType = "function"
)

type Definition struct {
	DefType   DefType
	name      string
	fileIndex int
	path      string
}

type Process struct {
	Heap             *Heap
	Threads          []*Thread
	current          int
	Name             string
	Files            []*ast.File
	Parent           *Process
	Evaluator        *Evaluator
	Children         []*Process
	FailedInvariants map[int][]int
	// Witness indicates the successful liveness checks
	// For liveness checks, not all nodes will pass the condition, witness indicates
	// which invariants this node passed.
	Witness     [][]bool
	Returns     starlark.StringDict
	SymbolTable map[string]*Definition
	Labels 		[]string
}

func NewProcess(name string, files []*ast.File, parent *Process) *Process {
	var mc *Evaluator
	var symbolTable map[string]*Definition

	if parent == nil {
		mc = NewModelChecker("example")
		symbolTable = make(map[string]*Definition)

		for i, file := range files {
			for j, function := range file.Functions {
				symbolTable[function.Name] = &Definition{
					DefType:   Function,
					name:      function.Name,
					fileIndex: i,
					path:      fmt.Sprintf("Functions[%d]", j),
				}
			}
		}
	} else {
		mc = parent.Evaluator
		symbolTable = parent.SymbolTable
	}
	p := &Process{
		Name:        name,
		Heap:        &Heap{starlark.StringDict{}},
		Threads:     []*Thread{},
		current:     0,
		Files:       files,
		Parent:      parent,
		Evaluator:   mc,
		Children:    []*Process{},
		Returns:     make(starlark.StringDict),
		SymbolTable: symbolTable,
		Labels: 	 make([]string, 0),
	}
	p.Witness = make([][]bool, len(files))
	for i, file := range files {
		p.Witness[i] = make([]bool, len(file.Invariants))
	}
	p.Children = append(p.Children, p)

	return p
}

func (p *Process) HasFailedInvariants() bool {
	if p == nil || p.FailedInvariants == nil {
		return false
	}
	for _, invIndex := range p.FailedInvariants {
		if len(invIndex) > 0 {
			return true
		}
	}
	return false
}

func (p *Process) Fork() *Process {
	p2 := &Process{
		Name:        p.Name,
		Heap:        p.Heap.Clone(),
		current:     p.current,
		Parent:      p,
		Evaluator:   p.Evaluator,
		Children:    []*Process{},
		Files:       p.Files,
		Returns:     make(starlark.StringDict),
		SymbolTable: p.SymbolTable,
		Labels: 	 make([]string, 0),
	}
	p2.Witness = make([][]bool, len(p.Files))
	for i, file := range p.Files {
		p2.Witness[i] = make([]bool, len(file.Invariants))
	}

	p.Children = append(p.Children, p2)
	clonedThreads := make([]*Thread, len(p.Threads))
	for i, thread := range p.Threads {
		clonedThreads[i] = thread.Clone()
		clonedThreads[i].Process = p2
	}
	p2.Threads = clonedThreads
	return p2
}

func (p *Process) NewThread() *Thread {
	thread := NewThread(p, p.Files, 0, "")
	p.Threads = append(p.Threads, thread)
	return thread
}

// String method for Process
func (n *Node) String() string {
	p := n.Process
	if p == nil {
		return "DUPLICATE"
	}
	buf := &strings.Builder{}
	buf.WriteString(fmt.Sprintf("%s\n", p.Name))
	buf.WriteString(fmt.Sprintf("Actions: %d, Forks: %d\n", n.actionDepth, n.forkDepth))

	n.appendState(p, buf)
	buf.WriteString("\n")
	if len(p.Threads) > 0 {
		buf.WriteString(fmt.Sprintf("Threads: %d/%d\n", p.current, len(p.Threads)))
	} else {
		buf.WriteString("Threads: 0\n")
	}

	return buf.String()
}

func (n *Node) GetStateString() string {
	buf := &strings.Builder{}
	n.appendState(n.Process, buf)
	return buf.String()
}
func (n *Node) appendState(p *Process, buf *strings.Builder) {
	if len(p.Heap.globals) > 0 {
		jsonString := p.Heap.String()
		// Escape double quotes
		escapedString := strings.ReplaceAll(jsonString, "\"", "\\\"")
		buf.WriteString("State: ")
		buf.WriteString(escapedString)
	}
	if len(p.Returns) > 0 {
		jsonString := StringDictToJsonString(p.Returns)
		// Escape double quotes
		escapedString := strings.ReplaceAll(jsonString, "\"", "\\\"")
		buf.WriteString("Returns: ")
		buf.WriteString(escapedString)
	}
}

// GetName returns the name
func (n *Node) GetName() string {
	p := n.Process
	if p == nil {
		return ""
	}
	return p.Name
}

func (p *Process) HashCode() string {
	threadHashes := make([]string, len(p.Threads))
	for i, thread := range p.Threads {
		threadHashes[i] = thread.HashCode()
	}

	h := sha256.New()

	// Use the current thread's hash first, not the index
	currentThreadHash := ""
	if len(threadHashes) > 0 {
		currentThreadHash = threadHashes[p.current]
	}
	h.Write([]byte(currentThreadHash))

	// Sort the thread hashes to make the hash deterministic
	sort.Strings(threadHashes)
	for _, hash := range threadHashes {
		h.Write([]byte(hash))
	}

	h.Write([]byte(StringDictToJsonString(p.Returns)))

	// hash the heap variables as well
	heapHash := p.Heap.HashCode()
	h.Write([]byte(heapHash))
	return fmt.Sprintf("%x", h.Sum(nil))
}

func (p *Process) currentThread() *Thread {
	return p.Threads[p.current]
}

func (p *Process) removeCurrentThread() {
	if len(p.Threads) == 0 {
		return
	}
	p.Threads = append(p.Threads[:p.current],
		p.Threads[p.current+1:]...)
	p.current = 0
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

func (p *Process) NewModelError(msg string, nestedError error) *ModelError {
	return NewModelError(msg, p, nestedError)
}

func (p *Process) PanicOnError(msg string, nestedError error)  {
	if nestedError != nil {
		panic(p.NewModelError(msg, nestedError))
	}
}

type Node struct {
	*Process

	Inbound  []*Link
	Outbound []*Link

	// The number of actions started until this node
	// Note: This is the shorted path to this node from the root as we do BFS.
	actionDepth int

	// The number of forks until this node from the root. This will be >= actionDepth
	// If every action is atomic, then this will be equal to actionDepth
	// Every non-determinism includes a fork, so this will be greater than actionDepth
	// Note: This is the shorted path to this node from the root as we do BFS.
	forkDepth  int
	stacktrace string
}

type Link struct {
	Node *Node
	Name string
	Labels []string
}

func NewNode(process *Process) *Node {
	return &Node{
		Process:     process,
		Inbound:     make([]*Link, 0),
		Outbound:    make([]*Link, 0),
		actionDepth: 0,
		forkDepth:   0,
		stacktrace:  captureStackTrace(),
	}
}

func (n *Node) Merge(other *Node) *Node {
	mergeNode := &Node{
		Inbound:     []*Link{&Link{Node: n}},
		Outbound:    []*Link{&Link{Node: other}},
		actionDepth: n.actionDepth,
		forkDepth:   n.forkDepth,
	}
	n.Outbound = append(n.Outbound, &Link{Node: mergeNode})
	other.Inbound = append(other.Inbound, &Link{Node: mergeNode})
	return mergeNode
}

func (n *Node) ForkForAction(process *Process, action *ast.Action) *Node {
	if process == nil {
		process = n.Process
	}
	forkNode := &Node{
		Process:     process.Fork(),
		Inbound:     []*Link{&Link{Node: n, Name: action.Name}},
		Outbound:    []*Link{},
		actionDepth: n.actionDepth + 1,
		forkDepth:   n.forkDepth + 1,
		stacktrace:  captureStackTrace(),
	}
	forkNode.Process.Name = action.Name

	n.Outbound = append(n.Outbound, &Link{Node: forkNode, Name: action.Name})
	return forkNode
}

func (n *Node) ForkForAlternatePaths(process *Process, name string) *Node {

	forkNode := &Node{
		Process:     process,
		Inbound:     []*Link{&Link{Node: n, Name: name}},
		Outbound:    []*Link{},
		actionDepth: n.actionDepth,
		forkDepth:   n.forkDepth + 1,
		stacktrace:  captureStackTrace(),
	}

	n.Outbound = append(n.Outbound, &Link{Node: forkNode, Name: name})
	return forkNode
}

type Options struct {
	// If true, continue processing even if an invariant fails
	IgnoreInvariantFailures bool
	// If true, continue processing other paths, but stop processing the current path
	// If false (default), usually returns the shortest path to the invariant failure
	ContinueOnInvariantFailure bool
	// The maximum number of nodes to process
	MaxNodes int
	// The maximum number of actions to process
	MaxActions int

	MaxConcurrentActions int
}

type Processor struct {
	Init    *Node
	Files   []*ast.File
	queue   collections.Queue[*Node]
	visited map[string]*Node
	config  *Options
}

func NewProcessor(files []*ast.File, config *Options) *Processor {
	return &Processor{
		Files:   files,
		queue:   linkedlist.New[*Node](),
		visited: make(map[string]*Node),
		config:  config,
	}
}

func (p *Processor) Start() (init *Node, failedNode *Node, err error) {
	// recover from panic
	defer func() {
		if r := recover(); r != nil {
			if modelErr, ok := r.(*ModelError); ok {
				err = modelErr
				return
			}
			panic(err)
		}
	}()
	if p.Init != nil {
		panic("processor already started")
	}
	startTime := time.Now()
	process := NewProcess("init", p.Files, nil)
	init = NewNode(process)
	globals, err := process.Evaluator.ExecInit(p.Files[0].States)
	if err != nil {
		panic(err)
	}
	process.Heap.globals = globals
	p.Init = init

	failed := CheckInvariants(process)
	if len(failed[0]) > 0 {
		init.Process.FailedInvariants = failed
		if !p.config.IgnoreInvariantFailures {
			return p.Init, nil, nil
		}
	}
	process.NewThread()

	_ = p.queue.Push(p.Init)
	prevCount := 0
	for p.queue.Count() != 0 {
		found, node := p.queue.Pop()
		if !found {
			panic("queue should not be empty")
		}
		//process := node.Process
		//if other, ok := p.visited[process.HashCode()]; ok {
		//	node.Merge(other)
		//	continue
		//}

		if node.actionDepth > p.config.MaxActions {
			// Add a node to indicate why this node was not processed
			continue
		}
		if len(p.visited)%10000 == 0 && len(p.visited) != prevCount {
			fmt.Printf("Nodes: %d, elapsed: %s\n", len(p.visited), time.Since(startTime))
			prevCount = len(p.visited)
		}

		invariantFailure := p.processNode(node)
		p.visited[node.HashCode()] = node
		if invariantFailure && failedNode == nil {
			failedNode = node
		}
		if invariantFailure && !p.config.ContinueOnInvariantFailure {
			break
		}
	}
	fmt.Printf("Nodes: %d, elapsed: %s\n", len(p.visited), time.Since(startTime))
	return p.Init, failedNode, err
}

func (p *Processor) processNode(node *Node) bool {
	if node.Process.currentThread().currentPc() == "" && node.Name == "init" {
		node.Process.removeCurrentThread()
		// This is init node, generate a fork for each action in the file
		for i, action := range p.Files[0].Actions {
			newNode := node.ForkForAction(nil, action)
			//newNode.Process.removeCurrentThread()
			thread := newNode.Process.NewThread()
			//thread := newNode.currentThread()
			thread.currentFrame().pc = fmt.Sprintf("Actions[%d]", i)
			thread.currentFrame().Name = action.Name
			_ = p.queue.Push(newNode)
		}
		return false
	}
	forks, yield := node.currentThread().Execute()
	node.Inbound[0].Labels = append(node.Inbound[0].Labels, node.Process.Labels...)
	for _, link := range node.Inbound[0].Node.Outbound {
		if link.Node == node {
			link.Labels = append(link.Labels, node.Process.Labels...)
		}
	}

	var failedInvariants map[int][]int
	if yield {
		failedInvariants = CheckInvariants(node.Process)
	}
	if len(failedInvariants[0]) > 0 {
		//panic(fmt.Sprintf("Invariant failed: %v", failedInvariants))
		node.Process.FailedInvariants = failedInvariants
		if !p.config.IgnoreInvariantFailures {
			return true
		}
	}
	//fmt.Printf("Forks: %d, Yield: %t, Threads: %d\n", len(forks), yield, len(node.Threads))
	if other, ok := p.visited[node.HashCode()]; ok {
		// Check if visited before scheduling children
		node.Merge(other)
		return false
	}
	if !yield {
		for _, fork := range forks {
			newNode := node.ForkForAlternatePaths(fork, "")
			_ = p.queue.Push(newNode)
		}
		return false
	}

	if yield {
		if len(forks) > 0 {
			//fmt.Println("yield and fork at the same time")
			for _, fork := range forks {
				p.YieldFork(node, fork)
			}
		} else {
			p.YieldNode(node)
			node.Name = "yield"
		}
		if len(node.Process.Threads) == 0 {
			return false
		}
		crashFork := node.Process.Fork()
		crashFork.Name = "crash"
		crashFork.removeCurrentThread()
		crashNode := node.ForkForAlternatePaths(crashFork, "crash")
		// TODO: We could just copy the failed invariants from the parent
		// instead of checking again
		CheckInvariants(crashFork)

		p.YieldNode(crashNode)
		return false
	}
	return false
}

func (p *Processor) YieldNode(node *Node) {
	//node.Name = "yield"
	if other, ok := p.visited[node.HashCode()]; ok {
		// Check if visited before scheduling children
		node.Merge(other)
		return
	}

	for i, thread := range node.Threads {
		if thread.currentPc() == "" {
			continue
		}
		name := fmt.Sprintf("thread-%d", i)
		newNode := node.ForkForAlternatePaths(thread.Process.Fork(), name)
		newNode.current = i

		_ = p.queue.Push(newNode)
	}

	if node.actionDepth >= p.config.MaxActions || len(node.Threads) >= p.config.MaxConcurrentActions {
		return
	}
	for i, action := range p.Files[0].Actions {
		newNode := node.ForkForAction(nil, action)
		newNode.Process.NewThread()
		newNode.Process.current = len(newNode.Process.Threads) - 1
		newNode.currentThread().currentFrame().pc = fmt.Sprintf("Actions[%d]", i)
		newNode.currentThread().currentFrame().Name = action.Name

		_ = p.queue.Push(newNode)
	}
}

func (p *Processor) YieldFork(node *Node, process *Process) {
	for i, thread := range process.Threads {
		if thread.currentPc() == "" {
			continue
		}
		name := fmt.Sprintf("thread-%d", i)
		newNode := node.ForkForAlternatePaths(thread.Process.Fork(), name)
		newNode.current = i

		_ = p.queue.Push(newNode)
	}
	if node.actionDepth >= p.config.MaxActions ||
		len(process.Threads) >= p.config.MaxConcurrentActions {

		return
	}
	for i, action := range p.Files[0].Actions {
		newNode := node.ForkForAction(process, action)
		newNode.Process.NewThread()
		newNode.Process.current = len(newNode.Process.Threads) - 1
		newNode.currentThread().currentFrame().pc = fmt.Sprintf("Actions[%d]", i)
		newNode.currentThread().currentFrame().Name = action.Name

		_ = p.queue.Push(newNode)
	}
}

func captureStackTrace() string {
	const depth = 32
	var pcs [depth]uintptr
	n := runtime.Callers(2, pcs[:])
	if n == 0 {
		return "Unable to capture stack trace"
	}

	var sb strings.Builder
	frames := runtime.CallersFrames(pcs[:n])
	for {
		frame, more := frames.Next()
		fmt.Fprintf(&sb, "- %s:%d %s\n", frame.File, frame.Line, frame.Function)
		if !more {
			break
		}
	}

	return sb.String()
}
