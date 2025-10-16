// This code was basically copied from https://github.com/tsoding/nob.h
package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
	"time"
)

func runCommand(command string) error {
	cmdBuffer := bytes.Buffer{}
	cmdBuffer.WriteString(command)
	defer cmdBuffer.Reset()

	if runtime.GOOS == "linux" {
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
	}

	return nil
}

func touchFile(name string) (*time.Time, error) {
	if runtime.GOOS == "linux" {
		file, err := os.Stat(name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to touch file %s becuase of %s\n", name, err.Error())
			return nil, err
		}
		stat := file.Sys().(*syscall.Stat_t)
		ctime := time.Unix(int64(stat.Ctim.Sec), int64(stat.Ctim.Nsec))
		return &ctime, nil
	}

	return nil, errors.New("OS Not supported")
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

const DEBUG = true

func main() {
	goRebuildYourSelfTek()

	defaultCompilation := flag.Bool("default", false, "Running default file for development purposes (requires DEBUG=true)")
	filename := flag.String("c", "", "Chaos file to be compiled")
	runChaosBin := flag.Bool("r", false, "Run binary after compilation")
	runTests := flag.Bool("test", false, "Run unit tests")

	flag.Parse()

	MakeDirIfNotExist("./build")

	srcDir := "./src/"
	testsDir := "./tests/"
	compilerLocation := "./build/chaosc"

	cmd := fmt.Sprintf("go build -o %s %s", compilerLocation, srcDir)
	if runCommand(cmd) != nil {
		os.Exit(1)
	}

	if *runTests {
		files, _ := os.ReadDir(testsDir)
		for idx, file := range files {
			fmt.Print("-----------------------------------------------------------------------\n")
			iteration := idx + 1
			filename := file.Name()
			fmt.Printf("%03d: Running the file %s\n", iteration, filename)
			cmd := fmt.Sprintf("%s %s%s", compilerLocation, testsDir, filename)
			if runCommand(cmd) != nil {
				fmt.Fprintf(os.Stderr, "[ERROR] Failed to run %s during unit tests\n", filename)
				continue
			}
			fmt.Printf("Success running file %s\n", filename)
		}
		fmt.Print("-----------------------------------------------------------------------\n")
		os.Exit(0)
	}

	defaultFile := ""
	if *defaultCompilation && DEBUG {
		defaultFile = "main.chaos"
		cmd := fmt.Sprintf("%s %s", compilerLocation, defaultFile)
		if runCommand(cmd) != nil {
			os.Exit(1)
		}
	}

	if *filename != "" {
		cmd := fmt.Sprintf("%s %s", compilerLocation, *filename)
		if runCommand(cmd) != nil {
			os.Exit(1)
		}
	}

	if *runChaosBin && (*filename != "" || defaultFile != "") {
		file := strings.TrimSuffix(*filename, ".chaos")
		fasmFile := fmt.Sprintf("%s.asm", file)
		if _, err := touchFile(fasmFile); err != nil {
			fmt.Fprintf(os.Stderr, "[ERROR] Did not find the fasm file '%s'. Make sure it exists.\n", fasmFile)
			os.Exit(1)
		}

		cmd := fmt.Sprintf("fasm %s testing", fasmFile)
		if runCommand(cmd) != nil {
			fmt.Fprintf(os.Stderr, "[ERROR] Failed to compile '%s'.\n", fasmFile)
			os.Exit(1)
		}

		os.Chmod("testing", 0744)
		cmd = fmt.Sprintf("%s", file)
		if runCommand(cmd) != nil {
			fmt.Fprintf(os.Stderr, "[ERROR] Failed to run '%s'.\n", file)
			os.Exit(1)
		}
	}
}
