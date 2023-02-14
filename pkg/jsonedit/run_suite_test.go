package jsonedit

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestProjectionProcessHostsSummary(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "JSON Edit")
}
