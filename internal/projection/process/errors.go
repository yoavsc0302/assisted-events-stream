package process

import (
	"fmt"
)

type MalformedEventError struct {
	msg string
}

func NewMalformedEventError(msg string) error {
	return &MalformedEventError{
		msg: msg,
	}
}

func (e *MalformedEventError) Error() string {
	return e.msg
}

type PathDoesNotExistError struct {
	msg string
}

func NewPathDoesNotExistError(path string) error {
	return &PathDoesNotExistError{
		msg: fmt.Sprintf("Path does not exist: %s", path),
	}
}

func (e *PathDoesNotExistError) Error() string {
	return e.msg
}
