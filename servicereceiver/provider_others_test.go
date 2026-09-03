// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !linux && !windows

package servicereceiver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// On an unsupported platform the receiver can be built and configured, but it
// reports the platform as unsupported as soon as it is started.
func TestUnsupportedProvider(t *testing.T) {
	p := newProvider(createDefaultConfig().(*Config), zap.NewNop())

	require.ErrorIs(t, p.start(context.Background()), errUnsupportedOS)

	_, err := p.list(context.Background())
	require.ErrorIs(t, err, errUnsupportedOS)

	require.NoError(t, p.shutdown(context.Background()))
}
