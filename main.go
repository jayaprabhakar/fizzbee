package main

import (
	"fizz/ast"
	"flag"
	"fmt"
	"github.com/golang/glog"
	"github.com/jayaprabhakar/fizzbee/modelchecker"
	"google.golang.org/protobuf/encoding/protojson"
	"io/ioutil"
	"math/rand"
	"os"
)

func main() {
	flag.Parse()
	defer glog.Flush()
	path, err := os.Getwd()
	// handle err
	fmt.Println(path)

	// Open our jsonFile
	jsonFile, err := os.Open("examples/ast/streaming_counter_ast.json")
	// if we os.Open returns an error then handle it
	if err != nil {
		fmt.Println(err)
		return
	}
	glog.Infof("Successfully Opened users.json")
	// defer the closing of our jsonFile so that we can parse it later on
	defer jsonFile.Close()

	bytes, _ := ioutil.ReadAll(jsonFile)

	//  bytes,err := protojson.Marshal(f)
	//  fmt.Println(string(bytes))
	//  fmt.Println(err)

	f := &ast.File{}
	err = protojson.Unmarshal(bytes, f)
	fmt.Println(f)
	fmt.Println(err)

	mc := modelchecker.NewModelChecker("example")

	globals, err := mc.ExecInit(f.Variables)
	if err != nil {
		panic(err)
	}

	// Print the global environment.
	fmt.Println("\nGlobals:")
	for _, name := range globals.Keys() {
		v := globals[name]
		fmt.Printf("%s (%s) = %s\n", name, v.Type(), v.String())
	}

	fmt.Println("Running actions")
	for _, action := range f.Actions {
		fmt.Printf("Action: %s\n", action.Name)

		mc.ExecAction("myfilename.fizz", action, globals)
	}

	// Randomly select multiple actions to run
	for i := 0; i < 3; i++ {
		action := f.Actions[rand.Intn(len(f.Actions))]
		fmt.Printf("------\nAction: %s\n", action.Name)
		mc.ExecAction("myfilename.fizz", action, globals)
	}

}
