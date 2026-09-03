// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package servicereceiver // import "github.com/pierre-prevoteau/otel-receiver-service/servicereceiver"

import (
	"context"
	"errors"
	"fmt"
	"syscall"

	"go.uber.org/multierr"
	"go.uber.org/zap"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc/mgr"
)

// scmProvider reads service state from the Windows Service Control Manager.
type scmProvider struct {
	logger *zap.Logger
	scm    *mgr.Mgr

	// disabled is set when the Service Control Manager cannot be opened because
	// the collector lacks the rights to do so. The receiver then keeps running
	// without reporting anything, rather than taking the collector down.
	disabled bool
}

func newProvider(_ *Config, logger *zap.Logger) serviceProvider {
	return &scmProvider{logger: logger}
}

func (p *scmProvider) start(context.Context) error {
	handle, err := windows.OpenSCManager(nil, nil, windows.GENERIC_READ)
	if err != nil {
		if errors.Is(err, windows.ERROR_ACCESS_DENIED) {
			p.logger.Warn("access denied to the Service Control Manager; no service metrics will be collected",
				zap.Error(err))
			p.disabled = true
			return nil
		}
		return err
	}

	p.scm = &mgr.Mgr{Handle: handle}
	return nil
}

func (p *scmProvider) list(context.Context) ([]serviceInfo, error) {
	if p.disabled {
		return nil, nil
	}
	if p.scm == nil {
		return nil, errNotStarted
	}

	names, err := p.scm.ListServices()
	if err != nil {
		return nil, err
	}

	var errs error
	services := make([]serviceInfo, 0, len(names))
	for _, name := range names {
		state, err := p.queryState(name)
		if err != nil {
			errs = multierr.Append(errs, fmt.Errorf("failed to query service %q: %w", name, err))
			continue
		}

		services = append(services, serviceInfo{
			Name:       name,
			NativeName: name,
			State:      state,
		})
	}

	return services, errs
}

func (p *scmProvider) shutdown(context.Context) error {
	if p.scm == nil {
		return nil
	}

	err := p.scm.Disconnect()
	p.scm = nil
	return err
}

func (p *scmProvider) queryState(name string) (State, error) {
	service, err := p.openService(name)
	if err != nil {
		return StateUnknown, err
	}
	defer func() {
		if closeErr := service.Close(); closeErr != nil {
			p.logger.Debug("failed to close service handle", zap.String("service", name), zap.Error(closeErr))
		}
	}()

	status, err := service.Query()
	if err != nil {
		return StateUnknown, err
	}

	return stateFromWindowsStatus(uint32(status.State), status.Win32ExitCode), nil
}

func (p *scmProvider) openService(name string) (*mgr.Service, error) {
	if name == "" {
		return nil, windows.ERROR_INVALID_PARAMETER
	}

	namePointer, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return nil, err
	}

	handle, err := windows.OpenService(p.scm.Handle, namePointer, windows.GENERIC_READ)
	if err != nil {
		return nil, err
	}

	return &mgr.Service{Handle: handle, Name: name}, nil
}
