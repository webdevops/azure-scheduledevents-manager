package manager

import "strings"

func stringArrayContainsCi(arr []string, needle string) bool {
	for _, val := range arr {
		if strings.EqualFold(val, needle) {
			return true
		}
	}

	return false
}
