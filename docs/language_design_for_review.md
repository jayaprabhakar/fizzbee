
# TLA+ to Fizz



## Next state or Primed variable
TlA+ uses primed variables to indicate the next state variable.
For example: `count' = count + 1`. If a programmer never saw
TLA+ before, they won't know what it means. We can make it
more explicit by naming them explictly. For example, 
`this` for the current state and `next` for the next state.
Then, the code will be `next.count = this.count + 1`.

Even though TLA+ language says `=` operator is commutative, TLC
does not treat it that way. So, the next state must always be on the left.

Then, can we infer it our self? Do we really need next/this separation
for the common case?

Then, this will simplify to `count = count + 1`. Every programmer
will get this without any confusion.


## State Variables
Since every variable has to be initialized anyway, do we need separate
declarations of variables and a separate init state? Almost, all
Specs name the init action as `Init`, can we make that a convention?

Since TLA+ is also untyped, and our target language python is also untyped,
we could get rid of explicit declaration and require initialization.

The spec will have a section for state variables.

```
state:
  count = 0
  list = []
```

## Non atomic actions
In TLA+, every action is atomic. In PlusCal, we can make the statements
atomic or serial by assigning labels to block of statements.

But, with TLA+ simulating error cases becomes harder as we need to
explicitly define the error cases, which is mostly not true in the distributed
systems world.

These are my observations designing distributed systems:

* Murphy's law is real. Anything that can go wrong will go wrong.
* Many operations in distributed systems are non-atomic
* Most common way of implementing steps is sequential. For example
  * Write a message to a DB
  * Publish an event
  * In practice, any of these can fail.
* Some operations can be parallel. They can be
  * Explicit like make multiple IO operations in parallel
  * Implicit like publish an event, and two separate services listen to the events
    and process them or update the DB.
* Obviously, some operations are atomic.

For example: we need to model an object counter, where an object will be written
to some persistent object store, and a counter will be update in a different key value store.

TLA+:
```
Add(b) == 
  /\ blobs' = blobs \union {b}
  /\ count' = count + 1
```
With this model, count will actually always match the blobs count. Unfortunately, 
in real system, the count update could also fail. So, unless the programmer
actually thinks about this case, this will lead to buggy implementation.

To model, the failure case, they have to explicitly write,
```
Add(b) == 
  \/ /\ blobs' = blobs \union {b}
     /\ count' = count + 1
  \/ /\ blobs' = blobs \union {b}
     /\ UNCHANGED <<count>>
```
In this case, blobs are written first and then the count is updated. And the count could fail.

Alternately, since this is the most common case, in our new language we will make serial order the default.

In Fizz:
```
add(b):
  # A block of statements can be labeled as serial,
  # then we will automatically test the cases where only some
  # steps succeeded.
  # serial is the default, so this can be ignored.
  serial:
    blobs.add(b)
    count = count + 1
    
add_alternative(b):
  # serial is the default
  blobs.add(a)
  count = count + 1
```
What if we actually want this to be atomic? Like, we use some
transactions on the same database. Then, label the block atomic.

```
add(b):
  atomic:
    blobs.add(b)
    count = count + 1
```
In this case, if blobs are updated, count will also be updated.

Then parallel. 
```
add(b):
  parallel:
    blobs.add(b)
    count = count + 1
```
In this case, there is a chance count updated successfully, 
but updating blobs failed since these steps all happened in parallel.

TLA+ equivalent of it is,
```
Add(b) == 
  \/ /\ blobs' = blobs \union {b}
     /\ count' = count + 1
  \/ /\ blobs' = blobs \union {b}
     /\ UNCHANGED <<count>>
  \/ /\ count' = count + 1
     /\ UNCHANGED <<blobs>>
```

Oneof:
TLA+ conjunct (`/\`) is equivalent to atomic operation. Similarly, the equivalent,
of disjunct (`\/`) is oneof

The same parallel Add case can be coded in Fizz as,
```
add(b):
  oneof:
    atomic:
      blobs.add(b)
      count = count + 1
    atomic:
      blobs.add(b)
    atomic:
      count = count + 1      
      
# since there is only a single statement, atomic can be ignored.
add_alternative(b):
  oneof:
    atomic:
      blobs.add(b)
      count = count + 1
    blobs.add(b)
    count = count + 1   
```

## UNCHANGED not required
Unlike TLA+, we will assume, if there is no next state transition,
then they are not changed.

## \A and \E
The idiomatic python for the universal qualifier `\A` is `for`.
TLA+
```
\A n \in nodes:
  n' = [n EXCEPT !.status = 'done']
```
Fizz
```
for n in nodes:
  n.status = 'done'
```

This is clearly intuitive idiomatic python, that even non-python
programmers would instantly understand.

There is no equivalent for the existential qualifier `\E` in python.
So, we are introducing an equivalent `any` that will be syntactically
similar to `for`. The primary behavioral difference is, if there are
5 elements in the list/set, `any` will create 5 different branches,
with 1 element selected in each branch. 

TLA+
```
\E r \in records:
  records' = records \ {r}
```
Fizz
```
any r in records:
  # alternately records.remove(r)
  records = records - {r}
```

## Implicit Spec and next state actions
All actions start with a keyword `action` similar to `def` for python functions. So, we don't need a separate
Next state or spec.

> Should we remove the action keyword altogether?

```
action Add:
  # ...
  
action Remove:
  # ...  
```
Each of these will be part of the next state actions.

## Fairness
> What would be a good keywords to specify to differentiate
> between strong and weak fairness.

For now, in each action, you can add a modifier. `weak` or `strong`

```
action FirstAction:
  # ... unfair action

weak<var1, var2> SecondAction
  # action with weak fairness
  # WF_<<var1,var2>>(Next)
  # like fair process of PlusCal 
    
weak action ThirdAction:
  # action with weak fairness, but all 
  # declared state variables
  # like fair process of PlusCal
  
  
strong action ForthAction:
  # ... action with strong fairness
  # similar to fair+ of PlusCal
  
```

> Note: Alternate option I was considering is to have a
> keyword fair for weak fairness, and parameterize with
> `fair<strong=true, var1, var2...>` Request for feedback.
> fair keyword is better than saying weak. But `fair+` for strong
> doesn't sound right, and don't want to introduce
> `strong_fair`, `weak_fair`.
> Yet another alternative, parameterize action.
> `action<fairness=strong, var1, var2,...>`
> or `action<fairness<strong<var1,var2>>`
> What do you all prefer and any other alternatives?


## Python functions
Most builtin python functions should be available to use,
and we will be able to add additional functions as needed.

For the initial implementation, Fizz will use Starlark language
instead of standard python, so many modules would not be
able to be imported. We might change this to standard python
eventually, if this is a huge limitation. But Starlark
provide significant advantages in terms of security and being
hermetic.

For now, this is not a major limitation since most important
python functions are available, and we are building a repository
of reusable libraries.

## Imports
Imports will follow Go language style syntax.
In addition, they should be able to import other fizz files,
or other starlark files. Any starlark/python function will
be treated as `atomic`.

## Roles
Roles are a way to organize the components of a large distributed 
system similar to a `class` in an object-oriented programming but
describes a higher level system. A role could be a microservice,
a database, a distributed cache or subcomponent of a monolith or
even a virtual/logical subservice. 

For example: in two-phase commit,
the coordinator (transaction manager) and the participants (resource
manager) are different roles. In practice, each participant could
act as a coordinator and they can be part of the same service/process.

As a convenience, every module is actually a role or specifically role type.
The module's constants (model parameters) is the same as parameters of a role.
The module's state is the role's state, actions defined within the role
gets called the same way.

In a new module or .fizz file, you can simply initialize the state with
the roles needed. The role's actions would be enabled.

Within the same module or .fizz file, you can define the role with
keyword `role`.

```
# Coordinator
role TransactionManager {
  state:
    # state variables
  
  # Action definitions
}
# Participant
role ResourceManager(rm) {
  state:
    # state variables
  
  # Action definitions
}

constants:
  RM

state:
  resMgrs = {ResourceManager(rm) for rm in RM}
  transMgr = TransactionManager()

```


## Channel (Messaging channel)
* Blocking/NonBlocking
* Delivery (atmost once, at least once, exactly once)
* Ordering (unordered, pairwise, ordered)

### Default
#### Intra-role call:
Since calls between roles are usually in-memory call, the default
will be reliable. `blocking exactlyonce ordered`
#### Inter-role call:
Inter role calls are usually some kind of message passing - 
 either blocking (RPC) operation or non-blocking (Message Queues)
so these are unreliable. We will default to rpc semantics.
`blocking atmostonce unordered`




# Alternative considered
## Separate Inputs, Guard clauses and post actions
TLA+ does not separate guard clauses (preconditions) or inputs
separately. Everything is just another state assertion.
Many other formal languages separate preconditions, state 
transitions, and some even separate inputs (For example: Event-B).

Separating precondition vs state transition has a significant
advantage for readability. And it also makes the implementation
a lot simpler.

```
# Option: NOT PLANNED for now, unless users prefer this.

# TLA+ equivalent of
# Count == count < 10 /\ count' = count + 1

action Count:
  pre:
    count < 10
  next:
    count = count + 1

# Current proposal
action Count:
  if count < 10:
    count = count + 1

```

The drawback is, it reduces expressiveness for many cases. Especially,
those actions that use Universal (`\A`) and Existential (`\E`)
qualifiers.

For example: Notify all subscribers in pending state to done.
TLA+
```
NotifySubscribers ==
  \A s \in subscribers:
    /\ status[s] == "pending"
    /\ status' = [status EXCEPT ![s] = "done"]

```
Fizz
```
action NotifySubscribers:
  for s in subscribers:
    if status[s] == 'pending'
      status[s] = 'done'
```
In this case, action NotifySubscribers is not ENABLED if no subscribers are in pending state.
However, this would become a lot more verbose to separate as precondition.
```
action NotifySubscribers:
  pre:
    len({sub,status for sub,status in subscribers if status=="pending"})>0
  next:
    for s in subscribers:
      if status[s] == 'pending'
        status[s] = 'done'
```
Note: As the implementation of model checker is a lot simpler if we can separate
guard clauses.

Also, not separating preconditions is actually how programmers
typically program since no major programming language does it.
For cases where preconditions seem natural, they could simply handle
it with if block at the top.

# Inputs to actions
Separating inputs is actually very intuitive for programmers.
This is mostly specified in the RPC specification. 

For example: To remove a document from stored documents.
TLA+
```
Remove == 
  \E d in documents:
    documents' = documents \ {d}
```
Fizz
```
# Fizz
action Remove:
  any d in documents:
    documents.remove(d)

```
Alternatives being considered:
```
# option1: Similar to event-b
action Remove:
  input:
    any d in documents:
  next:
    documents.remove(d)

# option2 
action Remove(any d in documents):
  documents.remove(d)
  
# option3 
# read as d such that d is in documents.
action Remove(d in documents):
  documents.remove(d)
```
An example combining all these actions:

```
# option 1:
action Add:
  input: 
    any d in ALL_DOCUMENTS # Model value; set of all documents
  pre:
    d not in documents
  next:
    documents.add(d)

# option 2: separate input no separate guard clause
# This makes the grammer a bit harder
action Add(any d in ALL_DOCUMENTS):
    if d not in documents:
      documents.add(d)
```
