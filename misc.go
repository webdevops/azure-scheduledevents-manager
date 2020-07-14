package main

import "strings"

func stringArrayContainsCi(arr []string, needle string) bool {
	for _, val := range arr {
		if strings.ToLower(val) == strings.ToLower(needle) {
			return true
		}
	}

	return false
}
