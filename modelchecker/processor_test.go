package modelchecker

import (
	"fizz/ast"
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.starlark.net/starlark"
	"google.golang.org/protobuf/encoding/protojson"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
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
	assert.Equal(t, 124, len(p1.visited))
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
	err := printFileNames(runfilesDir)
	require.Nil(t, err)
	tests := []struct {
		filename      string
		maxActions    int
		expectedNodes int
	}{
		//{
		//	filename:      "examples/tutorials/01-atomic-counters/Counter_ast.json",
		//	maxActions:    1,
		//	expectedNodes: 2, // 2 nodes: 1 for the init and 1 for the first action
		//},
		//{
		//	filename:      "examples/tutorials/01-atomic-counters/Counter_ast.json",
		//	maxActions:    3,
		//	expectedNodes: 2,
		//},
		//{
		//	filename:      "examples/tutorials/02-multiple-atomic-counters/Counter_ast.json",
		//	maxActions:    1,
		//	expectedNodes: 3,
		//},
		//{
		//	filename:      "examples/tutorials/03-multiple-serial-counters/Counter_ast.json",
		//	maxActions:    1,
		//	expectedNodes: 3, // 3 nodes: 1 for the init and 1 for each action
		//},
		{
			filename:      "examples/tutorials/04-multiple-oneof-counters/Counter_ast.json",
			maxActions:    1,
			expectedNodes: 7, // 7 nodes: 1 for the init and 1 for each action and 1 for each stmt in each action
		},
		{
			filename:   "examples/tutorials/05-multiple-parallel-counters/Counter_ast.json",
			maxActions: 1,
			// 11 nodes: 1 for the init and 1 for each action, then within each action, 4 possible combinations of stmts
			// [s1], [s2], [s1, s2], [s2, s1]. So, 1 + 2 + 4 + 4 = 11
			expectedNodes: 11,
		},
	}

	for _, test := range tests {
		filename := filepath.Join(runfilesDir, "_main", test.filename)
		file, err := readAstFromFile(filename)
		require.Nil(t, err)
		files := []*ast.File{file}
		p1 := NewProcessor(files, &Options{
			MaxActions: test.maxActions,
		})
		root := p1.Start()
		assert.NotNil(t, root)
		assert.Equal(t, test.expectedNodes, len(p1.visited))
	}
}

//
//filename := "../examples/tutorials/01-atomic-counters/Counter_ast.json"
//file, err := readAstFromFile(filename)
//
//require.Nil(t, err)
//files := []*ast.File{file}
//p1 := NewProcessor(files, &Options{
//	MaxActions: 1,
//})
//root := p1.Start()
//assert.NotNil(t, root)
//assert.Equal(t, 2, len(p1.visited))

func readAstFromFile(filename string) (*ast.File, error) {
	jsonFile, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer jsonFile.Close()
	bytes, _ := ioutil.ReadAll(jsonFile)
	f := &ast.File{}
	err = protojson.Unmarshal(bytes, f)
	return f, err
}
