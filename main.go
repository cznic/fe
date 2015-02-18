// Copyright 2015 The fe Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Command fe (Fix Examples) corrects errors produced by examples while running
// go test.
//
// For example
//
//	$ go test | fe
//
// or perhaps
//
//	$ go test -run ^Example | fe
//
// will parse the output of go test for the package in current directory.  If
// there are errors in examples[0] (got != want), the actual output (got) is
// written back to the example in its respective *_test.go file, assuming it's
// properly gofmt'ed.
//
//
// Any, potentially erroneous, example output is considered valid and
// incorporated back into the example.  No backups are made, use carefully.
//
// Named files are not supported by design.
//
//  [0]: http://golang.org/pkg/testing/#hdr-Examples
package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	if err := fe(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func fe() error {
	b, err := ioutil.ReadAll(bufio.NewReader(os.Stdin))
	if err != nil {
		return err
	}

	in := strings.Split(string(b), "\n")
	m := map[string]string{} // Example test name -> got
	var lines []string
parse:
	for i := 0; i < len(in); i++ {
		f := strings.Split(in[i], " ")
		if len(f) < 3 || f[0] != "---" || f[1] != "FAIL:" || !strings.HasPrefix(f[2], "Example") {
			continue
		}

		i++
		if i == len(in) {
			break
		}

		if in[i] != "got:" {
			continue
		}

		lines = lines[:0]
		for {
			i++
			if i == len(in) {
				break parse
			}

			switch s := in[i]; s {
			case "want:":
				m[f[2]] = strings.Join(lines, "\n")
				continue parse
			default:
				lines = append(lines, s)
			}
		}
	}

	files, err := filepath.Glob("*_test.go")
	if err != nil {
		return err
	}

	in = nil

	for _, file := range files {
		b, err := ioutil.ReadFile(file)
		if err != nil {
			return err
		}

		in := strings.Split(string(b), "\n")
		lines = lines[:0]
		modified := false

		const (
			idle = iota
			seekOutput
			seekEnd
		)
		var state int
		var got string
		for i := 0; i < len(in); i++ {
			s := in[i]
			switch state {
			case idle:
				lines = append(lines, s)
				if !strings.HasPrefix(s, "func ") {
					continue
				}

				var ok bool
				if got, ok = m[strings.Split(s[len("func "):], "(")[0]]; !ok {
					continue
				}

				state = seekOutput
			case seekOutput:
				lines = append(lines, s)
				if strings.TrimSpace(s) != "// Output:" {
					continue
				}

				state = seekEnd
			case seekEnd:
				if s != "}" {
					continue
				}

				for _, s := range strings.Split(got, "\n") {
					lines = append(lines, "\t// "+s)
				}
				lines = append(lines, s)
				modified = true
				state = idle
			}
		}

		if !modified {
			continue
		}

		f, err := os.Create(file)
		if err != nil {
			return err
		}

		if _, err := f.WriteString(strings.Join(lines, "\n")); err != nil {
			return err
		}

		if err = f.Close(); err != nil {
			return err
		}
	}

	return nil
}
