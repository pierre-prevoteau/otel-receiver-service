// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package servicereceiver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"golang.org/x/sys/windows"
)

func newTestSCMProvider(t *testing.T) *scmProvider {
	t.Helper()

	p, ok := newProvider(createDefaultConfig().(*Config), zap.NewNop()).(*scmProvider)
	require.True(t, ok)
	return p
}

func TestSCMProviderListBeforeStart(t *testing.T) {
	p := newTestSCMProvider(t)

	_, err := p.list(context.Background())
	require.ErrorIs(t, err, errNotStarted)
}

// When the Service Control Manager cannot be opened the receiver keeps running
// and reports nothing, instead of failing the collector.
func TestSCMProviderDisabled(t *testing.T) {
	p := newTestSCMProvider(t)
	p.disabled = true

	services, err := p.list(context.Background())
	require.NoError(t, err)
	assert.Empty(t, services)
}

func TestSCMProviderShutdownWithoutStart(t *testing.T) {
	p := newTestSCMProvider(t)

	require.NoError(t, p.shutdown(context.Background()))
}

func TestSCMProviderOpenServiceEmptyName(t *testing.T) {
	p := newTestSCMProvider(t)

	_, err := p.openService("")
	require.ErrorIs(t, err, windows.ERROR_INVALID_PARAMETER)
}

// Reading the state of the Service Control Manager itself needs no elevation,
// so a real end to end query is exercised when one can be opened.
func TestSCMProviderListReal(t *testing.T) {
	p := newTestSCMProvider(t)

	require.NoError(t, p.start(context.Background()))
	t.Cleanup(func() { require.NoError(t, p.shutdown(context.Background())) })

	if p.disabled {
		t.Skip("no access to the Service Control Manager")
	}

	services, err := p.list(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, services)

	for _, service := range services {
		assert.NotEmpty(t, service.Name)
		assert.Equal(t, service.Name, service.NativeName)
	}
}
