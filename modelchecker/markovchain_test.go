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
			filename:   "examples/tutorials/10-coins-to-dice-atomic-3sided/ThreeSidedDie_ast.json",
			maxActions: 10,
		},
		{
			filename:   "examples/tutorials/10.1-coins-to-dice-atomic-6sided/Die_ast.json",
			maxActions: 10,
		},
		{
			filename:   "examples/tutorials/21-unfair-coin/FairCoin_ast.json",
			maxActions: 10,
		},
		{
			filename:   "examples/tutorials/24-while-stmt-atomic/FairCoin_ast.json",
			maxActions: 1,
		},
		{
			filename:   "examples/tutorials/26-unfair-coin-toss-while/FairCoin_ast.json",
			maxActions: 1,
		},
		{
			filename:   "examples/tutorials/27-unfair-coin-toss-while-noreset/FairCoin_ast.json",
			maxActions: 1,
		},
	}
	for _, test := range tests {
		t.Run(fmt.Sprintf("%s", test.filename), func(t *testing.T) {
			filename := filepath.Join(runfilesDir, "_main", test.filename)
			file, err := readAstFromFile(filename)
			require.Nil(t, err)
			files := []*ast.File{file}
			p1 := NewProcessor(files, &Options{
				IgnoreInvariantFailures:    true,
				ContinueOnInvariantFailure: true,
				MaxActions:                 test.maxActions,
			})
			root := p1.Start()
			RemoveMergeNodes(root)

			dotString := generateDotFile(root, make(map[*Node]bool))
			fmt.Printf("\n%s\n", dotString)

			steadyStateDist := steadyStateDistribution(root)
			fmt.Println(steadyStateDist)
			allNodes := getAllNodes(root)
			for j, prob := range steadyStateDist {
				if prob > 1e-6 {
					fmt.Printf("%2d: prob: %1.6f, state: %s\n", j, prob, allNodes[j].Heap.String())
				}
			}
		})
	}
}
