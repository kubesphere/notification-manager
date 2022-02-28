package utils

import (
	"regexp"
	"strings"
)

func StringIsNil(s string) bool {
	return s == ""
}

func ArrayToString(array []string, sep string) string {

	if len(array) == 0 {
		return ""
	}

	s := ""
	for _, elem := range array {
		s = s + elem + sep
	}

	return strings.TrimSuffix(s, sep)
}

func StringInList(s string, list []string) bool {

	for _, v := range list {
		if s == v {
			return true
		}
	}

	return false
}

func RegularMatch(expr, s string) bool {

	if StringIsNil(expr) {
		return false
	}

	regex, _ := regexp.Compile(expr)
	return regex.Match([]byte(s))
}
