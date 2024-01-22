package modelchecker

import (
	"fmt"
	"math"
)

func matrixVectorProduct(matrix [][]float64, vector []float64) []float64 {
	result := make([]float64, len(vector))

	for i := range matrix {
		for j := range matrix[i] {
			result[i] += matrix[i][j] * vector[j]
		}
	}
	//fmt.Printf("Matrix Vector Product:%v\n", result)
	return result
}

func vectorNorm(vector []float64) float64 {
	sum := 0.0
	for _, v := range vector {
		sum += v * v
		//sum += v
	}
	//return sum / float64(len(vector))
	return math.Sqrt(sum)
}

func normalizeVector(vector []float64) {
	norm := vectorNorm(vector)
	for i := range vector {
		vector[i] /= norm
	}
}
func printMatrix(matrix [][]float64) {
	fmt.Println("[")
	for _, row := range matrix {
		fmt.Print("[")
		for _, v := range row {
			fmt.Printf("%f,", v)
		}
		fmt.Println("],")
	}
	fmt.Println("]")
}

func steadyStateDistribution(root *Node) []float64 {

	// Create the transition matrix
	nodes := getAllNodes(root)
	for i, node := range nodes {
		fmt.Printf("%d: %s\n", i, node.Heap.String())
	}

	transitionMatrix := createTransitionMatrix(nodes)
	//fmt.Printf("\nTransition Matrix:\n%v\n", transitionMatrix)
	transitionMatrix = transpose(transitionMatrix)
	//printMatrix(transitionMatrix)
	// Compute the matrix power (raise the matrix to a sufficiently large power)
	iterations := 20000

	initialDistribution := make([]float64, len(nodes))
	initialDistribution[0] = 1.0 // Start from the root node

	// Iterate to find the steady-state distribution
	currentDistribution := initialDistribution
	fmt.Println(currentDistribution)
	for i := 0; i < iterations; i++ { // Max iterations to avoid infinite loop
		nextDistribution := matrixVectorProduct(transitionMatrix, currentDistribution)
		fmt.Println(i, nextDistribution)
		// Check for convergence (you may define a suitable threshold)
		if vectorNorm(vectorDifference(nextDistribution, currentDistribution)) < 1e-7 {
			break
		}

		currentDistribution = nextDistribution
	}

	return currentDistribution
}

func checkLiveness(root *Node, fileId int, invariantId int) []float64 {
	// Create the transition matrix
	nodes := getAllNodes(root)

	transitionMatrix := createTransitionMatrix(nodes)
	//fmt.Printf("\nTransition Matrix:\n%v\n", transitionMatrix)
	transitionMatrix = transpose(transitionMatrix)
	for i, matrix := range transitionMatrix {
		if nodes[i].Witness[fileId][invariantId] {
			for j := range matrix {
				//if i == j {
				//	matrix[j] = 1.0
				//} else {
				matrix[j] = 0.0
				//}
			}
		}
	}
	//printMatrix(transitionMatrix)

	// Compute the matrix power (raise the matrix to a sufficiently large power)
	iterations := 2000

	initialDistribution := make([]float64, len(nodes))
	for i, _ := range initialDistribution {
		initialDistribution[i] = 1.0 / float64(len(nodes)) // Set every node to 1.0
	}

	// Iterate to find the steady-state distribution
	currentDistribution := initialDistribution
	fmt.Println(currentDistribution)
	for i := 0; i < iterations; i++ { // Max iterations to avoid infinite loop
		nextDistribution := matrixVectorProduct(transitionMatrix, currentDistribution)

		// Check for convergence (you may define a suitable threshold)
		if vectorNorm(vectorDifference(nextDistribution, currentDistribution)) < 1e-7 {
			break
		}

		currentDistribution = nextDistribution
	}

	return currentDistribution
}

func sum(distribution []float64) float64 {
	sum := 0.0
	for _, v := range distribution {
		sum += v
	}
	return sum
}
func vectorDifference(a, b []float64) []float64 {
	result := make([]float64, len(a))
	for i := range a {
		result[i] = a[i] - b[i]
	}
	return result
}

func createTransitionMatrix(nodes []*Node) [][]float64 {
	n := len(nodes)
	matrix := make([][]float64, n)
	for i := range matrix {
		matrix[i] = make([]float64, n)
	}

	indexMap := make(map[*Node]int)
	for i, node := range nodes {
		indexMap[node] = i
	}

	for _, node := range nodes {
		if len(node.outbound) == 0 {
			matrix[indexMap[node]][indexMap[node]] = 1.0
		}
		for _, outboundNode := range node.outbound {
			matrix[indexMap[node]][indexMap[outboundNode.Node]] += 1.0 / float64(len(node.outbound))
		}

	}

	return matrix
}
func transpose(matrix [][]float64) [][]float64 {
	rows := len(matrix)
	cols := len(matrix[0])

	result := make([][]float64, cols)
	for i := range result {
		result[i] = make([]float64, rows)
	}

	for i := 0; i < rows; i++ {
		for j := 0; j < cols; j++ {
			result[j][i] = matrix[i][j]
		}
	}

	return result
}
func getAllNodes(root *Node) []*Node {
	// Implement a traversal to get all nodes in the graph
	// This can be a simple depth-first or breadth-first traversal
	// depending on your requirements and graph structure.
	// For simplicity, let's assume a simple depth-first traversal here.

	visited := make(map[*Node]bool)
	var result []*Node
	traverseDFS(root, visited, &result)
	return result
}

func traverseDFS(node *Node, visited map[*Node]bool, result *[]*Node) {
	if node == nil || visited[node] {
		return
	}

	visited[node] = true
	*result = append(*result, node)

	for _, outboundNode := range node.outbound {
		traverseDFS(outboundNode.Node, visited, result)
	}
}
