package machine

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewStateMachine(t *testing.T) {
	type TestContext struct {
		name string
	}

	type TestTransition struct {
		id string
	}

	type TestState string

	const (
		StatePending   TestState = "Pending"
		StateSubmitted TestState = "Submitted"
		StateCanceled  TestState = "Canceled"
		StateDone      TestState = "Done"
	)

	t.Run("valid transition", func(t *testing.T) {
		machine := New[TestState](StatePending,
			From(StatePending).To(StateSubmitted),
			From(StateSubmitted).To(StateDone, StateCanceled),
		)

		if len(machine.toStates) != 2 {
			t.Errorf("expected %d toStates, got %d", 2, len(machine.toStates))
		}

		err := machine.ToState(StateSubmitted)
		assert.Equal(t, machine.fromState, StatePending)
		assert.Nil(t, err)
	})

	t.Run("invalid transition", func(t *testing.T) {
		machine := New[TestState](StateSubmitted,
			From(StatePending).To(StateSubmitted),
			From(StateSubmitted).To(StateDone, StateCanceled),
		)

		if len(machine.toStates) != 2 {
			t.Errorf("expected %d toStates, got %d", 2, len(machine.toStates))
		}

		err := machine.ToState(StatePending)
		assert.Equal(t, machine.fromState, StateSubmitted)
		assert.Equal(t, err, ErrInvalidTransition)
	})
}
