package tui

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

type loginLinkRequestMsg struct {
	url     string
	copyURL bool
}

type loginLinkResultMsg struct {
	url    string
	notice string
}

// Link actions only carry the public provider URL; the masked API-key
// buffer stays in the wizard. Results render inside the modal, since it
// covers the app's ordinary status bar.
func (a *App) handleLoginLinkMessage(msg tea.Msg) (tea.Cmd, bool) {
	steps, ok := keyStepsFor(a.loginDialog.provider())
	active := a.loginDialog.IsShowing() && a.loginDialog.step == LoginStepAPIKey &&
		!a.showHelp && !a.approvalDialog.IsVisible() && ok
	switch msg := msg.(type) {
	case loginLinkRequestMsg:
		if active && msg.url == steps.URL {
			return loginLinkActionCmd(msg, openLoginURL, copyToClipboard), true
		}
		return nil, true
	case loginLinkResultMsg:
		if active && msg.url == steps.URL {
			a.loginDialog.linkNotice = msg.notice
		}
		return nil, true
	case tea.KeyMsg:
		if active && !msg.Paste && (msg.String() == "ctrl+o" || msg.String() == "ctrl+y") {
			request := loginLinkRequestMsg{url: steps.URL, copyURL: msg.String() == "ctrl+y"}
			return func() tea.Msg { return request }, true
		}
	case tea.MouseMsg:
		if active {
			if msg.Button == tea.MouseButtonLeft && msg.Action == tea.MouseActionPress &&
				a.loginDialog.linkAt(msg.X, msg.Y, steps.URL) {
				return func() tea.Msg { return loginLinkRequestMsg{url: steps.URL} }, true
			}
			// Don't send clicks on the login overlay to the view behind it.
			return nil, true
		}
	}
	return nil, false
}

// linkAt reads the actual rendered hyperlink spans, so centered panels,
// wrapped URLs, and mobile placement share the same click coordinates.
func (m LoginDialogModel) linkAt(x, y int, url string) bool {
	linked := false
	for row, line := range strings.Split(m.View(), "\n") {
		col, state := 0, byte(0)
		for len(line) > 0 {
			seq, width, n, next := ansi.DecodeSequence(line, state, nil)
			if n == 0 {
				break
			}
			if strings.HasPrefix(seq, "\x1b]8;") {
				linked = seq == ansi.SetHyperlink(url)
			}
			if linked && row == y && x >= col && x < col+width {
				return true
			}
			col += width
			line, state = line[n:], next
		}
		if row >= y {
			break
		}
	}
	return false
}

func loginLinkActionCmd(request loginLinkRequestMsg, openURL func(string) error, copyURL func(string) bool) tea.Cmd {
	return func() tea.Msg {
		notice := "Opened API key page in your browser."
		if request.copyURL {
			notice = "URL copied to clipboard."
			if !copyURL(request.url) {
				notice = "Could not copy URL. Use Ctrl+O to open it."
			}
		} else if err := openURL(request.url); err != nil {
			notice = fmt.Sprintf("Could not open browser: %v. Use Ctrl+Y to copy the URL.", err)
		}
		return loginLinkResultMsg{url: request.url, notice: notice}
	}
}

// Use the platform URL handler directly, without passing URLs to a shell.
func openLoginURL(url string) error {
	switch runtime.GOOS {
	case "windows":
		return exec.Command("rundll32.exe", "url.dll,FileProtocolHandler", url).Run()
	case "darwin":
		return exec.Command("open", url).Run()
	default:
		if isTermux {
			return exec.Command("termux-open-url", url).Run()
		}
		if _, err := os.Stat("/proc/sys/fs/binfmt_misc/WSLInterop"); err == nil {
			return exec.Command("rundll32.exe", "url.dll,FileProtocolHandler", url).Run()
		}
		return exec.Command("xdg-open", url).Run()
	}
}
