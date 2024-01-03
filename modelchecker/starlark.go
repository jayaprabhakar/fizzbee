package modelchecker

import (
	"fizz/ast"
	"github.com/golang/glog"
	"go.starlark.net/starlark"
)

func (e *Evaluator) EvalPyExpr(filename string, src interface{}, prevState starlark.StringDict) (starlark.Value, error) {
	glog.Infof("\nEval Stmt: %v\n", src)
	//starCode := syntax.FilePortion{
	//	Content:   []byte(src),
	//	FirstLine: 10,
	//	FirstCol:  5,
	//}

	//f, err := e.options.Parse(filename, src, 0)
	//if err != nil {
	//	glog.Errorf("Error parsing expr: %+v", err)
	//	return nil, err
	//}
	//
	//err := starlark.ExecREPLChunk(f, e.thread, prevState)

	value, err := starlark.EvalOptions(e.options, e.thread, filename, src, prevState)
	if err != nil {
		glog.Errorf("Error evaluating expr: %+v", err)
		return nil, err
	}

	// Print the global environment.
	glog.Infof("EvalResult GoType: %T, StarlarkType: %s, Value: %s\n", value, value.Type(), value)
	return value, nil
}

func (e *Evaluator) ExecPyStmt(filename string, stmt *ast.PyStmt, prevState starlark.StringDict) (bool, error) {

	glog.Infof("\nExec Stmt: %v\n", stmt)
	starCode := stmt.Code

	f, err := e.options.Parse(filename, starCode, 0)
	if err != nil {
		glog.Errorf("Error parsing expr: %+v", err)
		return false, err
	}

	err = starlark.ExecREPLChunk(f, e.thread, prevState)
	globals := prevState
	//globals, err := starlark.ExecFileOptions(e.options, e.thread, filename, starCode, prevState)
	if err != nil {
		glog.Errorf("Error executing stmt: %+v", err)
		return false, err
	}

	// Print the global environment.
	glog.Infof("Globals:")
	for _, name := range globals.Keys() {
		v := globals[name]
		glog.Infof("%s (%s) = %s\n", name, v.Type(), v.String())

		prevState[name] = v
	}
	return true, nil
}
