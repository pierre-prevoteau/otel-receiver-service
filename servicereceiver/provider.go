// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package servicereceiver // import "github.com/pierre-prevoteau/otel-receiver-service/servicereceiver"

import (
	"context"
	"errors"
)

var (
	errUnsupportedOS = errors.New("the system service receiver is only supported on Linux and Windows")
	errNotStarted    = errors.New("service provider has not been started")
)

// serviceInfo is the state of a single service as reported by the host.
type serviceInfo struct {
	// Name is the normalized, cross platform service name emitted as the
	// system.service.name attribute. On Linux the ".service" suffix of the
	// unit name is stripped so that the same name is used on both platforms.
	Name string

	// NativeName is the name the host itself uses for the service: the full
	// unit name on Linux, and the same value as Name on Windows. Include and
	// exclude patterns are matched against both names.
	NativeName string

	State State
}

// serviceProvider reads service state from the host. It is implemented once per
// platform, which keeps the rest of the receiver free of build tags.
type serviceProvider interface {
	// start opens the connection to the platform's service manager.
	start(ctx context.Context) error

	// list returns the state of every service the host reports. A non-nil
	// error together with a non-empty result means some services could not be
	// read; the returned error is then a multierr of the individual failures.
	list(ctx context.Context) ([]serviceInfo, error)

	// shutdown closes the connection opened by start.
	shutdown(ctx context.Context) error
}
