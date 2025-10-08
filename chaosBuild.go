package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"
)

func runCommand(command string) error {
	cmdBuffer := bytes.Buffer{}
	cmdBuffer.WriteString(command)
	defer cmdBuffer.Reset()

	cmd := exec.Command("bash", "-c", cmdBuffer.String())
	out, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Fprintf(
			os.Stderr,
			"Failed to execute '%s' - error %s:\n\n%s\n",
			command,
			err.Error(),
			string(out),
		)
		return err
	}
	fmt.Printf("%s\n", string(out))
	return nil
}

func touchFile(name string) (*time.Time, error) {
	file, err := os.Stat(name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to touch file %s becuase of %s\n", name, err.Error())
		return nil, err
	}
	stat := file.Sys().(*syscall.Stat_t)
	ctime := time.Unix(int64(stat.Ctim.Sec), int64(stat.Ctim.Nsec))
	return &ctime, nil
}

func goRebuildYourSelfTek() {
	buildSourcePath := "./chaosBuild.go"
	timeSrc, err := touchFile(buildSourcePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to touch file %s because of %s\n", buildSourcePath, err.Error())
		os.Exit(1)
	}

	buildBinPath := "./chaosBuild"
	timeBin, err := touchFile(buildBinPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to touch file %s because of %s\n", buildSourcePath, err.Error())
		os.Exit(1)
	}

	if (*timeSrc).Compare(*timeBin) == 1 {
		if _, err := os.Stat(buildBinPath); err != nil {
			fmt.Printf("File %s not found. Skipping renaming\n", buildBinPath)
		} else {
			fmt.Printf("Renaming %s to %s.old\n", buildBinPath, buildBinPath)
			// Todo: why I need new line here?
			oldFile := fmt.Sprintf("%s.old\n", buildBinPath)
			os.Rename(buildBinPath, oldFile)
			os.Remove(oldFile)
		}

		if err := runCommand(fmt.Sprintf("go build %s", buildSourcePath)); err != nil {
			os.Exit(1)
		}
		goRebuildYourSelfTek()
	}
}

func MakeDirIfNotExist(dirName string) {
	var err error
	if _, err = os.Stat(dirName); err == nil {
		return
	}
	if err = os.Mkdir(dirName, 0766); err == nil {
		return
	}
	fmt.Fprintf(
		os.Stderr,
		"Filed to create directory %s because of %s\n", dirName, err.Error(),
	)
	os.Exit(1)
}

func main() {
	goRebuildYourSelfTek()
	MakeDirIfNotExist("./build")

	programName := os.Args[0]
	otherArgs := os.Args[1:]
	main := "./src/"

	var cmd string

	if 0 >= len(otherArgs) {
		cmd = fmt.Sprintf("go build -o ./build/main %s", main)
		if runCommand(cmd) != nil {
			os.Exit(1)
		}
	}

	length := len(otherArgs)
	count := 0
	for count < length {
		arg := otherArgs[count]
		defaultFileChaos := "./testing"
		switch arg {
		case "default":
			count++
			cmd = fmt.Sprintf("go build -o ./build/main %s && ./build/main %s.chaos", main, defaultFileChaos)
			if runCommand(cmd) != nil {
				os.Exit(1)
			}
			continue

		case "run":
			count++
			asmFile := fmt.Sprintf("%s.asm", defaultFileChaos)
			if _, err := touchFile(asmFile); err != nil {
				panic(fmt.Sprintf("%s not found - cannot compile", asmFile))
			}

			cmd = fmt.Sprintf("fasm %s testing", asmFile)
			if runCommand(cmd) != nil {
				os.Exit(1)
			}

			os.Chmod("testing", 0744)
			cmd = fmt.Sprint("./testing")
			if runCommand(cmd) != nil {
				os.Exit(1)
			}
			continue

		case "help":
			fmt.Printf("Help - Options:\n")
			fmt.Printf("    default\n")
			os.Exit(0)

		default:
			fmt.Fprintf(os.Stderr, "[ERROR] Unknown command '%s'\n", arg)
			fmt.Printf("Usage: %s [default]\n", programName)
			os.Exit(1)
		}
	}
}
