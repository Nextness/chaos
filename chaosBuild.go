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

func main() {
	goRebuildYourSelfTek()
	build_dir := "./build"
	if _, err := os.Stat(build_dir); err != nil {
		if err := os.Mkdir(build_dir, os.FileMode(0777)); err != nil {
			fmt.Fprintf(os.Stderr, "Filed to create directory %s because of %s\n", build_dir, err.Error())
			os.Exit(1)
		}
	}

	programName := os.Args[0]
	otherArgs := os.Args[1:]
	main := "./src/main.go"

	if 0 >= len(otherArgs) {
		if err := runCommand(fmt.Sprintf("go build -o ./build/main %s", main)); err != nil {
			os.Exit(1)
		}
	} else {
		for _, arg := range otherArgs {
			if arg == "build-and-run-default" {
				defaultFileChaos := "./example/assingment.chaos"
				if err := runCommand(fmt.Sprintf("go build -o ./build/main %s && ./build/main %s", main, defaultFileChaos)); err != nil {
					os.Exit(1)
				}
			} else {
				fmt.Fprintf(os.Stderr, "[ERROR] Unknown command '%s'\n", arg)
				fmt.Printf("Usage: %s [build-and-run-default]\n", programName)
				os.Exit(1)
			}
		}
	}

	os.Exit(0)
}
