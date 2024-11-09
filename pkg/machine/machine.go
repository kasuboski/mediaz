package machine

import "errors"

type State interface {
	~string
}

// Allowable maps where a from state is allowed to transition to
type Allowable[S State] struct {
	from S
	to   []S
}

// StateMachine manages the state of a context
type StateMachine[S State] struct {
	fromState S
	toStates  []Allowable[S]
}

var (
	ErrInvalidTransition = errors.New("invalid state transition")
)

// TransitionBuilder helps in creating a from-to relationship for state transitions
type TransitionBuilder[S State] struct {
	transition Allowable[S]
}

func New[S State](currentState S, transitions ...Allowable[S]) *StateMachine[S] {
	return &StateMachine[S]{fromState: currentState, toStates: transitions}
}

// From initializes a transition from a specific state
func From[S State](from S) *TransitionBuilder[S] {
	return &TransitionBuilder[S]{transition: Allowable[S]{from: from}}
}

// To sets the possible destination states and returns the configured transition
func (tb *TransitionBuilder[S]) To(to ...S) Allowable[S] {
	tb.transition.to = to
	return tb.transition
}

// ToState determines if a given state can transition to another state
func (m *StateMachine[S]) ToState(s S) error {
	for _, transition := range m.toStates {
		// can't transition from one state to another state if we're not in the same from state
		if transition.from != m.fromState {
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
