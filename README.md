# Chaos Language

A language created for no particular reason other than try to include things I which langues I use daily had.

## How to build it?

```bash
$ make                  # build both binaries into ./build
$ make build/chaosc     # build the compiler CLI
$ make build/chaos-lsp  # build the language server
```

`make test` runs the Go test suite; `make vet` runs static analysis.

# Objectives

- [ ] Create enought of the language to self host and bootstrap


# How it works?

The `chaosc` CLI tokenizes and parses a `.chaos` file and reports diagnostics.
The `chaos-lsp` language server, wired into Neovim, surfaces diagnostics,
document symbols, semantic highlighting, go-to-definition, references, hover,
and completion for the supported language subset.

The hello-world example below is illustrative: it parses under the language
server's tolerant mode, but the strict CLI still rejects it and there is no
codegen yet.

```chaos
main :: #entry proc {
    print(«Chaotic hello\n»);
}

#import «fmt.chaos»;
```
