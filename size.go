package bench

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var sizeRE = regexp.MustCompile(`(?i)^([0-9]+)([kmg]?b)?$`)

// ParseSize parses a size with B, MB, GB unit and returns its value in bytes.
func ParseSize(s string) (uint64, error) {
	m := sizeRE.FindStringSubmatch(s)
	if m == nil {
		return 0, fmt.Errorf("invalid size %q", s)
	}
	v, _ := strconv.ParseUint(m[1], 10, 64)
	switch strings.ToLower(m[2]) {
	case "kb":
		v *= 1024
	case "mb":
		v *= 1024 * 1024
	case "gb":
		v *= 1024 * 1024 * 1024
	}
	return v, nil
}
