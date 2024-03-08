package main

import (
    "encoding/json"
    "errors"
    ast "fizz/proto"
    "fmt"
    "github.com/jayaprabhakar/fizzbee/modelchecker"
    "google.golang.org/protobuf/encoding/protojson"
    "os"
    "path/filepath"
    "slices"
    "time"
)

func main() {
    // Check if the correct number of arguments is provided
    if len(os.Args) != 2 {
        fmt.Println("Usage:", os.Args[0], "<json_file>")
        os.Exit(1)
    }

    // Get the input JSON file name from command line argument
    jsonFilename := os.Args[1]

    // Read the content of the JSON file
    jsonContent, err := os.ReadFile(jsonFilename)
    if err != nil {
        fmt.Println("Error reading JSON file:", err)
        os.Exit(1)
    }
    f := &ast.File{}
    err = protojson.Unmarshal(jsonContent, f)
    if err != nil {
        fmt.Println("Error unmarshalling JSON:", err)
        os.Exit(1)
    }

    dirPath := filepath.Dir(jsonFilename)
    fmt.Println("dirPath:", dirPath)
    // Calculate the relative path
    configFileName := filepath.Join(dirPath, "fizz.yaml")
    fmt.Println("configFileName:", configFileName)
    stateConfig, err := modelchecker.ReadOptionsFromYaml(configFileName)
    if err != nil {
        if errors.Is(err, os.ErrNotExist) {
            fmt.Println("fizz.yaml not found. Using default options")
            stateConfig = &ast.StateSpaceOptions{Options: &ast.Options{MaxActions: 100, MaxConcurrentActions: 5}}
        } else {
            fmt.Println("Error reading fizz.yaml:", err)
            os.Exit(1)
        }

    }
    if stateConfig.Options.MaxConcurrentActions == 0 {
        stateConfig.Options.MaxConcurrentActions = stateConfig.Options.MaxActions
    }

    p1 := modelchecker.NewProcessor([]*ast.File{f}, stateConfig)
    startTime := time.Now()
    rootNode, failedNode, err := p1.Start()
    endTime := time.Now()
    fmt.Printf("Time taken for model checking: %v\n", endTime.Sub(startTime))



    outDir, err := createOutputDir(dirPath)
    if err != nil {
        return
    }
    if p1.GetVisitedNodesCount() < 150 {
        dotString := modelchecker.GenerateDotFile(rootNode, make(map[*modelchecker.Node]bool))
        dotFileName := filepath.Join(outDir, "graph.dot")
        // Write the content to the file
        err := os.WriteFile(dotFileName, []byte(dotString), 0644)
        if err != nil {
            fmt.Println("Error writing to file:", err)
            return
        }
        fmt.Printf("Writen graph dotfile: %s\nTo generate png, run: \n" +
            "dot -Tpng %s -o graph.png && open graph.png\n", dotFileName, dotFileName)
    } else {
        fmt.Printf("Skipping dotfile generation. Too many nodes: %d\n", p1.GetVisitedNodesCount())
    }

    if err != nil {
        var modelErr *modelchecker.ModelError
        if errors.As(err, &modelErr) {
            fmt.Println("Stack Trace:")
            fmt.Println(modelErr.SprintStackTrace())
        } else {
            fmt.Println("Error:", err)
        }
        os.Exit(1)
    }

    //fmt.Println("root", root)
    if failedNode == nil {
        //failurePath := nil
        //failedInvariant := nil
        var failurePath []*modelchecker.Node
        var failedInvariant *modelchecker.InvariantPosition
        if stateConfig.GetLiveness() == "strict" {
            nodes, _ := modelchecker.GetAllNodes(rootNode)
            failurePath, failedInvariant = modelchecker.CheckFastLiveness(nodes)
            fmt.Printf("IsLive: %t\n", failedInvariant == nil)
            fmt.Printf("Time taken to check liveness: %v\n", time.Now().Sub(endTime))
        } else if stateConfig.GetLiveness() == "strict/bfs" {
            failurePath, failedInvariant = modelchecker.CheckStrictLiveness(rootNode)
            fmt.Printf("IsLive: %t\n", failedInvariant == nil)
            fmt.Printf("Time taken to check liveness: %v\n", time.Now().Sub(endTime))
        }

        if failedInvariant == nil {
            fmt.Println("PASSED: Model checker completed successfully")
            nodes, _ := modelchecker.GetAllNodes(rootNode)
            nodeFiles, linkFileNames, err := modelchecker.GenerateProtoOfJson(nodes, outDir+"/")
            if err != nil {
                fmt.Println("Error generating proto files:", err)
                return
            }
            fmt.Printf("Writen %d node files and %d link files to dir %s\n", len(nodeFiles), len(linkFileNames), outDir)
        } else {
            fmt.Println("FAILED: Liveness check failed")
            if failedInvariant.FileIndex > 0 {
                fmt.Printf("Only one file expected. Got %d\n", failedInvariant.FileIndex)
            } else {
                fmt.Printf("Invariant: %s\n", f.Invariants[failedInvariant.InvariantIndex].Name)
            }
            GenerateFailurePath(failurePath, failedInvariant, outDir)
        }

        return
    }
    fmt.Println("FAILED: Model checker failed")

    // newStack of *Node
    failurePath := make([]*modelchecker.Node, 0)

    node := failedNode
    //fmt.Println(node.String())
    for node != nil {
        failurePath = append(failurePath, node)
        if len(node.Inbound) == 0 || node.Name == "init" || node == rootNode {
            break
        }
        //node.Name = node.Name + "/" + node.Inbound[0].Name
        node = node.Inbound[0].Node
    }
    slices.Reverse(failurePath)
    GenerateFailurePath(failurePath, nil, outDir)
}

func GenerateFailurePath(failurePath []*modelchecker.Node, invariant *modelchecker.InvariantPosition, outDir string) {
    for _, node := range failurePath {
        stepName := ""
        if len(node.Inbound) > 0 {
            stepName = node.Inbound[0].Name
        }
        if stepName == "" || stepName == "stutter" {
            stepName = node.GetName()
        }
        fmt.Printf("------\n%s\n", stepName)

        fmt.Printf("--\nstate: %s\n", node.Heap.ToJson())
        if len(node.Returns) > 0 {
            fmt.Printf("returns: %s\n", node.Returns.String())
        }
    }
    fmt.Println("------")
    errJsonFileName := filepath.Join(outDir, "error-graph.json")
    bytes, err := json.MarshalIndent(failurePath, "", "  ")
    if err != nil {
        fmt.Println("Error creating json:", err)
    }
    err = os.WriteFile(errJsonFileName, bytes, 0644)
    if err != nil {
        fmt.Println("Error writing to file:", err)
        return
    }
    fmt.Printf("Writen graph json: %s\n", errJsonFileName)
    dotStr := modelchecker.GenerateFailurePath(failurePath, invariant)
    //fmt.Println(dotStr)
    dotFileName := filepath.Join(outDir, "error-graph.dot")
    // Write the content to the file
    err = os.WriteFile(dotFileName, []byte(dotStr), 0644)
    if err != nil {
        fmt.Println("Error writing to file:", err)
        return
    }
    fmt.Printf("Writen graph dotfile: %s\nTo generate png, run: \n"+
        "dot -Tpng %s -o graph.png && open graph.png\n", dotFileName, dotFileName)
}

func createOutputDir(dirPath string) (string, error) {
    // Create the directory name with current date and time
    dateTimeStr := time.Now().Format("2006-01-02_15-04-05") // Format: YYYY-MM-DD_HH-MM-SS
    newDirName := fmt.Sprintf("run_%s", dateTimeStr)

    // Create the full path for the new directory
    newDirPath := filepath.Join(dirPath, "out", newDirName)

    // Create the directory
    if err := os.MkdirAll(newDirPath, 0755); err != nil {
        fmt.Println("Error creating directory:", err)
        return newDirPath, err
    }
    return newDirPath, nil
}
