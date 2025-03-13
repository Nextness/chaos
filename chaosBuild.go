package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"
)

var cmd bytes.Buffer

func runCommand(command string) error {
	cmd.WriteString(command)
	defer cmd.Reset()
	if _, err := exec.Command("bash", "-c", cmd.String()).Output(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to execute '%s' - error %s\n", command, err.Error())
		return err
	}
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

	main := "./src/main.go"
	if err := runCommand(fmt.Sprintf("go build -o ./build/main %s ", main)); err != nil {
		os.Exit(1)
	}
}
