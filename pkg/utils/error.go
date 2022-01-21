package utils

import "fmt"

func Error(msg string) error {
	return Errorf("%s", msg)
}

func Errorf(format string, a ...interface{}) error {
	return fmt.Errorf(format, a...)
}
