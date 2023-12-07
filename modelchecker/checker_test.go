package modelchecker

import (
	"fizz/ast"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.starlark.net/starlark"
	"google.golang.org/protobuf/encoding/protojson"
	"testing"
)

func TestExecInit(t *testing.T) {
	astJson := `
    {
        "variables": [
            {
              "name": "MAX_ELEMENTS",
              "expression": "5"
            },
            {
              "name": "elements",
              "expression": "set()"
            },
            {
              "name": "count",
              "expression": "0"
            }
        ]
    }
    `
	checker := NewModelChecker("test")
	f := &ast.File{}
	err := protojson.Unmarshal([]byte(astJson), f)
	require.Nil(t, err)

	vars, err := checker.ExecInit(f.Variables)
	require.Nil(t, err)
	require.NotNil(t, vars)
	assert.Len(t, vars, 3)
	assert.Equal(t, "int", vars["MAX_ELEMENTS"].Type())
	assert.Equal(t, "5", vars["MAX_ELEMENTS"].String())
	assert.Equal(t, "int", vars["count"].Type())
	assert.Equal(t, "0", vars["count"].String())
	assert.Equal(t, "set", vars["elements"].Type())
	assert.Equal(t, "set([])", vars["elements"].String())
}

func TestExecAnyStmt(t *testing.T) {
	astJson := `{
		"loop_vars": ["e"],
		"py_expr": "range(0,MAX_ELEMENTS)",
		"block": {
			"stmts": [
			  {
				"pyStmt": {
				  "code": "elements = elements | set([e])"
				}
			  },
			  {
				"pyStmt": {
				  "code": "count = count + 1"
				}
			  }
			]
		}
	}
	`
	checker := NewModelChecker("test")
	anystmt := &ast.AnyStmt{}
	err := protojson.Unmarshal([]byte(astJson), anystmt)
	require.Nil(t, err)
	_ = checker
}

func TestExecIfStmt(t *testing.T) {
	astJson := `
    {
        "branches": [
            {
                "condition": "step == 'add'",
                "block": {
                    "stmts": [
                      {
                        "pyStmt": {
                          "code": "elements = elements | set([e])"
                        }
                      },
                      {
                        "pyStmt": {
                          "code": "count = count + 1"
                        }
                      }
                    ]
                }
            },
            {
                "condition": "step == 'remove'",
                "block": {
                    "stmts": [
                      {
                        "pyStmt": {
                          "code": "elements = elements - set([e])"
                        }
                      },
                      {
                        "pyStmt": {
                          "code": "count = count - 1"
                        }
                      }
                    ]
                }
            }
        ]
    }
    `
	checker := NewModelChecker("test")
	ifstmt := &ast.IfStmt{}
	err := protojson.Unmarshal([]byte(astJson), ifstmt)
	require.Nil(t, err)

	t.Run("then", func(t *testing.T) {
		vars := starlark.StringDict{}
		vars["count"] = starlark.MakeInt(5)
		vars["elements"] = starlark.NewSet(3)
		vars["e"] = starlark.String("a")
		vars["step"] = starlark.String("add")
		valid := checker.ExecIfStmt("then.fizz", ifstmt, vars)
		assert.True(t, valid)
		assert.Equal(t, "int", vars["count"].Type())
		assert.Equal(t, "6", vars["count"].String())
		assert.Equal(t, "set", vars["elements"].Type())
		assert.Equal(t, "set([\"a\"])", vars["elements"].String())
	})
	t.Run("elif", func(t *testing.T) {
		vars := starlark.StringDict{}
		vars["count"] = starlark.MakeInt(5)
		vars["elements"] = starlark.NewSet(3)
		vars["e"] = starlark.String("a")
		vars["step"] = starlark.String("remove")
		valid := checker.ExecIfStmt("elif.fizz", ifstmt, vars)
		assert.True(t, valid)
		assert.Equal(t, "int", vars["count"].Type())
		assert.Equal(t, "4", vars["count"].String())
		assert.Equal(t, "set", vars["elements"].Type())
		assert.Equal(t, "set([])", vars["elements"].String())
	})
	t.Run("else", func(t *testing.T) {
		vars := starlark.StringDict{}
		vars["count"] = starlark.MakeInt(5)
		vars["elements"] = starlark.NewSet(3)
		vars["e"] = starlark.String("a")
		vars["step"] = starlark.String("")
		valid := checker.ExecIfStmt("else.fizz", ifstmt, vars)
		assert.False(t, valid)
		assert.Equal(t, "int", vars["count"].Type())
		assert.Equal(t, "5", vars["count"].String())
		assert.Equal(t, "set", vars["elements"].Type())
		assert.Equal(t, "set([])", vars["elements"].String())
	})
}

func TestExecBlock(t *testing.T) {
	astJson := `
    {
        "stmts": [
          {
            "pyStmt": {
              "code": "elements = elements | set([e])"
            }
          },
          {
            "pyStmt": {
              "code": "count = count + 1"
            }
          }
        ]
    }
    `
	checker := NewModelChecker("test")
	b := &ast.Block{}

	t.Run("empty_block", func(t *testing.T) {
		vars := starlark.StringDict{}
		valid, err := checker.ExecBlock("name.fizz", b, vars)
		require.Nil(t, err)
		assert.False(t, valid)
	})
	t.Run("with_simple_stmts", func(t *testing.T) {
		vars := starlark.StringDict{}
		err := protojson.Unmarshal([]byte(astJson), b)
		require.Nil(t, err)

		vars["count"] = starlark.MakeInt(5)
		vars["elements"] = starlark.NewSet(3)
		vars["e"] = starlark.String("a")
		valid, err := checker.ExecBlock("name.fizz", b, vars)
		require.Nil(t, err)
		assert.True(t, valid)
		assert.Equal(t, "int", vars["count"].Type())
		assert.Equal(t, "6", vars["count"].String())
		assert.Equal(t, "set", vars["elements"].Type())
		assert.Equal(t, "set([\"a\"])", vars["elements"].String())
	})
	t.Run("with_nested_block", func(t *testing.T) {
		vars := starlark.StringDict{}
		b2 := &ast.Block{}
		err := protojson.Unmarshal([]byte(astJson), b)
		require.Nil(t, err)
		err = protojson.Unmarshal([]byte(astJson), b2)
		require.Nil(t, err)
		b.Stmts = append(b.Stmts, &ast.Statement{Block: b2})
		vars["count"] = starlark.MakeInt(5)
		vars["elements"] = starlark.NewSet(3)
		vars["e"] = starlark.String("a")
		valid, err := checker.ExecBlock("name.fizz", b, vars)
		require.Nil(t, err)
		assert.True(t, valid)
		assert.Equal(t, "int", vars["count"].Type())
		assert.Equal(t, "7", vars["count"].String())
		assert.Equal(t, "set", vars["elements"].Type())
		assert.Equal(t, "set([\"a\"])", vars["elements"].String())
	})

}
