package pr

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestPr(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Pr Suite")
}
