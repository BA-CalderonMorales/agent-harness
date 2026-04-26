package builtin

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestBuiltin(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Builtin Tools Suite")
}
