package modelchecker

const (
	ActionsWithMultipleBlocks = `
{
  "states": {
    "code": "MAX_ELEMENTS = 5\nelements = set()\ncount = 0"
  },
  "actions": [
    {
      "name": "FirstAction",
      "block": {
        "flow": "FLOW_ATOMIC",
        "stmts": [
          {
            "pyStmt": {
              "code": "a = a + 1"
            }   
          },
          {
            "pyStmt": {
              "code": "b = b + 2"
            }
          },
          {
            "block": {
              "flow": "FLOW_ATOMIC",
              "stmts": [
                {
                  "pyStmt": {
                    "code": "a = a + 1"
                  }   
                },
                {
                  "pyStmt": {
                    "code": "b = b + 2"
                  }
                }
              ]
            }
          },
          {
            "pyStmt": {
              "code": "a = a + 1"
            }   
          },
          {
            "pyStmt": {
              "code": "b = b + 2"
            }
          }
        ]
      }
    },
    {
      "name": "SecondAction",
      "block": {
        "flow": "FLOW_ONEOF",
        "stmts": [
          {
            "pyStmt": {
              "code": "a = a + 1"
            }   
          },
          {
            "pyStmt": {
              "code": "b = b + 2"
            }
          },
          {
            "block": {
              "flow": "FLOW_ONEOF",
              "stmts": [
                {
                  "pyStmt": {
                    "code": "a = a + 1"
                  }   
                },
                {
                  "pyStmt": {
                    "code": "b = b + 2"
                  }
                }
              ]
            }
          },
          {
            "pyStmt": {
              "code": "a = a + 1"
            }   
          },
          {
            "pyStmt": {
              "code": "b = b + 2"
            }
          }
        ]
      }
    },
    {
      "name": "ThirdAction",
      "block": {
        "flow": "FLOW_SERIAL",
        "stmts": [
          {
            "pyStmt": {
              "code": "a = a + 1"
            }   
          },
          {
            "pyStmt": {
              "code": "b = b + 2"
            }
          },
          {
            "block": {
              "flow": "FLOW_SERIAL",
              "stmts": [
                {
                  "pyStmt": {
                    "code": "a = a + 1"
                  }   
                },
                {
                  "pyStmt": {
                    "code": "b = b + 2"
                  }
                }
              ]
            }
          },
          {
            "pyStmt": {
              "code": "a = a + 1"
            }   
          },
          {
            "pyStmt": {
              "code": "b = b + 2"
            }
          }
        ]
      }
    },
    {
      "name": "ThirdAction",
      "block": {
        "flow": "FLOW_PARALLEL",
        "stmts": [
          {
            "pyStmt": {
              "code": "a = a + 1"
            }   
          },
          {
            "pyStmt": {
              "code": "b = b + 2"
            }
          },
          {
            "block": {
              "flow": "FLOW_PARALLEL",
              "stmts": [
                {
                  "pyStmt": {
                    "code": "a = a + 1"
                  }   
                },
                {
                  "pyStmt": {
                    "code": "b = b + 2"
                  }
                }
              ]
            }
          },
          {
            "pyStmt": {
              "code": "a = a + 1"
            }   
          },
          {
            "pyStmt": {
              "code": "b = b + 2"
            }
          }
        ]
      }
    }
  ]
}
`
)
