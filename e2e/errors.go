package e2e

// BehaviorError represents a behavior test failure
type BehaviorError string

func (e BehaviorError) Error() string {
	return string(e)
}

// Common behavior errors
var (
	ErrSessionNotPersisted = BehaviorError("session state not persisted")
	ErrToolNotSafe         = BehaviorError("tool execution safety violated")
	ErrConfigNotLayered    = BehaviorError("configuration layering incorrect")
	ErrNoGracefulDegrade   = BehaviorError("graceful degradation failed")
	ErrNoLearning          = BehaviorError("cross-session learning not working")
)
