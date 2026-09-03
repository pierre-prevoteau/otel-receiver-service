// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package servicereceiver

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/pierre-prevoteau/otel-receiver-service/servicereceiver/internal/metadata"
)

func TestNewFactory(t *testing.T) {
	factory := NewFactory()
	assert.Equal(t, metadata.Type, factory.Type())
}

func TestCreateDefaultConfig(t *testing.T) {
	cfg, ok := createDefaultConfig().(*Config)
	require.True(t, ok)

	assert.Equal(t, scopeSystem, cfg.Scope)
	assert.Empty(t, cfg.IncludeServices)
	assert.Empty(t, cfg.ExcludeServices)
	assert.Equal(t, 60*time.Second, cfg.CollectionInterval)
	assert.NoError(t, cfg.Validate())
}

func TestCreateMetrics(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	r, err := factory.CreateMetrics(
		context.Background(),
		receivertest.NewNopSettings(metadata.Type),
		cfg,
		consumertest.NewNop(),
	)
	require.NoError(t, err)
	require.NotNil(t, r)
	require.NoError(t, r.Shutdown(context.Background()))
}

func TestCreateMetricsInvalidConfig(t *testing.T) {
	factory := NewFactory()
	_, err := factory.CreateMetrics(
		context.Background(),
		receivertest.NewNopSettings(metadata.Type),
		nil,
		consumertest.NewNop(),
	)
	require.ErrorIs(t, err, errConfigNotValid)
}
