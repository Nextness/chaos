package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type CompilationConfig struct {
	filePath string
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

func RunCmdArguments() (CompilationConfig, error) {
	argsWithoutProgram := InitCmdArguments()
	if argsWithoutProgram.CountArguments == 0 {
		fmt.Printf("ERROR: No arguments were provided\n")
		return CompilationConfig{}, errors.New("no arguments were provided")
	}

	compilationConfig := CompilationConfig{}
	for argsWithoutProgram.CountArguments != 0 {
		if argsWithoutProgram.MatchCurrentArgument([]string{"--file_path", "-f"}) {
			argsWithoutProgram.ConsumeArgument(0, 1)
			compilationConfig.filePath, _ = filepath.Abs(argsWithoutProgram.CurrentArgument())
			// TODO: Make the else part of this scope
		} else {
			fmt.Printf("ERROR: Missing argument after %s\n", argsWithoutProgram.CurrentArgument())
			return CompilationConfig{}, errors.New("missing argument")
		}
		argsWithoutProgram.ConsumeArgument(0, 1)
	}
	return compilationConfig, nil
}
