package modelchecker

import (
	"fizz/ast"
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.starlark.net/starlark"
	"google.golang.org/protobuf/encoding/protojson"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestRemoveCurrentThread is a unit test for Process.removeCurrentThread.
func TestRemoveCurrentThread(t *testing.T) {
	p := &Process{
		Threads: []*Thread{
			&Thread{},
			&Thread{},
			&Thread{},
		},
		current: 1,
	}
	p.removeCurrentThread()
	assert.Equal(t, 2, len(p.Threads))
	assert.Equal(t, 0, p.current)

	p.current = 1
	p.removeCurrentThread()
	assert.Equal(t, 1, len(p.Threads))
	assert.Equal(t, 0, p.current)

	p.current = 0
	p.removeCurrentThread()
	assert.Equal(t, 0, len(p.Threads))
	assert.Equal(t, 0, p.current)
}

// TestHash is a unit test for Process.Hash.
func TestHash(t *testing.T) {
	file, err := parseAstFromString(ActionsWithMultipleBlocks)
	require.Nil(t, err)
	files := []*ast.File{file}
	process := NewProcess("", files, nil)
	process.Heap.globals = starlark.StringDict{"a": starlark.MakeInt(10), "b": starlark.MakeInt(20)}

	thread := process.currentThread()
	assert.Equal(t, thread.Stack.Len(), 1)

	thread.currentFrame().pc = "Actions[0]"

	h1 := process.HashCode()
	process.removeCurrentThread()
	assert.NotEqual(t, h1, process.HashCode())

	t0 := NewThread(process, files, 0, "Actions[0]")
	t1 := NewThread(process, files, 0, "Actions[1]")
	t2 := NewThread(process, files, 0, "Actions[2]")
	t3 := NewThread(process, files, 0, "Actions[3]")
	p1 := &Process{
		Threads: []*Thread{
			t0,
			t1,
			t2,
			t3,
		},
		current: 1,
		Heap: &Heap{
			globals: starlark.StringDict{"a": starlark.MakeInt(10), "b": starlark.MakeInt(20)},
		},
	}
	p2 := &Process{
		Threads: []*Thread{
			t2,
			t3,
			t0,
			t1,
		},
		current: 3,
		Heap: &Heap{
			globals: starlark.StringDict{"a": starlark.MakeInt(10), "b": starlark.MakeInt(20)},
		},
	}

	assert.Equal(t, p1.HashCode(), p2.HashCode())
}

func TestProcessor_Start(t *testing.T) {
	file, err := parseAstFromString(ActionsWithMultipleBlocks)
	require.Nil(t, err)
	files := []*ast.File{file}
	p1 := NewProcessor(files, &Options{
		MaxActions: 1,
	})
	root := p1.Start()
	assert.NotNil(t, root)
	assert.Equal(t, 93, len(p1.visited))
}

func printFileNames(rootDir string) error {
	return filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			relativePath, _ := filepath.Rel(rootDir, path)
			fmt.Println(relativePath)
		}
		return nil
	})
}

func TestProcessor_Tutorials(t *testing.T) {
	runfilesDir := os.Getenv("RUNFILES_DIR")
	tests := []struct {
		filename             string
		maxActions           int
		expectedNodes        int
		maxConcurrentActions int
	}{
		{
			filename:      "examples/tutorials/00-no-op/Counter_ast.json",
			maxActions:    5,
			expectedNodes: 1, // 1 nodes: 1 for the init
		},
		{
			filename:      "examples/tutorials/01-atomic-counters/Counter_ast.json",
			maxActions:    1,
			expectedNodes: 2, // 2 nodes: 1 for the init and 1 for the first action
		},
		{
			filename:      "examples/tutorials/01-atomic-counters/Counter_ast.json",
			maxActions:    3,
			expectedNodes: 4, // 2 nodes: 1 for the init and 1 for each action
		},
		{
			filename:      "examples/tutorials/01-atomic-counters/Counter_ast.json",
			maxActions:    100,
			expectedNodes: 101, // 0.01s
		},
		{
			filename:      "examples/tutorials/02-multiple-atomic-counters/Counter_ast.json",
			maxActions:    1,
			expectedNodes: 2,
		},
		{
			filename:      "examples/tutorials/02-multiple-atomic-counters/Counter_ast.json",
			maxActions:    2,
			expectedNodes: 3,
		},
		{
			filename:      "examples/tutorials/02-multiple-atomic-counters/Counter_ast.json",
			maxActions:    4,
			expectedNodes: 8,
		},
		{
			filename:      "examples/tutorials/02-multiple-atomic-counters/Counter_ast.json",
			maxActions:    10,
			expectedNodes: 144, // 0.01s
			// 20 actions, 17711 nodes, 3.79s
		},
		{
			filename:      "examples/tutorials/06-inc-dec-atomic-counters/Counter_ast.json",
			maxActions:    2,
			expectedNodes: 5,
		},
		{
			filename:      "examples/tutorials/06-inc-dec-atomic-counters/Counter_ast.json",
			maxActions:    10,
			expectedNodes: 21,
			// 20 actions, 41 nodes, 0.01s
			// this grows much slower than multiply counter, because any combination of inc / dec forms a loop
		},
		{
			filename:      "examples/tutorials/02-multiple-atomic-counters/Counter_ast.json",
			maxActions:    3,
			expectedNodes: 5,
		},
		{
			filename:      "examples/tutorials/06-inc-dec-atomic-counters/Counter_ast.json",
			maxActions:    3,
			expectedNodes: 7,
		},
		{
			filename:      "examples/tutorials/03-multiple-serial-counters/Counter_ast.json",
			maxActions:    1,
			expectedNodes: 6,
		},
		{
			filename:      "examples/tutorials/03-multiple-serial-counters/Counter_ast.json",
			maxActions:    2,
			expectedNodes: 40,
		},
		{
			filename:      "examples/tutorials/03-multiple-serial-counters/Counter_ast.json",
			maxActions:    4,
			expectedNodes: 1481,
			// 4 actions, 463 nodes, .03s
			// 5 actions, 1093 nodes, 0.09s
			// 6 actions, 2269 nodes, 0.27s
			// 10 actions, 19735 nodes, 14s
		},
		{
			filename:      "examples/tutorials/04-multiple-oneof-counters/Counter_ast.json",
			maxActions:    1,
			expectedNodes: 5, // 5 nodes: 1 for the init and 1 for each action and 1 for each stmt in add. multipy counteres end up being no-op
		},
		{
			filename:      "examples/tutorials/04-multiple-oneof-counters/Counter_ast.json",
			maxActions:    2,
			expectedNodes: 12, // 7 nodes: 1 for the init and 1 for each action and 1 for each stmt in each action
		},
		{
			filename:      "examples/tutorials/04-multiple-oneof-counters/Counter_ast.json",
			maxActions:    3,
			expectedNodes: 24,
		},
		{
			filename:      "examples/tutorials/05-multiple-parallel-counters/Counter_ast.json",
			maxActions:    1,
			expectedNodes: 8,
		},
		{
			filename:      "examples/tutorials/09-inc-dec-parallel-counters/Counter_ast.json",
			maxActions:    1,
			expectedNodes: 9,
		},
		{
			filename:      "examples/tutorials/05-multiple-parallel-counters/Counter_ast.json",
			maxActions:    2,
			expectedNodes: 47,
		},
		{
			filename:      "examples/tutorials/05-multiple-parallel-counters/Counter_ast.json",
			maxActions:    3,
			expectedNodes: 219, // .03s
		},
		{
			filename:      "examples/tutorials/10-coins-to-dice-atomic-3sided/ThreeSidedDie_ast.json",
			maxActions:    1,
			expectedNodes: 4, // 2 nodes: 1 for the init and 1 for the Toss action and 1 for each fork
		},
		{
			filename:      "examples/tutorials/10-coins-to-dice-atomic-3sided/ThreeSidedDie_ast.json",
			maxActions:    2,
			expectedNodes: 9,
		},
		{
			filename:      "examples/tutorials/10-coins-to-dice-atomic-3sided/ThreeSidedDie_ast.json",
			maxActions:    3,
			expectedNodes: 9,
		},
		{
			filename:      "examples/tutorials/10-coins-to-dice-atomic-3sided/ThreeSidedDie_ast.json",
			maxActions:    10,
			expectedNodes: 9,
		},
		{
			filename:      "examples/tutorials/13-any-stmt/Counter_ast.json",
			maxActions:    1,
			expectedNodes: 7,
		},
		{
			filename:      "examples/tutorials/13-any-stmt/Counter_ast.json",
			maxActions:    10,
			expectedNodes: 63,
		},
		{
			filename:      "examples/tutorials/14-elements-counter-atomic/Counter_ast.json",
			maxActions:    1,
			expectedNodes: 5,
		},
		{
			filename:      "examples/tutorials/14-elements-counter-atomic/Counter_ast.json",
			maxActions:    3,
			expectedNodes: 21,
		},
		{
			filename:   "examples/tutorials/14-elements-counter-atomic/Counter_ast.json",
			maxActions: 10,
			// Just one more node than 3 actions, because maximum unique state is 3 added followed by 1 remove
			expectedNodes: 22,
		},
		{
			filename:      "examples/tutorials/15-elements-counter-serial/Counter_ast.json",
			maxActions:    1,
			expectedNodes: 15,
		},
		{
			filename:      "examples/tutorials/15-elements-counter-serial/Counter_ast.json",
			maxActions:    2,
			expectedNodes: 191,
		},
		{
			filename:      "examples/tutorials/15-elements-counter-serial/Counter_ast.json",
			maxActions:    3,
			expectedNodes: 1727,
			// 4 actions, 11461 nodes, 3.37s
			// 5 actions, 62233 nodes, 65s
		},
		{
			filename:      "examples/tutorials/16-elements-counter-parallel/Counter_ast.json",
			maxActions:    1,
			expectedNodes: 14,
		},
		{
			filename:      "examples/tutorials/16-elements-counter-parallel/Counter_ast.json",
			maxActions:    2,
			expectedNodes: 146,
		},
		{
			filename:             "examples/tutorials/16-elements-counter-parallel/Counter_ast.json",
			maxActions:           3,
			expectedNodes:        1052,
			maxConcurrentActions: 3,
		},
		{
			filename:             "examples/tutorials/16-elements-counter-parallel/Counter_ast.json",
			maxActions:           3,
			maxConcurrentActions: 2,
			expectedNodes:        572, // 0.16s 131
		},
		{
			filename:             "examples/tutorials/16-elements-counter-parallel/Counter_ast.json",
			maxActions:           4,
			maxConcurrentActions: 2,
			expectedNodes:        1371, // 0.16s 162
		},
		{
			filename:      "examples/tutorials/17-for-stmt-atomic/ForLoop_ast.json",
			maxActions:    5,
			expectedNodes: 2, // Only 2 nodes, because the for loop is executed as a single action
		},
		{
			filename:   "examples/tutorials/18-for-stmt-serial/ForLoop_ast.json",
			maxActions: 2,
			// The main reason for the significant increase in the nodes is because, the two threads can execute
			// concurrently. So, in one thread might have deleted first element, then the second thread would start
			// the loop, then both the threads would start interleaving between the two threads for each iteration.
			expectedNodes: 100,
		},
		{
			filename:      "examples/tutorials/19-for-stmt-serial-check-again/ForLoop_ast.json",
			maxActions:    1,
			expectedNodes: 6,
		},
		{
			filename:      "examples/tutorials/19-for-stmt-serial-check-again/ForLoop_ast.json",
			maxActions:    2,
			expectedNodes: 20,
		},
		{
			filename:      "examples/tutorials/20-for-stmt-parallel-check-again/ForLoop_ast.json",
			maxActions:    1,
			expectedNodes: 24,
		},
		{
			filename:      "examples/tutorials/20-for-stmt-parallel-check-again/ForLoop_ast.json",
			maxActions:    2,
			expectedNodes: 150,
		},
		{
			filename:      "examples/tutorials/21-unfair-coin/FairCoin_ast.json",
			maxActions:    10,
			expectedNodes: 8,
		},
		{
			filename:      "examples/tutorials/22-while-stmt-atomic/Counter_ast.json",
			maxActions:    1,
			expectedNodes: 2,
		},
		{
			filename:      "examples/tutorials/22-while-stmt-atomic/Counter_ast.json",
			maxActions:    5,
			expectedNodes: 2,
		},
		{
			filename:      "examples/tutorials/23-while-stmt-serial/Counter_ast.json",
			maxActions:    1,
			expectedNodes: 8,
		},
		{
			filename:      "examples/tutorials/23-while-stmt-serial/Counter_ast.json",
			maxActions:    4,
			expectedNodes: 29,
		},
		{
			filename:      "examples/tutorials/24-while-stmt-atomic/FairCoin_ast.json",
			maxActions:    1,
			expectedNodes: 6,
		},
		{
			filename:      "examples/tutorials/25-break-continue/Loop_ast.json",
			maxActions:    1,
			expectedNodes: 3,
		},
		{
			filename:      "examples/tutorials/26-unfair-coin-toss-while/FairCoin_ast.json",
			maxActions:    1,
			expectedNodes: 6,
		},
		{
			filename:      "examples/tutorials/27-unfair-coin-toss-while-noreset/FairCoin_ast.json",
			maxActions:    1,
			expectedNodes: 12,
		},
		{
			filename:      "examples/tutorials/28-unfair-coin-toss-while-return/FairCoin_ast.json",
			maxActions:    1,
			expectedNodes: 6,
		},
		{
			filename:      "examples/tutorials/29-simple-function/FlipCoin_ast.json",
			maxActions:    1,
			expectedNodes: 4,
		},
		{
			filename:      "examples/tutorials/30-unfair-coin-toss-method/FairCoin_ast.json",
			maxActions:    1,
			expectedNodes: 6,
		},
		{
			filename:      "examples/tutorials/31-fair-die-from-coin-toss-method/FairDie_ast.json",
			maxActions:    1,
			expectedNodes: 14,
		},
		{
			filename:      "examples/tutorials/32-fair-die-from-unfair-coin/FairDie_ast.json",
			maxActions:    1,
			expectedNodes: 28,
		},
	}
	tempDir := CreateTempDirectory(t)
	_ = tempDir
	for _, test := range tests {
		t.Run(fmt.Sprintf("%s", test.filename), func(t *testing.T) {
			filename := filepath.Join(runfilesDir, "_main", test.filename)
			file, err := readAstFromFile(filename)
			require.Nil(t, err)
			files := []*ast.File{file}
			maxThreads := test.maxConcurrentActions
			if maxThreads == 0 {
				maxThreads = test.maxActions
			}
			p1 := NewProcessor(files, &Options{
				MaxActions:                 test.maxActions,
				MaxConcurrentActions:       maxThreads,
				IgnoreInvariantFailures:    true,
				ContinueOnInvariantFailure: true,
			})
			startTime := time.Now()
			root := p1.Start()
			assert.NotNil(t, root)
			assert.Equal(t, test.expectedNodes, len(p1.visited))
			fmt.Printf("Completed Nodes: %d, elapsed: %s\n", len(p1.visited), time.Since(startTime))
			//dotString := generateDotFile(root, make(map[*Node]bool))
			//fmt.Printf("\n%s\n", dotString)

			//RemoveMergeNodes(root)
			//// Print the modified graph
			//fmt.Println("\nModified Graph:")
			//fmt.Printf("Removing merge nodes, elapsed: %s\n", time.Since(startTime))
			//dotString := generateDotFile(root, make(map[*Node]bool))
			//fmt.Printf("Generating dotfile, elapsed: %s\n", time.Since(startTime))
			//dotFileName := RemoveLastSegment(filename, ".json") + ".dot"
			//WriteFile(t, tempDir, dotFileName, []byte(dotString))
			//fmt.Printf("Writing dotfile, elapsed: %s\n", time.Since(startTime))
			//fmt.Printf("\n%s\n", dotString)
		})
	}
}

func readAstFromFile(filename string) (*ast.File, error) {
	jsonFile, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer jsonFile.Close()
	bytes, _ := io.ReadAll(jsonFile)
	f := &ast.File{}
	err = protojson.Unmarshal(bytes, f)
	return f, err
}

func CreateTempDirectory(t *testing.T) string {
	tempDir, err := ioutil.TempDir("", "test_artifacts_")
	if err != nil {
		t.Fatalf("Failed to create temporary directory: %v", err)
	}
	//defer os.RemoveAll(tempDir)
	return tempDir
}

func WriteFile(t *testing.T, tempDir string, filename string, content []byte) {
	fullPath := filepath.Join(tempDir, filename)
	dir := filepath.Dir(fullPath)
	err := os.MkdirAll(dir, os.ModePerm)
	if err != nil {
		fmt.Printf("Error creating directory %s: %v\n", dir, err)
		return
	}
	fmt.Println("Writing file: ", fullPath)
	err = os.WriteFile(fullPath, content, 0644)
	if err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}
}
