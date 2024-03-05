# FizzBee Quick Start for TLA+ Users

## Introduction
The meat of the code will be in Starlark language, a subset of Python. So expressions can be tried
with a typical Python REPL. But there are some additions suitable for model checking. 

## High level structure
Directory:
fizz.yaml file. It is a config file, for model checking


###  directory
- .fizz file: The main model specification file
- fizz.yaml (Optional): The model checking config file. Yaml representation for the protobuf defined in proto/statespace_options.proto
- performance config files (Optional): The performance config files. Yaml representation for the protobuf defined in proto/performance_model.proto

### fizz.yaml file
Example:
```yaml
options:
  maxActions: 10
  maxConcurrentActions: 2
  
actionOptions:
  YourActionName:
    maxActions: 1
```

### .fizz file
The main file that contains the specification. It is a text file with the extension .fizz.

The generic structure of the file is:

```
# init action

# invariants

# action1

# action2

# additional_fuctions 
```
## Actions
Actions are the main building blocks of the model. They are the steps that the model takes to reach a state.

### Action definition
```
[atomic|serial] action YourActionName
  # Almost python code
  a += 1
  b += 1
```
Note: Each python statement is executed independently, they are the basic building
blocks. The python statements themselves are not parsed or interpreted directly by fizzbee.


### Atomic actions:
In TLA+ actions are atomic. In Fizz we explicitly state the atomicity of the action.
Atomic actions, mean there is no yield points between the statements.

In the above example, a and b are incremented atomically. If they both started at the same value,
they will end up at the same value.

### Serial action
Here, after each statement, there will be a yield point.
In the above example, a and b are incremented serially. 
So, if a=0 and b=0 at the beginning, the possible next steps are:

- a=1, b=0
- a=1, b=1

## Block modifiers
Every block can have a block modifier.
The block modifiers are: `atomic`, `serial`, `parallel`, `oneof`
`atomic` and `serial` are already explained.

### Oneof `oneof`
`oneof` is equivalent to \/ in TLA+.
For example,
```
action IncrementAny:
  oneof:
    a += 1
    b += 1
```
Here, either a or b will be incremented. Not both.
So, if a=0 and b=0 at the beginning, the possible next steps are:

- a=1, b=0
- a=0, b=1

### Parallel `parallel`
`parallel` implies the statements can be executed concurrently. 
So, they can be executed in any order. And there can be yield points between them,
so other actions could be interleaved between them.

```
action IncrementAny:
  oneof:
    a += 1
    b += 1
```
So, if a=0 and b=0 at the beginning, the possible next steps are:

- a=1, b=1
- a=0, b=1
- a=1, b=0

Note: These can be nested. But `atomic` can only contain `atomic` or `oneof` blocks.

## init action

`Init` is just another action called once at the beginning of the model checking. 
It is used to initialize the state of the model.

```
atomic action Init:
  a = 0
  b = 0
```

Some examples use the older way of defining the init action. 
It is still supported, but it will be removed soon. The new ways are more expressive and flexible.
as it can support non-determinism in the Init itself. 
Init can lead to multiple Init states. But the old way cannot express that.

```
# Old way
init:
  a = 0
  b = 0
```
Example with non-determinism in Init:
```
action Init:
  # More common usecases will use `any` statements
  oneof:
    atomic:
        a = 0
        b = 10
    atomic:
        a = 10
        b = 0
```


## Functions
Note: Functions is not fully implemented yet. Specifically, no parameters yet :(
It will be implemented soon.

Functions are defined with `func` keyword. It is syntactically similar to actions.

```
func TossACoin:
  oneof:
    return 0
    return 1
```

## Control Flow

### If-else
Same as python: if-elif-else
```
if a > b:
  b += 1
else:
  a += 1
```

### While
Same as python: while. (Note: Python's else clause on while is not supported)
```
while a < 5:
  a += 1
```
If a is 10 at the beginning, a will be 15 at the end.

### For
Same as python: for. (Note: Python's else clause on for is not supported)
Similar to `\A` in TLA+
```
for i in range(5):
  a += 1
```
If a is 10 at the beginning, a will be 15 at the end.

### Any statement
`any` is a non-deterministic statement. It is similar to `oneof` but for loop.
Similar to `\E` in TLA+
```
any i in range(5):
  a += i
```
If a is 10 at the beginning, there are 5 possible next states.
with a being 10, 11, 12, 13, 14 at the end.

## Invariants/Assertions
Invariants are the properties that should hold true at every state of the model.

There are two ways to define invariants. For most practical purposes, you'll need the first way.

Note: `assert` is a keyword in Python. So, we use `assertion`. 

```
always assertion FirstInvariant:
  return a == b
  
always assertion SecondInvariant2:
  # it can have loops, if-else, etc.
  return some_boolean_expression
  
```
Another way for most simple cases.
```
invariant:
  # Here each statement is a separate invariant.
  always a == 10
  always a < 10
  always b < 10
```

### always
This is equivalent to `[]` in TLA+. For safety properties.

### always eventually
This is equivalent to `[]<>` in TLA+. For liveness properties.

### eventually always
This is equivalent to `<>[]` in TLA+. For liveness properties.

Note: at this time, we don't have a way to nest these temporal operators.
