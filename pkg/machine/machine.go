package machine

import "errors"

type State interface {
	~string
}

// Transition is the transaction model
type Transition any

// Configuration maps where a from state can transition to
type Configuration[S State] struct {
	from S
	to   []S
}

// StateMachine manages the state of a context
type StateMachine[S State] struct {
	state       S
	transitions []Configuration[S]
}

var (
	ErrInvalidTransition = errors.New("invalid state transition")
)

// TransitionBuilder helps in creating a from-to relationship for state transitions
type TransitionBuilder[S State] struct {
	transition Configuration[S]
}

func New[T Transition, S State](currentState S, transitions ...Configuration[S]) *StateMachine[S] {
	return &StateMachine[S]{transitions: transitions, state: currentState}
}

// From initializes a transition from a specific state
func From[S State](from S) *TransitionBuilder[S] {
	return &TransitionBuilder[S]{transition: Configuration[S]{from: from}}
}

// To sets the possible destination states and returns the configured transition
func (tb *TransitionBuilder[S]) To(to ...S) Configuration[S] {
	tb.transition.to = to
	return tb.transition
}

// ToState determines if a given state can transition to another state
func (m *StateMachine[S]) ToState(s S) error {
	for _, transition := range m.transitions {
		// can't transition from one state to another state if we're not in the same from state
		if transition.from != m.state {
			continue
		}

		for _, transitionToState := range transition.to {
			if transitionToState == s {
				return nil
			}
		}
	}

	return ErrInvalidTransition
}
