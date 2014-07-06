package main

import (
	"io"
	"log"
)

type Errors []error

func (e Errors) hasErrors() bool {
	return len(e) > 0
}

func (e Errors) printErrors(w io.Writer) {
	log.SetOutput(w)
	for _, err := range e {
		log.Println(err)
	}
}
