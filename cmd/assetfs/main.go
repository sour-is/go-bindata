package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
)

func isDebug(args []string) bool {
	flagset := flag.NewFlagSet("", flag.ContinueOnError)
	debug := flagset.Bool("debug", false, "")
	debugArgs := make([]string, 0)
	for _, arg := range args {
		if strings.HasPrefix(arg, "-debug") {
			debugArgs = append(debugArgs, arg)
		}
	}
	flagset.Parse(debugArgs)
	if debug == nil {
		return false
	}
	return *debug
}

func getBinDataFile() (*os.File, string, []string, error) {
	bindataArgs := make([]string, 0)
	outputLoc := "bindata.go"

	for i := 1; i < len(os.Args); i++ {
		if os.Args[i] == "-o" {
			outputLoc = os.Args[i+1]
			i++
		} else {
			bindataArgs = append(bindataArgs, os.Args[i])
		}
	}

	tempFile, err := ioutil.TempFile(os.TempDir(), "")
	if err != nil {
		return &os.File{}, "", nil, err
	}
	tempFile.Close()

	outputFile, err := os.Create(outputLoc)
	if err != nil {
		return &os.File{}, "", nil, err
	}

	bindataArgs = append([]string{"-o", tempFile.Name()}, bindataArgs...)
	return outputFile, tempFile.Name(), bindataArgs, nil
}

func main() {
	path, err := exec.LookPath("go")
	if err != nil {
		fmt.Println("Cannot find go executable in path")
		os.Exit(1)
	}
	out, inFile, args, err := getBinDataFile()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error: cannot create temporary file", err)
		os.Exit(1)
	}

	lis := append([]string{}, "run", "github.com/sour-is/go-assetfs/cmd/bindata")
	lis = append(lis, args...)

	cmd := exec.Command(path, lis...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "Error: go-bindata: ", err)
		os.Exit(1)
	}

	debug := isDebug(os.Args[1:])
	in, _ := os.Open(inFile)
	r := bufio.NewReader(in)
	done := false
	for line, isPrefix, err := r.ReadLine(); err == nil; line, isPrefix, err = r.ReadLine() {
		if !isPrefix {
			line = append(line, '\n')
		}
		if _, err := out.Write(line); err != nil {
			fmt.Fprintln(os.Stderr, "Cannot write to ", out.Name(), err)
			return
		}
		if !done && !isPrefix && bytes.HasPrefix(line, []byte("import (")) {
			if debug {
				fmt.Fprintln(out, "\t\"net/http\"")
			} else {
				fmt.Fprintln(out, "\t\"github.com/sour-is/go-assetfs/pkg/assetfs\"")
			}
			done = true
		}
	}
	in.Close()
	if err := os.Remove(in.Name()); err != nil {
		fmt.Fprintln(os.Stderr, "Cannot remove", in.Name(), err)
	}

	if debug {
		fmt.Fprintln(out, `
func assetFS() http.FileSystem {
	for k := range _bintree.Children {
		return http.Dir(k)
	}
	panic("unreachable")
}`)
	} else {
		fmt.Fprintln(out, `
func assetFS() *assetfs.AssetFS {
	assetInfo := func(path string) (os.FileInfo, error) {
		return os.Stat(path)
	}
	for k := range _bintree.Children {
		return &assetfs.AssetFS{Asset: Asset, AssetDir: AssetDir, AssetInfo: assetInfo, Prefix: k}
	}
	panic("unreachable")
}`)
	}

	// Close files BEFORE remove calls (don't use defer).
	out.Close()
}
