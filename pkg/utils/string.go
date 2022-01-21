package utils

import (
	"strings"
)

func StringIsNil(s string) bool {
	return s == ""
}

func ArrayToString(array []string, sep string) string {

	if array == nil || len(array) == 0 {
		return ""
	}

	s := ""
	for _, elem := range array {
		s = s + elem + sep
	}

	return strings.TrimSuffix(s, sep)
}
