package main

import (
	"fizz/ast"
	"fmt"
	"go.starlark.net/starlark"
	"go.starlark.net/syntax"
	"google.golang.org/protobuf/encoding/protojson"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"strings"
)

func main() {
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
	fmt.Println("Successfully Opened users.json")
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

	var sb strings.Builder
	for _, v := range f.Variables {
		fmt.Fprintf(&sb, "%s = %s\n", v.Name, v.Expression)
	}
	initStr := sb.String()

	predeclared := starlark.StringDict{}

	thread := &starlark.Thread{
		Name:  "example",
		Print: func(_ *starlark.Thread, msg string) { fmt.Println(msg) },
	}

	fmt.Println("Running Init")
	options := &syntax.FileOptions{Set: true, GlobalReassign: true, TopLevelControl: true}
	globals, err := starlark.ExecFileOptions(options, thread, "apparent/filename.star", initStr, predeclared)
	if err != nil {
		if evalErr, ok := err.(*starlark.EvalError); ok {
			log.Fatal(evalErr.Backtrace())
		}
		log.Fatal(err)
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

		ExecAction(options, thread, "myfilename.fizz", action, globals)
	}

	// Randomly select multiple actions to run
	for i := 0; i < 10; i++ {
		action := f.Actions[rand.Intn(len(f.Actions))]
		fmt.Printf("------\nAction: %s\n", action.Name)
		ExecAction(options, thread, "myfilename.fizz", action, globals)
	}

}

func ExecAction(options *syntax.FileOptions, thread *starlark.Thread, filename string, action *ast.Action, prevState starlark.StringDict) {
	ExecBlock(options, thread, filename, action.Block, prevState)
}

func ExecBlock(options *syntax.FileOptions, thread *starlark.Thread, filename string, block *ast.Block, prevState starlark.StringDict) bool {
	valid := false
	for _, stmt := range block.Stmts {
		if stmt.PyStmt != nil {
			valid = ExecPyStmt(options, thread, filename, stmt.PyStmt, prevState) || valid
		} else if stmt.AnyStmt != nil {
			valid = ExecAnyStmt(options, thread, filename, stmt.AnyStmt, prevState) || valid
		} else if stmt.ForStmt != nil {
			valid = ExecForStmt(options, thread, filename, stmt.ForStmt, prevState) || valid
		} else if stmt.IfStmt != nil {
			valid = ExecIfStmt(options, thread, filename, stmt.IfStmt, prevState) || valid
		}
	}
	return valid
}

func ExecIfStmt(options *syntax.FileOptions, thread *starlark.Thread, filename string, ifStmt *ast.IfStmt, prevState starlark.StringDict) bool {
	valid := false

	for _, branch := range ifStmt.Branches {
		val := EvalPyExpr(options, thread, filename, branch.Condition, prevState)
		fmt.Printf("Branch: %v, value: %s", branch, val)
		if val.Truth() {
			v := ExecBlock(options, thread, filename, branch.Block, prevState)
			return v
		}
	}

	return valid
}

func ExecForStmt(options *syntax.FileOptions, thread *starlark.Thread, filename string, forStmt *ast.ForStmt, prevState starlark.StringDict) bool {
	valid := false

	val := EvalPyExpr(options, thread, filename, forStmt.PyExpr, prevState)
	rangeVal, _ := val.(starlark.Iterable)
	iter := rangeVal.Iterate()
	defer iter.Done()
	var x starlark.Value
	if len(forStmt.LoopVars) != 1 {
		log.Fatal("Not supported: multiple variables in forstmt")
	}
	loopVar := forStmt.LoopVars[0]
	if _, ok := prevState[loopVar]; ok {
		log.Fatal("Not supported: overriding variables in nested scope")
	}
	for iter.Next(&x) {
		fmt.Printf("Iter: %s, Type: %s\n", x, x.Type())
		prevState[loopVar] = x
		match := ExecBlock(options, thread, filename, forStmt.Block, prevState)
		valid = match || valid
	}
	delete(prevState, loopVar)
	return valid
}

func ExecAnyStmt(options *syntax.FileOptions, thread *starlark.Thread, filename string, anyStmt *ast.AnyStmt, prevState starlark.StringDict) bool {
	valid := false

	val := EvalPyExpr(options, thread, filename, anyStmt.PyExpr, prevState)
	rangeVal, _ := val.(starlark.Iterable)
	iter := rangeVal.Iterate()
	defer iter.Done()

	fmt.Printf("LoopVars: %s\n", anyStmt.LoopVars)
	if len(anyStmt.LoopVars) != 1 {
		log.Fatal("Not supported: multiple variables in anystmt")
	}
	loopVar := anyStmt.LoopVars[0]
	if _, ok := prevState[loopVar]; ok {
		log.Fatal("Not supported: overriding variables in nested scope")
	}
	var x starlark.Value
	for iter.Next(&x) {
		fmt.Printf("Iter: %s, Type: %s\n", x, x.Type())
		prevState[loopVar] = x
		match := ExecBlock(options, thread, filename, anyStmt.Block, prevState)
		valid = match || valid
		if match {
			break
		}
	}
	delete(prevState, loopVar)
	return valid
}

func ExecPyStmt(options *syntax.FileOptions, thread *starlark.Thread, filename string, stmt *ast.PyStmt, prevState starlark.StringDict) bool {

	fmt.Printf("\nExec Stmt: %v\n", stmt)

	starCode := stmt.Code
	globals, err := starlark.ExecFileOptions(options, thread, filename, starCode, prevState)
	if err != nil {
		if evalErr, ok := err.(*starlark.EvalError); ok {
			log.Fatal(evalErr.Backtrace())
		}
		log.Fatal(err)
	}

	// Print the global environment.
	fmt.Println("Globals:")
	for _, name := range globals.Keys() {
		v := globals[name]
		fmt.Printf("%s (%s) = %s\n", name, v.Type(), v.String())

		prevState[name] = v
	}
	return true
}

func EvalPyExpr(options *syntax.FileOptions, thread *starlark.Thread, filename string, src string, prevState starlark.StringDict) starlark.Value {

	fmt.Printf("\nEval Stmt: %v\n", src)

	starCode := src
	value, err := starlark.EvalOptions(options, thread, filename, starCode, prevState)
	if err != nil {
		if evalErr, ok := err.(*starlark.EvalError); ok {
			log.Fatal(evalErr.Backtrace())
		}
		log.Fatal(err)
	}

	// Print the global environment.
	fmt.Printf("EvalResult GoType: %T, StarlarkType: %s, Value: %s\n", value, value.Type(), value)
	return value
}
