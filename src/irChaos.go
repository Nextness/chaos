package main

// import (
// 	// "bytes"
// 	"fmt"
// )
//
// type Architecture = int
//
// type IrInstructions interface {
// 	toString() string
// 	toAsm(arch Architecture)
// }
//
// type ExitInstruction struct {
// 	name   string
// 	status int
// 	msg    string
// }
//
// type AssignmentInstruction struct {
// 	lhs string
// 	rhs string
// 	typ string
// }
//
// type Details struct {
// 	symbols map[string]string
// 	types   map[string]string
// 	procs   map[string]string
// }
//
// func (ai AssignmentInstruction) toAsm(arc Architecture) {
// 	assert(false, "AssignmentIntrusction.asAsim() not implemented")
// 	return
// }
//
// func (ai AssignmentInstruction) toString() string {
// 	return fmt.Sprintf("%s %s := %s;", ai.lhs, ai.typ, ai.rhs)
// }
//
// func (ei ExitInstruction) toAsm(arc Architecture) {
// 	assert(false, "ExitInstruct.asAsm() not implemented")
// }
//
// func (ei ExitInstruction) toString() string {
// 	if ei.msg == "" {
// 		return fmt.Sprintf("call %s %d;", ei.name, ei.status)
// 	}
// 	return fmt.Sprintf(
// 		"call %s %d '%s';",
// 		ei.name, ei.status, ei.msg,
// 	)
// }
//
// type IrState struct {
// 	details Details
// 	data    []IrInstructions
// 	count   int
// 	cursor  int
// }
//
// // sym0 someVariable;
// // sym1 something;
// // sym2 something1;
// //
// // typ0 String;
// // typ1 U64;
// //
// // sym1 typ1 := 1;
// // sym2 typ1 := sym1;
// //
// // $EXIT_SYSCALL := 60;
// //
// // exit :=
// //     param sym0 I32
// //     & param sym1 ?String undefined
// //     & isundef sym1 goto lab0
// //     & call print sym1
// //     & lab0:
// //     & syscall $EXIT_SYSCALL sym0;
// //
// // call exit 1;
//
// func irChaos(ls *LexerState) *IrState {
// 	is := IrState{
// 		details: Details{
// 			symbols: map[string]string{},
// 			types:   map[string]string{},
// 			procs:   map[string]string{},
// 		},
// 	}
// 	symbolsCount := 0
// 	typesCount := 0
// 	for ls.cursor < ls.count {
// 		entry := ls.data[ls.cursor]
// 		switch v := entry.(type) {
// 		case *NodeIdentifier:
// 			// Stora type information and keep track of it
// 			typ, ok := is.details.types[v.tpe]
// 			if !ok {
// 				typ = fmt.Sprintf("typ%d", typesCount)
// 				is.details.types[v.tpe] = typ
// 				typesCount++
// 			}
//
// 			// Store variable symbol and keep track of it
// 			sym, ok := is.details.symbols[v.sym]
// 			if !ok {
// 				sym = fmt.Sprintf("sym%d", symbolsCount)
// 				is.details.symbols[v.sym] = sym
// 				symbolsCount++
// 			}
//
// 			if v.val == "" {
// 				v.val = "undefined"
// 			}
//
// 			switch val := v.val.(type) {
// 			case string:
// 				if result, ok := is.details.symbols[val]; ok {
// 					val = result
// 				}
//
// 				ai := AssignmentInstruction{
// 					lhs: sym,
// 					rhs: val,
// 					typ: typ,
// 				}
// 				is.data = append(is.data, ai)
// 			case NodeBinOp:
// 				ai := AssignmentInstruction{
// 					lhs: sym,
// 					rhs: fmt.Sprintf("%s %s %s", val.lhs, val.op, val.rhs),
// 					typ: typ,
// 				}
// 				is.data = append(is.data, ai)
// 			}
// 			ls.cursor++
// 			continue
// 		case *NodeExit:
// 			// TODO: Include this as an intrisict and not as part of calling exit
// 			if _, ok := is.details.procs["exit"]; !ok {
// 				is.details.procs["exit"] = "exit :=\n" +
// 					"        param stt AnyNumber\n" +
// 					"        & param msg ?String undefined\n" +
// 					"        & isundef msg goto lab0\n" +
// 					"        & call print msg\n" +
// 					"        & lab0:\n" +
// 					"        & syscall 60 stt;"
// 			}
//
// 			sym, ok := is.details.symbols[fmt.Sprintf("%d", v.val)]
// 			if !ok {
// 				sym = fmt.Sprintf("sym%d", symbolsCount)
// 				is.details.symbols["literal0"] = sym
// 				symbolsCount++
// 			}
//
// 			ei := ExitInstruction{
// 				name:   "exit",
// 				status: v.val,
// 				msg:    v.msg,
// 			}
// 			is.data = append(is.data, ei)
// 			ls.cursor++
// 			continue
// 		default:
// 			fmt.Printf("IR Not implemented for %T\n", v)
// 			panic("not implemented")
// 		}
// 	}
// 	return &is
// }
//
// func irChaosToString(is *IrState) {
// 	fmt.Print("symbols\n")
// 	for idx, val := range is.details.symbols {
// 		fmt.Printf("    %s := %s;\n", val, idx)
// 	}
// 	fmt.Print("types\n")
// 	for idx, val := range is.details.types {
// 		fmt.Printf("    %s := %s;\n", val, idx)
// 	}
// 	fmt.Print("pocs\n")
// 	for _, val := range is.details.procs {
// 		fmt.Printf("    %s\n", val)
// 	}
// 	fmt.Print("main\n")
// 	for _, val := range is.data {
// 		fmt.Printf("    %v\n", val.toString())
// 	}
// }
