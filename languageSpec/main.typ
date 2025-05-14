#let numberingH(c)={
  return numbering(c.numbering,..counter(heading).at(c.location()))
}

#let currentH(level: 1)={
  let elems = query(selector(heading.where(level: level)).after(here()))

  if elems.len() != 0 and elems.first().location().page() == here().page() {
    return [#numberingH(elems.first()) #elems.first().body] 
  } else {
    elems = query(selector(heading.where(level: level)).before(here()))
    if elems.len() != 0 {
      return [#numberingH(elems.last()) #elems.last().body] 
    }
  }
  return ""
}

#set page(
    header: context {
        if counter(page).get().first() > 1 {
            align(right)[Chaos Programming Language Specification]
        }
    },
    numbering: "1",
)
#set par(justify: true)

#align(center, text(22pt)[*Chaos Programming Language Specification Definition*])
#grid(
    columns: (1fr),
    align(center)[
        Natan Colombo Menegasse \
        #link("mailto:natan.menegasse@hotmail.com")
    ]
)

= Simple Examples

== Entry Point and Hello World
```chaos
proc main
    expects argc U64, argv ...String
    returns U64
do
    call print
        with «Hello world!\n»
    return 0
end proc
```

== Identifiers and Assignments
```chaos
proc main
    expects argc U64, argv ...String
    returns U64
do
    // These are mutable variables
    let unitializedVarMut String
    unitializedVarMut = «String Literal»
    let initializedVarMut String = «String Litral»

    // These are constante variables
    let anotherUninitializedVarConst String
    anohterUnitializedVarConst := «String Literal»
    let initializedVarConst String := «String Literal»

    // Type inference
    let inferedMut = «String Literal»
    let inferedConst := «String Literal»
    return 0
end proc
```

== Procedures and Generics
```chaos
// Procedure defintion
proc someCustomProcedure
    expects input1 String, input2 Bool, input3 U64
    returns Void
do
    // perform operations
    call print
        with input1
    return
end proc

// Generic procedure definition
generic T for
proc someCustomGenericProcedure
    expects input1 T, input2 Bool, input3 U64
    returns Void
do
    // perform operations
    call print
        with input1
    return
end proc

proc main
    expects argc U64, argv ...String
    returns U64
do
    call someCustomProcedure
        with «Hello from Chaos», true, 1
    call someCustomProcedure inserts String
        with «Hello from Chaos», true, 1
    return 0
end proc
```

== Structs and Generics
```chaos
proc main
    expects argc U64, argv ...String
    returns U64
do
    // Struct definition
    type CustomStruct struct
        field1 String,
        field2 String = «default value»,
        field3 Bool   := true,
    end type
    let customStruct1 CustomStruct :=
        new with
            field1 = «some string»,
            field2 = «another value»,
        end new
    let customStruct2 :=
        new CustomStruct with
            field1 = «some string»,
            field2 = «another value»,
        end new

    // Generic struct definition
    generic T for
    type AnotherCustomType struct
        field1 T,
        field2 String = «default value»,
        field3 Bool   := true,
    end type
    let anotherCustomType1 AnotherCustomType :=
        new inserts String with
            field1 = «some string»,
            field2 = «another value»,
        end new
    let anotherCustomType2 :=
        new AnotherCustomType inserts String with
            field1 = «some string»,
            field2 = «another value»,
        end new
end proc
```

== Methods and Generics

= Advanced Examples
