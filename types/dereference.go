package types

import (
	"errors"
	"regexp"
	"strings"
)

func GetDereferenceType(cType string) (string, error) {
	// In the form of: "char [8]" -> "char"
	search := regexp.MustCompile(`([\w ]+)\s*\[\d+\]`).FindStringSubmatch(cType)
	if len(search) > 0 {
		return strings.TrimSpace(search[1]), nil
	}

	// In the form of: "char **" -> "char *"
	search = regexp.MustCompile(`([\w ]+)\s*(\*+)`).FindStringSubmatch(cType)
	if len(search) > 0 {
		return strings.TrimSpace(search[1] + search[2][0:len(search[2])-1]), nil
	}

	return "", errors.New(cType)
}
