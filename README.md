# Chaos Language

A language created for no particular reason other than try to include things I which langues I use daily had.

## How to build it?

```bash
$ go build chaosBuild.go # Bootstrap the build binary
$ ./chaosBuild           # Run the build binary
```

# Objectives

- [ ] Create enought of the language to self host and bootstrap


# How it works?

This is the basic. For more examples look at *examples*...

```chaos
// ./build/main ./<file_name>.chaos
// Hello world
```

```chaos
main :: #entry proc {
    print(«Chaotic hello\n»);
}

#import «fmt.chaos»;
```

