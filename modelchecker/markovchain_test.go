package modelchecker

import (
	ast "fizz/proto"
	"fmt"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"testing"
)

func TestSteadyStateDistribution(t *testing.T) {

	runfilesDir := os.Getenv("RUNFILES_DIR")
	tests := []struct {
	filename             string
	maxActions           int
	expectedNodes        int
	maxConcurrentActions int
	perfModel            *ast.PerformanceModel
}{

		{
			filename:   "examples/tutorials/10-coins-to-dice-atomic-3sided/ThreeSidedDie.json",
			maxActions: 10,
		},
		{
			filename:   "examples/tutorials/10.1-coins-to-dice-atomic-6sided/Die.json",
			maxActions: 10,
		},
		{
			filename:   "examples/tutorials/21-unfair-coin/FairCoin.json",
			maxActions: 10,
		},
		{
			filename:   "examples/tutorials/24-while-stmt-atomic/FairCoin.json",
			maxActions: 1,
		},
		{
			filename:   "examples/tutorials/26-unfair-coin-toss-while/FairCoin.json",
			maxActions: 1,
		},
		{
			filename:   "examples/tutorials/27-unfair-coin-toss-while-noreset/FairCoin.json",
			maxActions: 1,
		},
		{
			filename:   "examples/tutorials/28-unfair-coin-toss-while-return/FairCoin.json",
			maxActions: 1,
		},
		{
			filename:   "examples/tutorials/29-simple-function/FlipCoin.json",
			maxActions: 1,
		},
		{
			filename:   "examples/tutorials/30-unfair-coin-toss-method/FairCoin.json",
			maxActions: 1,
		},
		{
			filename:   "examples/tutorials/31-fair-die-from-coin-toss-method/FairDie.json",
			maxActions: 1,
		},
		{
			filename:   "examples/tutorials/31-fair-die-from-coin-toss-method/FairDie.json",
			maxActions: 1,
			perfModel:     &ast.PerformanceModel{
				Configs: map[string]*ast.TransitionConfig{
					"Toss.call": {
						Counters: map[string]*ast.Counter{
							"toss": {Numeric: 1},
						},
					},
				},
			},
		},
		{
			filename:   "examples/tutorials/32-fair-die-from-unfair-coin/FairDie.json",
			maxActions: 1,
		},
		{
			filename:   "examples/tutorials/33-fair-die-from-coin-toss-method-any-stmt/FairDie.json",
			maxActions: 1,
		},
		{
			filename:   "examples/tutorials/16-elements-counter-parallel/Counter.json",
			maxActions: 2,
		},
		{
			filename:      "examples/tutorials/34-simple-hour-clock/HourClock.json",
			maxActions:    100,
		},
		{
			filename:      "examples/tutorials/37-unfair-coin-toss-labels/FairCoin.json",
			maxActions:    1,
			perfModel:     &ast.PerformanceModel{},
		},
		{
			filename:      "examples/tutorials/37-unfair-coin-toss-labels/FairCoin.json",
			maxActions:    1,
			perfModel:     &ast.PerformanceModel{
				Configs: map[string]*ast.TransitionConfig{
					"UnfairToss.head": {Probability: 0.99},
					"UnfairToss.tail": {Probability: 0.01},
					"UnfairToss.call": {
						Counters: map[string]*ast.Counter{
							"toss": {Numeric: 1},
							"latency": {Numeric: 0.5},
						},
					},
				},
			},
		},
		{
			filename:      "examples/tutorials/38-two-dice-with-coins/TwoDice.json",
			maxActions:    1,
			perfModel:     &ast.PerformanceModel{
				Configs: map[string]*ast.TransitionConfig{
					"Toss.call": {
						Counters: map[string]*ast.Counter{
							"toss": {Numeric: 1},
						},
					},
				},
			},
		},
	}
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
				IgnoreInvariantFailures:    true,
				ContinueOnInvariantFailure: true,
				MaxActions:                 test.maxActions,
				MaxConcurrentActions:       maxThreads,
			})
			root, _, _ := p1.Start()
			RemoveMergeNodes(root)

			dotString := generateDotFile(root, make(map[*Node]bool))
			fmt.Printf("\n%s\n", dotString)

			perfModel := test.perfModel
			if perfModel == nil {
				perfModel = &ast.PerformanceModel{}
			}

			steadyStateDist := steadyStateDistribution(root, perfModel)
			fmt.Println(steadyStateDist)
			allNodes := getAllNodes(root)
			for j, prob := range steadyStateDist {
				if prob > 1e-6 {
					fmt.Printf("%2d: prob: %1.6f, state: %s / returns: %s\n", j, prob, allNodes[j].Heap.String(), allNodes[j].Returns.String())
				}
			}
			for k, inv := range files[0].Invariants {
				if !inv.Eventually {
					continue
				}
				liveness := checkLiveness(root, 0, k)
				fmt.Println(liveness)
				fmt.Println("Liveness")
				for j, prob := range liveness {
					if prob > 1e-6 {
						status := "DEAD"
						if allNodes[j].Process.Witness[0][k] {
							status = "LIVE"
						}
						fmt.Printf("%s %3d: prob: %1.6f, state: %s / returns: %s\n", status, j, prob, allNodes[j].Heap.String(), allNodes[j].Returns.String())
					}
				}
			}
		})
	}
}
