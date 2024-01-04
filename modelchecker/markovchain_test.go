package modelchecker

import (
	"fizz/ast"
	"fmt"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"testing"
)

func TestSteadyStateDistribution(t *testing.T) {

	runfilesDir := os.Getenv("RUNFILES_DIR")
	tests := []struct {
		filename      string
		maxActions    int
		expectedNodes int
	}{

		{
			filename:      "examples/tutorials/10-coins-to-dice-atomic-3sided/ThreeSidedDie_ast.json",
			maxActions:    10,
			expectedNodes: 9,
		},
	}
	for _, test := range tests {
		t.Run(fmt.Sprintf("%s", test.filename), func(t *testing.T) {
			filename := filepath.Join(runfilesDir, "_main", test.filename)
			file, err := readAstFromFile(filename)
			require.Nil(t, err)
			files := []*ast.File{file}
			p1 := NewProcessor(files, &Options{
				MaxActions: test.maxActions,
			})
			root := p1.Start()
			RemoveMergeNodes(root)

			dotString := generateDotFile(root, make(map[*Node]bool))
			fmt.Printf("\n%s\n", dotString)

			steadyStateDist := steadyStateDistribution(root)
			fmt.Println(steadyStateDist)

		})
	}
}
