// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package servicereceiver // import "github.com/pierre-prevoteau/otel-receiver-service/servicereceiver"

// State is the platform independent service state reported as the value of the
// system.service.state metric. The numeric codes are part of the metric
// contract and must not be reordered.
type State int64

const (
	StateUnknown  State = 0
	StateStopped  State = 1
	StateStarting State = 2
	StateStopping State = 3
	StateRunning  State = 4
	StatePaused   State = 5
	StateFailed   State = 6
)

// systemd unit ActiveState values.
// See https://www.freedesktop.org/software/systemd/man/latest/systemd.html#Units
const (
	systemdActive       = "active"
	systemdReloading    = "reloading"
	systemdRefreshing   = "refreshing"
	systemdInactive     = "inactive"
	systemdFailed       = "failed"
	systemdActivating   = "activating"
	systemdDeactivating = "deactivating"
	systemdMaintenance  = "maintenance"
)

// stateFromSystemdActiveState maps a systemd unit ActiveState to a State.
//
// A unit that is reloading or refreshing is still serving requests, so it is
// reported as running rather than as a transitional state.
func stateFromSystemdActiveState(activeState string) State {
	switch activeState {
	case systemdActive, systemdReloading, systemdRefreshing:
		return StateRunning
	case systemdActivating:
		return StateStarting
	case systemdDeactivating:
		return StateStopping
	case systemdInactive:
		return StateStopped
	case systemdMaintenance:
		return StatePaused
	case systemdFailed:
		return StateFailed
	default:
		return StateUnknown
	}
}

// Windows SERVICE_STATUS.dwCurrentState values.
// See https://learn.microsoft.com/windows/win32/api/winsvc/ns-winsvc-service_status
const (
	winStateStopped         uint32 = 1
	winStateStartPending    uint32 = 2
	winStateStopPending     uint32 = 3
	winStateRunning         uint32 = 4
	winStateContinuePending uint32 = 5
	winStatePausePending    uint32 = 6
	winStatePaused          uint32 = 7
)

// errorServiceNeverStarted is the Win32 exit code (1077) the Service Control
// Manager reports for a service that has not been started since the last boot.
// It is not an indication of failure.
const errorServiceNeverStarted uint32 = 1077

// stateFromWindowsStatus maps a Windows service status to a State.
//
// Windows has no dedicated failed state: a service whose process terminated
// abnormally is reported as stopped with a non-zero exit code. That case is
// mapped to StateFailed so the metric behaves the same way as on Linux.
func stateFromWindowsStatus(currentState, win32ExitCode uint32) State {
	switch currentState {
	case winStateRunning:
		return StateRunning
	case winStateStartPending, winStateContinuePending:
		return StateStarting
	case winStateStopPending:
		return StateStopping
	case winStatePausePending, winStatePaused:
		return StatePaused
	case winStateStopped:
		if win32ExitCode != 0 && win32ExitCode != errorServiceNeverStarted {
			return StateFailed
		}
		return StateStopped
	default:
		return StateUnknown
	}
}
