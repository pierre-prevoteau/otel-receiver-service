// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package servicereceiver

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"

	"github.com/pierre-prevoteau/otel-receiver-service/servicereceiver/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	customExpected := createDefaultConfig().(*Config)
	customExpected.CollectionInterval = 30 * time.Second
	customExpected.IncludeServices = []string{"nginx", "sshd*"}
	customExpected.ExcludeServices = []string{"systemd-*"}
	customExpected.Scope = scopeUser

	tests := []struct {
		id       component.ID
		expected component.Config
	}{
		{
			id:       component.NewIDWithName(metadata.Type, ""),
			expected: createDefaultConfig(),
		},
		{
			id:       component.NewIDWithName(metadata.Type, "custom"),
			expected: customExpected,
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			cfg := createDefaultConfig()
			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestValidate(t *testing.T) {
	t.Run("default config is valid", func(t *testing.T) {
		require.NoError(t, createDefaultConfig().(*Config).Validate())
	})

	t.Run("user scope is valid", func(t *testing.T) {
		cfg := createDefaultConfig().(*Config)
		cfg.Scope = scopeUser
		require.NoError(t, cfg.Validate())
	})

	t.Run("unknown scope is rejected", func(t *testing.T) {
		cfg := createDefaultConfig().(*Config)
		cfg.Scope = "session"
		require.ErrorIs(t, cfg.Validate(), errInvalidScope)
	})

	t.Run("patterns are validated", func(t *testing.T) {
		cfg := createDefaultConfig().(*Config)
		cfg.IncludeServices = []string{"nginx", "sshd*", "[a-"}
		cfg.ExcludeServices = []string{""}

		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), `"include_services" contains invalid pattern "[a-"`)
		assert.Contains(t, err.Error(), `"exclude_services" contains an empty pattern`)
	})
}
