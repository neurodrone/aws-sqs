package main

import (
	"fmt"
	"io"
)

type Errors []error

func (e Errors) hasErrors() bool {
	return len(e) > 0
}

func (e Errors) printErrors(w io.Writer) {
	for _, err := range e {
		fmt.Fprintln(w, err)
	}
}
