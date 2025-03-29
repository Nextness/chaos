<!--
    lastUpdate: 19/03/2025
    moduleName: random2.chaos
--->

# Documentation for Random2

## Context Description

Some information about context, and shit... *more information and details, but using bold text*

```chaos
type Context struct {
    allocator Memory := .{
        calloc Calloc = calloc;
        alloc Alloc = alloc;
        free Free = free;
        realloc Realloc = realloc;
        memset Memset = memset;
    };
    logger Logger? = null;
}
```

More information about the context and the fields + how to use


### Examples on Cotext

More information about example can be found in [random2 examples](./../example/random2.chaos.examples)
