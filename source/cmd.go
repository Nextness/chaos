package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type CompilationConfig struct {
	filePath          string
	enableDebugLogger bool
}

type CmdArguments struct {
	Arguments      []string
	CountArguments int
}

func InitCmdArguments() CmdArguments {
	return CmdArguments{
		Arguments:      os.Args[1:],
		CountArguments: len(os.Args[1:]),
	}
}

func (cmda *CmdArguments) CurrentArgument() string {
	if cmda.CountArguments > 0 {
		return cmda.Arguments[0]
	}
	return ""
}

func (cmda *CmdArguments) ConsumeArgument(position int, consumeIterations int) {
	for i := 0; i < consumeIterations; i++ {
		cmda.Arguments = append(cmda.Arguments[:position], cmda.Arguments[position+1:]...)
		cmda.CountArguments--
	}
}

func (cmda *CmdArguments) MatchCurrentArgument(arguments []string) bool {
	for _, tmp := range arguments {
		if tmp == cmda.CurrentArgument() {
			return true
		}
	}
	return false
}

func (cc *CompilationConfig) RunCmdArguments() error {
	argsWithoutProgram := InitCmdArguments()
	if argsWithoutProgram.CountArguments == 0 {
		fmt.Printf("ERROR: No arguments were provided\n")
		return errors.New("no arguments were provided")
	}

	for argsWithoutProgram.CountArguments != 0 {
		if argsWithoutProgram.MatchCurrentArgument([]string{"--file_path", "-f"}) {
			argsWithoutProgram.ConsumeArgument(0, 1)
			cc.filePath, _ = filepath.Abs(argsWithoutProgram.CurrentArgument())
			// TODO: Make the else part of this scope
		} else if argsWithoutProgram.MatchCurrentArgument([]string{"--enable_debug", "-D"}) {
			fmt.Printf("COMPILER CONFIG: Debug Information Enabled\n")
			cc.enableDebugLogger = true
		} else {
			fmt.Printf("ERROR: Missing argument after %s\n", argsWithoutProgram.CurrentArgument())
			return errors.New("missing argument")
		}
		argsWithoutProgram.ConsumeArgument(0, 1)
	}
	return nil
}
