package jsonedit

import (
	"strings"
)

func isComplexPath(path string) bool {
	return strings.Contains(path, "[*]")
}
