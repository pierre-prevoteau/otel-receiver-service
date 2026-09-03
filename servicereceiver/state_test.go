// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package servicereceiver

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStateFromSystemdActiveState(t *testing.T) {
	for _, tt := range []struct {
		activeState string
		expected    State
	}{
		{"active", StateRunning},
		{"reloading", StateRunning},
		{"refreshing", StateRunning},
		{"activating", StateStarting},
		{"deactivating", StateStopping},
		{"inactive", StateStopped},
		{"maintenance", StatePaused},
		{"failed", StateFailed},
		{"", StateUnknown},
		{"something-new", StateUnknown},
	} {
		t.Run(tt.activeState, func(t *testing.T) {
			require.Equal(t, tt.expected, stateFromSystemdActiveState(tt.activeState))
		})
	}
}

func TestStateFromWindowsStatus(t *testing.T) {
	for _, tt := range []struct {
		name         string
		currentState uint32
		exitCode     uint32
		expected     State
	}{
		{"running", winStateRunning, 0, StateRunning},
		{"start pending", winStateStartPending, 0, StateStarting},
		{"continue pending", winStateContinuePending, 0, StateStarting},
		{"stop pending", winStateStopPending, 0, StateStopping},
		{"pause pending", winStatePausePending, 0, StatePaused},
		{"paused", winStatePaused, 0, StatePaused},
		{"stopped cleanly", winStateStopped, 0, StateStopped},
		{"stopped and never started", winStateStopped, errorServiceNeverStarted, StateStopped},
		{"stopped with error", winStateStopped, 1067, StateFailed},
		{"unrecognized state", 42, 0, StateUnknown},
	} {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, stateFromWindowsStatus(tt.currentState, tt.exitCode))
		})
	}
}
