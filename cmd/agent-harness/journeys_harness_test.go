package main

import (
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/BA-CalderonMorales/agent-harness/internal/interface/tui"
	"github.com/BA-CalderonMorales/agent-harness/internal/testharness/testenv"
	"github.com/BA-CalderonMorales/agent-harness/internal/testharness/ttui"
)

// Journey tests boot the real app (newApp) in an isolated home, drive
// it through the paths a user takes, and assert on the frame the
// terminal receives. run() is not called: it probes the network and
// owns the terminal. wireJourneyTUI mirrors its wiring instead.

// journeyEnvSettings are the config settings the environment can pin.
// Each is read as AH_<name> and as AGENT_HARNESS_<name> (see the
// firstEnv calls in internal/core/config), so journeyEnvNames clears both.
var journeyEnvSettings = []string{
	"PROVIDER", "RUNTIME", "MODEL", "MODEL_PATH", "ENDPOINT_URL", "API_KEY",
	"CONTEXT_LENGTH", "TEMPERATURE", "MAX_TOKENS", "WORKSPACE_PATH",
	"LOCAL_SERVER_COMMAND", "PERSONA", "PERMISSION_MODE", "EXECUTION_MODE",
	"PERM_READ", "PERM_WRITE", "PERM_DELETE", "PERM_EXECUTE",
	"HTTP_TIMEOUT", "STREAM_IDLE_TIMEOUT", "LOOP_TRACK", "REDUCED_MOTION",
	"SESSION_DIR",
}

// journeyExtraEnv are variables read outside the AH_/AGENT_HARNESS_
// pairs: provider key fallbacks the credential flow consults, and the
// local-server host.
var journeyExtraEnv = []string{
	"OPENROUTER_API_KEY", "NVIDIA_API_KEY", "OMNITROUTE_API_KEY",
	"OPENAI_API_KEY", "ANTHROPIC_API_KEY", "FIREWORKS_API_KEY",
	"OLLAMA_HOST",
}

// journeyEnvNames are the variables that pin config or credentials from
// the environment. A developer's shell must not leak into a journey.
var journeyEnvNames = func() []string {
	names := append([]string{}, journeyExtraEnv...)
	for _, setting := range journeyEnvSettings {
		names = append(names, "AH_"+setting, "AGENT_HARNESS_"+setting)
	}
	return names
}()

// journeyHome isolates a journey: a temp workspace as the working
// directory, a temp home, and no inherited config pins. It returns the
// workspace root. Booting newApp twice in one journey reuses it.
func journeyHome(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	workspace := filepath.Join(root, "workspace")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	if err := os.Chdir(workspace); err != nil {
		t.Fatalf("Chdir(%q) error = %v", workspace, err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(originalDir); err != nil {
			t.Errorf("restore working directory: %v", err)
		}
	})
	testenv.SetHome(t, filepath.Join(root, "home"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "data"))
	t.Setenv("AGENT_HARNESS_SESSION_DIR", filepath.Join(root, "sessions"))
	for _, name := range journeyEnvNames {
		t.Setenv(name, "")
	}
	journeyOffline(t)
	return workspace
}

// journeyLocal pins the local provider (and its default model) through
// the environment, the way a llama.cpp user runs the harness, so turns
// need no credentials.
func journeyLocal(t *testing.T) {
	t.Helper()
	t.Setenv("AH_PROVIDER", "local")
	t.Setenv("AH_API_KEY", "local")
	t.Setenv("AH_EXECUTION_MODE", "interactive")
}

// journeyOffline replaces the default transport for the journey. Model
// discovery and provider probes get an answer without a socket; the
// journey's own fake client never touches HTTP at all.
func journeyOffline(t *testing.T) {
	t.Helper()
	original := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = original })
	http.DefaultTransport = lemonadeTransport(func(r *http.Request) (*http.Response, error) {
		if strings.HasSuffix(r.URL.Path, "/models") {
			body := `{"data":[{"id":"journey/model-b","context_length":65536}]}`
			return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
		}
		return nil, errors.New("journey: network disabled")
	})
}

// bootJourneyApp boots the real app the way main does, minus run().
func bootJourneyApp(t *testing.T) *App {
	t.Helper()
	app, err := newApp()
	if err != nil {
		t.Fatalf("newApp() error = %v", err)
	}
	return app
}

// wireJourneyTUI connects a fresh TUI to the app with the delegates and
// initial data run() installs, without the probes and goroutines that
// reach the network or the terminal.
func wireJourneyTUI(t *testing.T, app *App) *tui.App {
	t.Helper()
	tuiApp := tui.NewApp()
	app.tuiApp = tuiApp
	app.initCommandsForTUI(tuiApp)
	tuiApp.SetUserSubmitHandler(func(text string, ta *tui.App) { app.handleUserSubmit(text, ta) })
	tuiApp.SetUserCommandHandler(func(cmd string, ta *tui.App) { app.handleUserCommand(cmd, ta) })
	tuiApp.SetHomeDelegate(&tuiHomeDelegate{app: app, tuiApp: tuiApp})
	tuiApp.SetSessionsDelegate(&tuiSessionsDelegate{app: app, tuiApp: tuiApp})
	tuiApp.SetSettingsDelegate(&tuiSettingsDelegate{app: app, tuiApp: tuiApp})
	tuiApp.SetChatDelegate(&tuiChatDelegate{app: app, tuiApp: tuiApp})
	tuiApp.SetLoginHandler(func(provider, apiKey, model string, ta *tui.App) {
		app.completeLogin(provider, apiKey, model, ta)
	})
	tuiApp.SetProviderPickHandler(func(provider string, ta *tui.App) { app.pickProvider(provider, ta) })
	tuiApp.SetModelContextHandler(func(provider, model string, contextLength int) {
		app.applyDiscoveredModelContext(tuiApp, provider, model, contextLength)
	})

	tuiApp.SetChatMessages(app.session.Messages)
	tuiApp.RefreshSessions(app.getSessionInfos())
	tuiApp.SetSettings(app.getSettings())
	tuiApp.SetModels(app.getModelItems())
	tuiApp.SetChatModel(app.session.Model)
	tuiApp.SetChatPersona(app.session.Persona)
	tuiApp.SetRuntimeContext(app.config.Provider, app.config.Effort, app.cwd)
	tuiApp.SetReducedMotion(true)
	tuiApp.SetAgentMode(string(app.agentMode))
	tuiApp.SetHomeStatus(app.session.Model, app.config.PermissionMode.String(), app.session.Persona, app.session.EstimateTokens())
	return tuiApp
}

// journeyScreen is a terminal-sized harness around the TUI plus a
// source that reads the app's message channel, which is where every
// turn, approval, and session event lands.
type journeyScreen struct {
	*ttui.Harness
	tuiApp *tui.App
	source func() (tea.Msg, bool)
}

func newJourneyScreen(t *testing.T, tuiApp *tui.App, width, height int) *journeyScreen {
	t.Helper()
	h := ttui.New(t, tuiApp)
	h.Resize(width, height)
	channel := exportedField(reflect.ValueOf(tuiApp).Elem().FieldByName("msgChan"))
	source := func() (tea.Msg, bool) {
		chosen, msg, ok := reflect.Select([]reflect.SelectCase{
			{Dir: reflect.SelectRecv, Chan: channel},
			{Dir: reflect.SelectDefault},
		})
		if chosen == 1 || !ok {
			return nil, false
		}
		return msg.Interface().(tea.Msg), true
	}
	return &journeyScreen{Harness: h, tuiApp: tuiApp, source: source}
}

// Pump delivers everything the app has sent and every command that
// follows, until the screen is idle.
func (s *journeyScreen) Pump() int {
	return s.Drain(s.source)
}

// Keys types text one rune at a time, running each key's commands the
// way a live program would before the next key arrives.
func (s *journeyScreen) Keys(text string) {
	for _, r := range text {
		s.SendAndPump(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
}

// journeyChatDelegate records what the composer submits instead of
// starting the turn on a command goroutine. Production runs the turn on
// a command goroutine, where runAgentTurn's direct tui.App writes
// (refreshTelemetry -> SetTelemetry) race the renderer; the journeys
// run the turn on the test goroutine, between frames, so the race
// detector reports the journeys' own bugs rather than that one.
type journeyChatDelegate struct{ submitted []string }

func (d *journeyChatDelegate) OnSubmit(text string) tea.Cmd {
	d.submitted = append(d.submitted, text)
	return nil
}

func (d *journeyChatDelegate) OnCommand(string) {}

// Submit types text into the composer, presses Enter, and runs the turn
// the composer submitted through the app's own submit handler.
func (s *journeyScreen) Submit(t *testing.T, app *App, text string) {
	t.Helper()
	delegate := &journeyChatDelegate{}
	s.tuiApp.SetChatDelegate(delegate)
	s.Keys(text)
	s.SendAndPump(tea.KeyMsg{Type: tea.KeyEnter})
	// Enter submits after the composer's debounce timer, which outlives
	// one pump's idle window.
	for deadline := time.Now().Add(2 * time.Second); len(delegate.submitted) == 0 && time.Now().Before(deadline); {
		time.Sleep(10 * time.Millisecond)
		s.Pump()
	}
	if len(delegate.submitted) != 1 || delegate.submitted[0] != text {
		t.Fatalf("composer submitted %q, want [%q]:\n%s", delegate.submitted, text, s.Text())
	}
	app.handleUserSubmit(text, s.tuiApp)
	s.Pump()
}

// waitFor pumps the screen until the frame shows fragment.
func (s *journeyScreen) waitFor(t *testing.T, fragment string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		s.Pump()
		if strings.Contains(s.Text(), fragment) {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("frame never showed %q:\n%s", fragment, s.Text())
}

// waitIdle pumps the screen until no turn is in flight.
func (s *journeyScreen) waitIdle(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		s.Pump()
		if !s.tuiApp.TurnInFlight() {
			s.Pump()
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("turn never settled:\n%s", s.Text())
}
