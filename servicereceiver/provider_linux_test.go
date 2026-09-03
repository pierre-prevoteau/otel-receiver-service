// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package servicereceiver

import (
	"context"
	"errors"
	"testing"

	"github.com/godbus/dbus/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// fakeBus is a dbusConnection that answers every method call with a canned
// reply, so the systemd provider can be tested without a running bus.
type fakeBus struct {
	object   *fakeBusObject
	closed   bool
	closeErr error
}

func (b *fakeBus) Object(string, dbus.ObjectPath) dbus.BusObject {
	return b.object
}

func (b *fakeBus) Close() error {
	b.closed = true
	return b.closeErr
}

type fakeBusObject struct {
	dbus.BusObject

	reply *dbus.Call

	calledMethod string
	calledArgs   []any
}

func (o *fakeBusObject) CallWithContext(_ context.Context, method string, _ dbus.Flags, args ...any) *dbus.Call {
	o.calledMethod = method
	o.calledArgs = args
	return o.reply
}

// listUnitsReply builds the reply body systemd returns for
// ListUnitsByPatterns: an array of (ssssssouso) structs.
func listUnitsReply(units ...[]any) *dbus.Call {
	body := make([][]any, 0, len(units))
	body = append(body, units...)
	return &dbus.Call{Body: []any{body}}
}

func unitReply(name, activeState string) []any {
	return []any{
		name, "a unit", "loaded", activeState, "running", "",
		dbus.ObjectPath("/org/freedesktop/systemd1/unit/test"), uint32(0), "",
		dbus.ObjectPath("/"),
	}
}

func newTestSystemdProvider(t *testing.T, scope string, bus *fakeBus) *systemdProvider {
	t.Helper()

	cfg := createDefaultConfig().(*Config)
	cfg.Scope = scope

	p, ok := newProvider(cfg, zap.NewNop()).(*systemdProvider)
	require.True(t, ok)
	require.Equal(t, scope, p.scope)

	p.connect = func(context.Context) (dbusConnection, error) {
		if bus == nil {
			return nil, errors.New("no bus")
		}
		return bus, nil
	}

	return p
}

func TestSystemdProviderList(t *testing.T) {
	bus := &fakeBus{object: &fakeBusObject{reply: listUnitsReply(
		unitReply("nginx.service", "active"),
		unitReply("cron.service", "inactive"),
		unitReply("systemd-timesyncd.service", "failed"),
	)}}
	p := newTestSystemdProvider(t, scopeSystem, bus)

	require.NoError(t, p.start(context.Background()))

	services, err := p.list(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []serviceInfo{
		{Name: "nginx", NativeName: "nginx.service", State: StateRunning},
		{Name: "cron", NativeName: "cron.service", State: StateStopped},
		{Name: "systemd-timesyncd", NativeName: "systemd-timesyncd.service", State: StateFailed},
	}, services)

	assert.Equal(t, listUnitsByPatternsMethod, bus.object.calledMethod)
	assert.Equal(t, []any{[]string{}, []string{"*.service"}}, bus.object.calledArgs)

	require.NoError(t, p.shutdown(context.Background()))
	assert.True(t, bus.closed)
	assert.Nil(t, p.conn)
}

func TestSystemdProviderListCallError(t *testing.T) {
	wantErr := errors.New("call failed")
	bus := &fakeBus{object: &fakeBusObject{reply: &dbus.Call{Err: wantErr}}}
	p := newTestSystemdProvider(t, scopeSystem, bus)

	require.NoError(t, p.start(context.Background()))

	_, err := p.list(context.Background())
	require.ErrorIs(t, err, wantErr)
}

func TestSystemdProviderListBeforeStart(t *testing.T) {
	p := newTestSystemdProvider(t, scopeSystem, nil)

	_, err := p.list(context.Background())
	require.ErrorIs(t, err, errNotStarted)
}

func TestSystemdProviderStartError(t *testing.T) {
	p := newTestSystemdProvider(t, scopeSystem, nil)

	require.Error(t, p.start(context.Background()))
	assert.Nil(t, p.conn)
}

func TestSystemdProviderShutdownWithoutStart(t *testing.T) {
	p := newTestSystemdProvider(t, scopeSystem, nil)

	require.NoError(t, p.shutdown(context.Background()))
}

// A scope other than system or user is rejected by Config.Validate, so this is
// only a guard against a provider built from an unvalidated config.
func TestSystemdProviderConnectInvalidScope(t *testing.T) {
	p := &systemdProvider{scope: "session"}

	_, err := p.connectBus(context.Background())
	require.ErrorIs(t, err, errInvalidScope)
}

func TestUnitToServiceInfo(t *testing.T) {
	for _, tt := range []struct {
		unitName string
		expected serviceInfo
	}{
		{
			unitName: "nginx.service",
			expected: serviceInfo{Name: "nginx", NativeName: "nginx.service", State: StateRunning},
		},
		{
			// Instantiated units keep their instance name.
			unitName: "getty@tty1.service",
			expected: serviceInfo{Name: "getty@tty1", NativeName: "getty@tty1.service", State: StateRunning},
		},
		{
			// A name without the suffix is reported as is.
			unitName: "nginx",
			expected: serviceInfo{Name: "nginx", NativeName: "nginx", State: StateRunning},
		},
	} {
		t.Run(tt.unitName, func(t *testing.T) {
			unit := &unitTuple{Name: tt.unitName, ActiveState: systemdActive}
			assert.Equal(t, tt.expected, unitToServiceInfo(unit))
		})
	}
}
