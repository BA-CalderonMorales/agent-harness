package state

import (
	"strings"
	"testing"
	"time"

	"github.com/BA-CalderonMorales/agent-harness/pkg/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestStateExports(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "State Export Suite")
}

var _ = Describe("Session exports", func() {
	var session *Session

	BeforeEach(func() {
		session = NewSession("test-model")
		session.ID = "12345678-1234-1234-1234-123456789abc"
		session.CreatedAt = time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
		session.UpdatedAt = session.CreatedAt
		session.AddMessage(types.Message{
			Role: types.RoleUser,
			Content: []types.ContentBlock{types.TextBlock{
				Text: `OPENROUTER_API_KEY=sk-or-v1-secret123 at /data/data/com.termux/files/home/projects/app`,
			}},
		})
		session.AddMessage(types.Message{
			Role: types.RoleAssistant,
			Content: []types.ContentBlock{
				types.ToolUseBlock{
					ID:   "tool-1",
					Name: "bash",
					Input: map[string]any{
						"command": "cat /root/private/config.json",
						"api_key": "sk-ant-secret456",
					},
				},
				types.ToolResultBlock{
					ToolUseID: "tool-1",
					Content:   "failed in /home/dev/project with token ghp_secret789",
					IsError:   true,
				},
			},
			APIError: "ANTHROPIC_API_KEY=sk-ant-secret456",
		})
	})

	It("writes a maintainer-friendly text export with secrets and paths obfuscated", func() {
		By("when exporting text")
		out := session.ExportToText()

		By("then the useful transcript shape remains")
		Expect(out).To(ContainSubstring("Agent Harness Session Export"))
		Expect(out).To(ContainSubstring("== USER =="))
		Expect(out).To(ContainSubstring("[tool result]"))

		By("and sensitive values are removed")
		expectNoRawSensitiveValues(out)
		Expect(out).To(ContainSubstring("OPENROUTER_API_KEY=<redacted>"))
		Expect(out).To(ContainSubstring("<termux-home>"))
		Expect(out).To(ContainSubstring("<user-home>"))
	})

	It("redacts markdown exports", func() {
		By("when exporting markdown")
		out := session.ExportToMarkdown()

		By("then raw secrets and paths are absent")
		expectNoRawSensitiveValues(out)
		Expect(out).To(ContainSubstring("api_key"))
		Expect(out).To(ContainSubstring("<redacted>"))
	})

	It("redacts JSON exports", func() {
		By("when exporting JSON")
		data, err := session.ExportToRedactedJSON()
		Expect(err).ToNot(HaveOccurred())
		out := string(data)

		By("then raw secrets and paths are absent")
		expectNoRawSensitiveValues(out)
		Expect(out).To(ContainSubstring(`"api_key": "<redacted>"`))
	})
})

func expectNoRawSensitiveValues(out string) {
	GinkgoHelper()
	raw := []string{
		"sk-or-v1-secret123",
		"sk-ant-secret456",
		"ghp_secret789",
		"/data/data/com.termux/files/home/projects/app",
		"/root/private/config.json",
		"/home/dev/project",
	}
	for _, value := range raw {
		Expect(strings.Contains(out, value)).To(BeFalse(), "export leaked %q in:\n%s", value, out)
	}
}
