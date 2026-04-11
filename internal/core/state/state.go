package state

import (
	"sync"
)

// Store is a thread-safe generic state container.
type Store struct {
	mu    sync.RWMutex
	state map[string]any
}

// NewStore creates an empty state store.
func NewStore() *Store {
	return &Store{state: make(map[string]any)}
}

// Get retrieves a value by key.
func (s *Store) Get(key string) (any, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.state[key]
	return v, ok
}

// Set stores a value by key.
func (s *Store) Set(key string, value any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state[key] = value
}

// Update applies a transformation function to the current value.
func (s *Store) Update(key string, updater func(current any) any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state[key] = updater(s.state[key])
}

// Delete removes a key.
func (s *Store) Delete(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.state, key)
}

// Snapshot returns a shallow copy of the entire state.
func (s *Store) Snapshot() map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]any, len(s.state))
	for k, v := range s.state {
		out[k] = v
	}
	return out
}

// AppState is the concrete application state shape.
type AppState struct {
	Store          *Store
	PermissionMode string
	FastMode       bool
	Model          string
	McpTools       []any
	McpClients     []any
	FileHistory    map[string]FileVersion
}

// FileVersion tracks a file for undo operations.
type FileVersion struct {
	Path    string
	Content string
	Hash    string
}

// NewAppState creates the default application state.
func NewAppState() *AppState {
	return &AppState{
		Store:          NewStore(),
		PermissionMode: "default",
		Model:          "",
		FileHistory:    make(map[string]FileVersion),
	}
}

// PushFileHistory saves the current content of a file for potential undo.
func (a *AppState) PushFileHistory(path, content, hash string) {
	a.FileHistory[path] = FileVersion{
		Path:    path,
		Content: content,
		Hash:    hash,
	}
}

// PopFileHistory restores the last known version of a file.
func (a *AppState) PopFileHistory(path string) (FileVersion, bool) {
	v, ok := a.FileHistory[path]
	if ok {
		delete(a.FileHistory, path)
	}
	return v, ok
}
