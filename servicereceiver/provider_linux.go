// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package servicereceiver // import "github.com/pierre-prevoteau/otel-receiver-service/servicereceiver"

import (
	"context"
	"strings"

	"github.com/godbus/dbus/v5"
	"go.uber.org/zap"
)

const (
	systemdBusName            = "org.freedesktop.systemd1"
	systemdObjectPath         = dbus.ObjectPath("/org/freedesktop/systemd1")
	listUnitsByPatternsMethod = "org.freedesktop.systemd1.Manager.ListUnitsByPatterns"

	// serviceUnitSuffix limits collection to service units; timers, sockets and
	// the other unit types are not services.
	serviceUnitSuffix = ".service"
)

// dbusConnection is the subset of [dbus.Conn] used by the provider, so that
// tests can substitute a fake connection.
type dbusConnection interface {
	Object(dest string, path dbus.ObjectPath) dbus.BusObject
	Close() error
}

// unitTuple is one entry of the array returned by ListUnitsByPatterns.
type unitTuple struct {
	Name        string
	Description string
	LoadState   string
	ActiveState string
	SubState    string
	Following   string
	Path        dbus.ObjectPath
	JobID       uint32
	JobType     string
	JobPath     dbus.ObjectPath
}

// systemdProvider reads unit state from systemd over D-Bus.
type systemdProvider struct {
	scope string
	conn  dbusConnection

	// connect is a field so tests can inject a fake bus.
	connect func(ctx context.Context) (dbusConnection, error)
}

func newProvider(cfg *Config, _ *zap.Logger) serviceProvider {
	p := &systemdProvider{scope: cfg.Scope}
	p.connect = p.connectBus
	return p
}

func (p *systemdProvider) connectBus(ctx context.Context) (dbusConnection, error) {
	var (
		conn *dbus.Conn
		err  error
	)

	switch p.scope {
	case scopeSystem:
		conn, err = dbus.ConnectSystemBus(dbus.WithContext(ctx))
	case scopeUser:
		conn, err = dbus.ConnectSessionBus(dbus.WithContext(ctx))
	default:
		return nil, errInvalidScope
	}
	if err != nil {
		return nil, err
	}

	return conn, nil
}

func (p *systemdProvider) start(ctx context.Context) error {
	conn, err := p.connect(ctx)
	if err != nil {
		return err
	}

	p.conn = conn
	return nil
}

func (p *systemdProvider) list(ctx context.Context) ([]serviceInfo, error) {
	if p.conn == nil {
		return nil, errNotStarted
	}

	// systemd only knows about loaded units, so services that have never been
	// loaded since boot are not reported.
	var units []unitTuple
	err := p.conn.Object(systemdBusName, systemdObjectPath).
		CallWithContext(ctx, listUnitsByPatternsMethod, 0, []string{}, []string{"*" + serviceUnitSuffix}).
		Store(&units)
	if err != nil {
		return nil, err
	}

	services := make([]serviceInfo, 0, len(units))
	for i := range units {
		services = append(services, unitToServiceInfo(&units[i]))
	}

	return services, nil
}

func (p *systemdProvider) shutdown(context.Context) error {
	if p.conn == nil {
		return nil
	}

	err := p.conn.Close()
	p.conn = nil
	return err
}

func unitToServiceInfo(unit *unitTuple) serviceInfo {
	return serviceInfo{
		Name:       strings.TrimSuffix(unit.Name, serviceUnitSuffix),
		NativeName: unit.Name,
		State:      stateFromSystemdActiveState(unit.ActiveState),
	}
}
