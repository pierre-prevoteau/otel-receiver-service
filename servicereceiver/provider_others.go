// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !linux && !windows

package servicereceiver // import "github.com/pierre-prevoteau/otel-receiver-service/servicereceiver"

import (
	"context"

	"go.uber.org/zap"
)

// unsupportedProvider is used on platforms without a supported service manager.
// The receiver can still be built and configured there, but it fails to start.
type unsupportedProvider struct{}

func newProvider(_ *Config, _ *zap.Logger) serviceProvider {
	return &unsupportedProvider{}
}

func (*unsupportedProvider) start(context.Context) error {
	return errUnsupportedOS
}

func (*unsupportedProvider) list(context.Context) ([]serviceInfo, error) {
	return nil, errUnsupportedOS
}

func (*unsupportedProvider) shutdown(context.Context) error {
	return nil
}
