package builtin

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/BA-CalderonMorales/agent-harness/internal/runtime/tools"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Filesystem Tools", func() {
	var tmpDir string
	var ctx tools.Context

	BeforeEach(func() {
		tmpDir = GinkgoT().TempDir()
		ctx = tools.Context{
			AbortController: GinkgoT().Context(),
			GlobLimits:      tools.GlobLimits{MaxResults: 100},
		}

		// Seed a simple directory structure
		Expect(os.WriteFile(filepath.Join(tmpDir, "alpha.txt"), []byte("alpha"), 0644)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(tmpDir, "beta.go"), []byte("beta"), 0644)).To(Succeed())
		Expect(os.MkdirAll(filepath.Join(tmpDir, "subdir"), 0755)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(tmpDir, "subdir", "gamma.go"), []byte("gamma"), 0644)).To(Succeed())
	})

	Describe("ListDirectoryTool", func() {
		Context("Given a directory with files and subdirectories", func() {
			It("should list all entries with types and sizes", func() {
				By("calling ls on the temp directory")
				result, err := ListDirectoryTool.Call(map[string]any{"path": tmpDir}, ctx, nil, nil)
				Expect(err).ToNot(HaveOccurred())

				By("verifying the result contains expected entries")
				content, ok := result.Data.(string)
				Expect(ok).To(BeTrue())
				Expect(content).To(ContainSubstring("alpha.txt"))
				Expect(content).To(ContainSubstring("beta.go"))
				Expect(content).To(ContainSubstring("subdir"))
				Expect(content).To(ContainSubstring("3 entries"))
			})
		})

		Context("Given an empty directory", func() {
			It("should return zero entries", func() {
				emptyDir := filepath.Join(tmpDir, "empty")
				Expect(os.MkdirAll(emptyDir, 0755)).To(Succeed())

				result, err := ListDirectoryTool.Call(map[string]any{"path": emptyDir}, ctx, nil, nil)
				Expect(err).ToNot(HaveOccurred())

				content, _ := result.Data.(string)
				Expect(content).To(ContainSubstring("0 entries"))
			})
		})

		Context("Given a missing path", func() {
			It("should return an error message", func() {
				result, err := ListDirectoryTool.Call(map[string]any{"path": filepath.Join(tmpDir, "nope")}, ctx, nil, nil)
				Expect(err).ToNot(HaveOccurred())

				content, _ := result.Data.(string)
				Expect(content).To(ContainSubstring("error:"))
			})
		})

		Context("Given no path", func() {
			It("should fail validation", func() {
				vr := ListDirectoryTool.ValidateInput(map[string]any{}, ctx)
				Expect(vr.Valid).To(BeFalse())
			})
		})
	})

	Describe("FindTool", func() {
		Context("Given a pattern matching multiple files recursively", func() {
			It("should return all matching paths", func() {
				By("searching for '*.go' files")
				result, err := FindTool.Call(map[string]any{"pattern": "*.go", "path": tmpDir}, ctx, nil, nil)
				Expect(err).ToNot(HaveOccurred())

				By("verifying both .go files are found")
				content, _ := result.Data.(string)
				lines := strings.Split(strings.TrimSpace(content), "\n")
				Expect(lines).To(HaveLen(2))
				Expect(content).To(ContainSubstring("beta.go"))
				Expect(content).To(ContainSubstring("gamma.go"))
			})
		})

		Context("Given a pattern with no matches", func() {
			It("should return a no-files-found message", func() {
				result, err := FindTool.Call(map[string]any{"pattern": "*.rs", "path": tmpDir}, ctx, nil, nil)
				Expect(err).ToNot(HaveOccurred())

				content, _ := result.Data.(string)
				Expect(content).To(Equal("(no files found)"))
			})
		})

		Context("Given an ignored directory", func() {
			It("should skip descending into common ignored dirs", func() {
				By("creating a file inside node_modules")
				nodeMods := filepath.Join(tmpDir, "node_modules", "pkg")
				Expect(os.MkdirAll(nodeMods, 0755)).To(Succeed())
				Expect(os.WriteFile(filepath.Join(nodeMods, "hidden.go"), []byte("hidden"), 0644)).To(Succeed())

				By("searching for '*.go'")
				result, err := FindTool.Call(map[string]any{"pattern": "*.go", "path": tmpDir}, ctx, nil, nil)
				Expect(err).ToNot(HaveOccurred())

				By("verifying hidden.go inside node_modules is NOT found")
				content, _ := result.Data.(string)
				Expect(content).ToNot(ContainSubstring("hidden.go"))
				Expect(content).To(ContainSubstring("beta.go"))
				Expect(content).To(ContainSubstring("gamma.go"))
			})
		})

		Context("Given no pattern", func() {
			It("should fail validation", func() {
				vr := FindTool.ValidateInput(map[string]any{}, ctx)
				Expect(vr.Valid).To(BeFalse())
			})
		})
	})

	Describe("LsRecursiveTool", func() {
		Context("Given a directory with nested contents", func() {
			It("should list all paths up to the depth limit", func() {
				result, err := LsRecursiveTool.Call(map[string]any{"path": tmpDir, "depth": 3}, ctx, nil, nil)
				Expect(err).ToNot(HaveOccurred())

				content, _ := result.Data.(string)
				Expect(content).To(ContainSubstring("alpha.txt"))
				Expect(content).To(ContainSubstring("beta.go"))
				Expect(content).To(ContainSubstring("gamma.go"))
			})
		})
	})

	Describe("GlobTool", func() {
		Context("Given a simple pattern", func() {
			It("should match files in the specified directory", func() {
				By("creating files")
				Expect(os.WriteFile(filepath.Join(tmpDir, "root.go"), []byte("root"), 0644)).To(Succeed())

				By("using a '*.go' pattern")
				result, err := GlobTool.Call(map[string]any{"pattern": "*.go", "path": tmpDir}, ctx, nil, nil)
				Expect(err).ToNot(HaveOccurred())

				By("verifying matches are found")
				content, _ := result.Data.(string)
				Expect(content).To(ContainSubstring("root.go"))
				Expect(content).To(ContainSubstring("beta.go"))
			})
		})
	})
})
