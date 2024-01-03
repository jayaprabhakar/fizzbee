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
	"fizz/ast"
	"fmt"
	"github.com/zeroflucs-given/generics/collections"
	_ "github.com/zeroflucs-given/generics/collections"
	"github.com/zeroflucs-given/generics/collections/linkedlist"
	_ "github.com/zeroflucs-given/generics/collections/linkedlist"
	"go.starlark.net/starlark"
	"os"
	"runtime"
	"sort"
	"strings"
)

type Process struct {
	Heap      *Heap
	Threads   []*Thread
	current   int
	Name      string
	Files     []*ast.File
	Parent    *Process
	Evaluator *Evaluator
	Children  []*Process
}

func NewProcess(name string, Files []*ast.File, parent *Process) *Process {
	var mc *Evaluator
	if parent == nil {
		mc = NewModelChecker("example")
	} else {
		mc = parent.Evaluator
	}
	p := &Process{
		Name:      name,
		Heap:      &Heap{starlark.StringDict{}},
		Threads:   []*Thread{},
		current:   0,
		Files:     Files,
		Parent:    parent,
		Evaluator: mc,
		Children:  []*Process{},
	}
	p.Children = append(p.Children, p)
	thread := NewThread(p, Files, 0, "")
	p.Threads = append(p.Threads, thread)
	return p
}

func (p *Process) Fork() *Process {
	p2 := &Process{
		Name:      p.Name,
		Heap:      p.Heap.Clone(),
		current:   p.current,
		Parent:    p,
		Evaluator: p.Evaluator,
		Children:  []*Process{},
		Files:     p.Files,
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
	buf := strings.Builder{}
	buf.WriteString(fmt.Sprintf("Process: %s\n", p.Name))
	buf.WriteString(fmt.Sprintf("Actions: %d, Forks: %d\n", n.actionDepth, n.forkDepth))

	// Your original JSON string
	jsonString := p.Heap.String()

	// Escape double quotes
	escapedString := strings.ReplaceAll(jsonString, "\"", "\\\"")
	buf.WriteString(escapedString)

	return buf.String()
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

	// hash the heap variables as well
	heapHash := p.Heap.HashCode()
	h.Write([]byte(heapHash))
	return fmt.Sprintf("%x", h.Sum(nil))
}

func (p *Process) currentThread() *Thread {
	return p.Threads[p.current]
}

func (p *Process) removeCurrentThread() {
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

type Node struct {
	*Process

	inbound  []*Node
	outbound []*Node

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

func NewNode(process *Process) *Node {
	return &Node{
		Process:     process,
		inbound:     make([]*Node, 0),
		outbound:    make([]*Node, 0),
		actionDepth: 0,
		forkDepth:   0,
		stacktrace:  captureStackTrace(),
	}
}

func (n *Node) Merge(other *Node) *Node {
	mergeNode := &Node{
		inbound:     []*Node{n},
		outbound:    []*Node{other},
		actionDepth: n.actionDepth,
		forkDepth:   n.forkDepth,
	}
	n.outbound = append(n.outbound, mergeNode)
	other.inbound = append(other.inbound, mergeNode)
	return mergeNode
}

func (n *Node) ForkForAction(action *ast.Action) *Node {
	forkNode := &Node{
		Process:     n.Process.Fork(),
		inbound:     []*Node{n},
		outbound:    []*Node{},
		actionDepth: n.actionDepth + 1,
		forkDepth:   n.forkDepth + 1,
		stacktrace:  captureStackTrace(),
	}
	forkNode.Process.Name = action.Name

	n.outbound = append(n.outbound, forkNode)
	return forkNode
}

func (n *Node) ForkForAlternatePaths(process *Process) *Node {

	forkNode := &Node{
		Process:     process,
		inbound:     []*Node{n},
		outbound:    []*Node{},
		actionDepth: n.actionDepth,
		forkDepth:   n.forkDepth + 1,
		stacktrace:  captureStackTrace(),
	}

	n.outbound = append(n.outbound, forkNode)
	return forkNode
}

type Options struct {
	// The maximum number of nodes to process
	MaxNodes int
	// The maximum number of actions to process
	MaxActions int
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

func (p *Processor) Start() *Node {
	if p.Init != nil {
		panic("processor already started")
	}
	process := NewProcess("init", p.Files, nil)
	init := NewNode(process)
	globals, err := process.Evaluator.ExecInit(p.Files[0].States)
	if err != nil {
		panic(err)
	}
	process.Heap.globals = globals
	p.Init = init

	_ = p.queue.Push(p.Init)
	//p.visited[p.Init.HashCode()] = p.Init

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
		//p.visited[process.HashCode()] = node
		p.processNode(node)
		p.visited[node.HashCode()] = node
	}
	return p.Init
}

func (p *Processor) processNode(node *Node) {
	if node.Process.currentThread().currentPc() == "" && node.Name == "init" {
		node.Process.removeCurrentThread()
		// This is init node, generate a fork for each action in the file
		for i, action := range p.Files[0].Actions {
			newNode := node.ForkForAction(action)
			//newNode.Process.removeCurrentThread()
			thread := newNode.Process.NewThread()
			//thread := newNode.currentThread()
			thread.currentFrame().pc = fmt.Sprintf("Actions[%d]", i)
			_ = p.queue.Push(newNode)
		}
		return
	}
	forks, yield := node.currentThread().Execute()
	//fmt.Printf("Forks: %d, Yield: %t, Threads: %d\n", len(forks), yield, len(node.Threads))
	if other, ok := p.visited[node.HashCode()]; ok {
		// Check if visited before scheduling children
		node.Merge(other)
		return
	}
	if !yield {
		for _, fork := range forks {
			newNode := node.ForkForAlternatePaths(fork)
			_ = p.queue.Push(newNode)
		}
		return
	}

	if yield {
		if len(forks) > 0 {
			//fmt.Println("yield and fork at the same time")
			for _, fork := range forks {
				p.YieldFork(node, fork)
			}
		} else {
			p.YieldNode(node)
		}

		return
	}
	//for _, fork := range forks {
	//	nodes = append(nodes, node.ForkForAlternatePaths(fork))
	//}
	//if yield && len(forks) > 0 {
	//	panic("yield and fork at the same time, not sure if it is needed")
	//}
	//return nodes
}

func (p *Processor) YieldNode(node *Node) {
	if other, ok := p.visited[node.HashCode()]; ok {
		// Check if visited before scheduling children
		node.Merge(other)
		return
	}
	for i, thread := range node.Threads {
		if thread.currentPc() == "" {
			continue
		}
		newNode := node.ForkForAlternatePaths(thread.Process.Fork())
		newNode.current = i

		_ = p.queue.Push(newNode)
	}
	if node.actionDepth >= p.config.MaxActions {
		return
	}
	for i, action := range p.Files[0].Actions {
		newNode := node.ForkForAction(action)
		newNode.Process.NewThread()
		newNode.Process.current = len(newNode.Process.Threads) - 1
		newNode.currentThread().currentFrame().pc = fmt.Sprintf("Actions[%d]", i)

		if strings.Contains(newNode.currentThread().currentPc(), ".$") {
			fmt.Println("PC contains $")
			fmt.Printf("node: %+v\n", newNode)
			fmt.Printf("node.heap: %+v\n", newNode.Heap.String())
			fmt.Printf("node.currentThread().currentPc(): %+v\n", newNode.currentThread().currentPc())
			_, _ = fmt.Fprintf(os.Stderr, "node.currentThread().currentPc(): %v\n", newNode.currentThread().currentPc())
		}

		_ = p.queue.Push(newNode)
	}
}

func (p *Processor) YieldFork(node *Node, process *Process) {
	for i, thread := range process.Threads {
		if thread.currentPc() == "" {
			continue
		}
		newNode := node.ForkForAlternatePaths(thread.Process.Fork())
		newNode.current = i

		_ = p.queue.Push(newNode)
	}
	if node.actionDepth >= p.config.MaxActions {
		return
	}
	for i, action := range p.Files[0].Actions {
		newNode := node.ForkForAction(action)
		newNode.Process.NewThread()
		newNode.Process.current = len(newNode.Process.Threads) - 1
		newNode.currentThread().currentFrame().pc = fmt.Sprintf("Actions[%d]", i)

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
