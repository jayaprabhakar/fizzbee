# fizzbee

A Formal specification language and model checker
to specify distributed systems.

# Docs
If you are familiar with [TLA+](https://lamport.azurewebsites.net/tla/tla.html), this would be a quick start
[From TLA+ to Fizz](https://github.com/jayaprabhakar/fizzbee/blob/main/docs/language_design_for_review.md)

# Run a model checker
For now, no prebuilt binaries. 
- Bazel: You need bazel installed to build. [Bazelisk](https://github.com/bazelbuild/bazelisk?tab=readme-ov-file#installation) is the recommended way to use bazel. Rename the binary to bazel and put it part of your PATH.
- gcc: This project uses protobuf. Bazel proto_library does not use precompiled protoc, and it builds from scratch. It requires g++ compiler. `sudo apt update; sudo apt install g++`


```
./fizz path_to_spec.fizz  
```
Example:
```
./fizz examples/tutorials/19-for-stmt-serial-check-again/ForLoop.fizz 
```


# Development

## Bazel build
To run all tests:

```
bazel test //...
```

To regenerate BUILD.bazel files,

```
bazel run //:gazelle
```

To add a new dependency,

```
bazel run //:gazelle -- update-repos github.com/your/repo
```
or
```
gazelle update-repos github.com/your/repo
```

When making grammar changes, run

```
antlr4 -Dlanguage=Python3 -visitor *.g4
```
and commit the py files.
TODO: Automate this using gen-rule, so the generated files are not required in the repository
