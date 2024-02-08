package main

import (
    "errors"
    ast "fizz/proto"
    "fmt"
    "github.com/jayaprabhakar/fizzbee/modelchecker"
    "google.golang.org/protobuf/encoding/protojson"
    "os"
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

    p1 := modelchecker.NewProcessor([]*ast.File{f}, &ast.StateSpaceOptions{
        Options:                         &ast.Options{
            MaxActions:           5,
            MaxConcurrentActions: 3,
        },
    })
    startTime := time.Now()
    _, failedNode, err := p1.Start()
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
    endTime := time.Now()
    fmt.Printf("Time taken: %v\n", endTime.Sub(startTime))
    //fmt.Println("root", root)
    if failedNode == nil {
        fmt.Println("PASSED: Model checker completed successfully")
        return
    }
    fmt.Println("FAILED: Model checker failed")

    // newStack of *Node
    nodes := make([]*modelchecker.Node, 0)

    node := failedNode
    //fmt.Println(node.String())
    for node != nil {
        nodes = append(nodes, node)
        if len(node.Inbound) == 0 || node.Name == "init" {
            break
        }
        node.Name = node.Name + "/" + node.Inbound[0].Name
        node = node.Inbound[0].Node
    }
    for i := len(nodes) - 1; i >= 0; i-- {
        node = nodes[i]
        fmt.Printf("--\n%s\n", node.GetName())
        fmt.Printf("--\n%s\n", node.GetStateString())
    }
}
