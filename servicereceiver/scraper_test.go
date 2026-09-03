// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package servicereceiver

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/multierr"

	"github.com/pierre-prevoteau/otel-receiver-service/servicereceiver/internal/metadata"
)

// writeGolden controls whether the expected golden file is (re)generated from
// the current scrape output. Run `WRITE_GOLDEN=true go test ./...` to refresh
// the testdata after intentionally changing the emitted metrics.
var writeGolden = os.Getenv("WRITE_GOLDEN") == "true"

var expectedMetricsFile = filepath.Join("testdata", "expected_metrics.yaml")

// sampleServices covers every state code, with Linux style unit names for the
// units that carry a ".service" suffix.
var sampleServices = []serviceInfo{
	{Name: "nginx", NativeName: "nginx.service", State: StateRunning},
	{Name: "cron", NativeName: "cron.service", State: StateStopped},
	{Name: "sshd", NativeName: "sshd.service", State: StateStarting},
	{Name: "postgresql", NativeName: "postgresql.service", State: StateStopping},
	{Name: "spooler", NativeName: "spooler", State: StatePaused},
	{Name: "systemd-timesyncd", NativeName: "systemd-timesyncd.service", State: StateFailed},
	{Name: "mystery", NativeName: "mystery", State: StateUnknown},
}

// fakeProvider stands in for the platform specific provider so the scraper can
// be exercised on any operating system.
type fakeProvider struct {
	services []serviceInfo
	listErr  error

	startErr    error
	shutdownErr error

	starts    int
	shutdowns int
}

func (p *fakeProvider) start(context.Context) error {
	p.starts++
	return p.startErr
}

func (p *fakeProvider) list(context.Context) ([]serviceInfo, error) {
	return p.services, p.listErr
}

func (p *fakeProvider) shutdown(context.Context) error {
	p.shutdowns++
	return p.shutdownErr
}

func newTestScraper(t *testing.T, provider *fakeProvider, configure func(*Config)) *serviceScraper {
	t.Helper()

	cfg := createDefaultConfig().(*Config)
	if configure != nil {
		configure(cfg)
	}
	require.NoError(t, cfg.Validate())

	s := newScraper(cfg, receivertest.NewNopSettings(metadata.Type))
	s.provider = provider
	return s
}

func requireMatchesGolden(t *testing.T, actual pmetric.Metrics) {
	t.Helper()

	expected, err := golden.ReadMetrics(expectedMetricsFile)
	require.NoError(t, err)

	require.NoError(t, pmetrictest.CompareMetrics(expected, actual,
		pmetrictest.IgnoreStartTimestamp(),
		pmetrictest.IgnoreTimestamp(),
		pmetrictest.IgnoreMetricDataPointsOrder(),
	))
}

// reportedServices returns the value of the system.service.state metric per
// service name.
func reportedServices(t *testing.T, m pmetric.Metrics) map[string]int64 {
	t.Helper()

	reported := map[string]int64{}
	for i := 0; i < m.ResourceMetrics().Len(); i++ {
		scopeMetrics := m.ResourceMetrics().At(i).ScopeMetrics()
		for j := 0; j < scopeMetrics.Len(); j++ {
			metrics := scopeMetrics.At(j).Metrics()
			for k := 0; k < metrics.Len(); k++ {
				metric := metrics.At(k)
				require.Equal(t, "system.service.state", metric.Name())

				points := metric.Gauge().DataPoints()
				for l := 0; l < points.Len(); l++ {
					point := points.At(l)
					name, ok := point.Attributes().Get("system.service.name")
					require.True(t, ok)
					reported[name.Str()] = point.IntValue()
				}
			}
		}
	}

	return reported
}

func TestScrape(t *testing.T) {
	s := newTestScraper(t, &fakeProvider{services: sampleServices}, nil)

	actual, err := s.scrape(context.Background())
	require.NoError(t, err)

	if writeGolden {
		require.NoError(t, golden.WriteMetrics(t, expectedMetricsFile, actual))
	}

	requireMatchesGolden(t, actual)
}

func TestScrapeNoServices(t *testing.T) {
	s := newTestScraper(t, &fakeProvider{}, nil)

	m, err := s.scrape(context.Background())
	require.NoError(t, err)
	require.Equal(t, 0, m.MetricCount())
}

func TestScrapeListError(t *testing.T) {
	wantErr := errors.New("no D-Bus connection")
	s := newTestScraper(t, &fakeProvider{listErr: wantErr}, nil)

	_, err := s.scrape(context.Background())
	require.ErrorIs(t, err, wantErr)
}

func TestScrapePartialListError(t *testing.T) {
	// Two services could not be read; the ones that could are still reported.
	partial := multierr.Combine(errors.New("service a: access denied"), errors.New("service b: access denied"))
	s := newTestScraper(t, &fakeProvider{services: sampleServices, listErr: partial}, nil)

	m, err := s.scrape(context.Background())
	require.Error(t, err)
	assert.Len(t, reportedServices(t, m), len(sampleServices))
}

func TestScrapeFiltering(t *testing.T) {
	for _, tt := range []struct {
		name      string
		configure func(*Config)
		expected  []string
	}{
		{
			name:      "no filters reports everything",
			configure: nil,
			expected:  []string{"nginx", "cron", "sshd", "postgresql", "spooler", "systemd-timesyncd", "mystery"},
		},
		{
			name:      "exact include",
			configure: func(c *Config) { c.IncludeServices = []string{"nginx", "cron"} },
			expected:  []string{"nginx", "cron"},
		},
		{
			name:      "include matches the full unit name too",
			configure: func(c *Config) { c.IncludeServices = []string{"nginx.service"} },
			expected:  []string{"nginx"},
		},
		{
			name:      "glob include",
			configure: func(c *Config) { c.IncludeServices = []string{"s*"} },
			expected:  []string{"sshd", "spooler", "systemd-timesyncd"},
		},
		{
			name:      "exclude only",
			configure: func(c *Config) { c.ExcludeServices = []string{"systemd-*", "mystery"} },
			expected:  []string{"nginx", "cron", "sshd", "postgresql", "spooler"},
		},
		{
			name: "exclude wins over include",
			configure: func(c *Config) {
				c.IncludeServices = []string{"s*"}
				c.ExcludeServices = []string{"systemd-*"}
			},
			expected: []string{"sshd", "spooler"},
		},
		{
			name:      "include matching nothing reports nothing",
			configure: func(c *Config) { c.IncludeServices = []string{"absent"} },
			expected:  []string{},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s := newTestScraper(t, &fakeProvider{services: sampleServices}, tt.configure)

			m, err := s.scrape(context.Background())
			require.NoError(t, err)

			reported := reportedServices(t, m)
			names := make([]string, 0, len(reported))
			for name := range reported {
				names = append(names, name)
			}
			assert.ElementsMatch(t, tt.expected, names)
		})
	}
}

// The generated lifecycle test is skipped because it would need a live systemd
// or Service Control Manager connection, so the lifecycle is covered here with
// the fake provider instead.
func TestScraperLifecycle(t *testing.T) {
	provider := &fakeProvider{services: sampleServices}
	s := newTestScraper(t, provider, nil)

	for range 2 {
		require.NoError(t, s.start(context.Background(), componenttest.NewNopHost()))
		_, err := s.scrape(context.Background())
		require.NoError(t, err)
		require.NoError(t, s.shutdown(context.Background()))
	}

	assert.Equal(t, 2, provider.starts)
	assert.Equal(t, 2, provider.shutdowns)
}

func TestScraperStartError(t *testing.T) {
	wantErr := errors.New("failed to connect")
	s := newTestScraper(t, &fakeProvider{startErr: wantErr}, nil)

	require.ErrorIs(t, s.start(context.Background(), componenttest.NewNopHost()), wantErr)
}

func TestScraperShutdownError(t *testing.T) {
	wantErr := errors.New("failed to close")
	s := newTestScraper(t, &fakeProvider{shutdownErr: wantErr}, nil)

	require.ErrorIs(t, s.shutdown(context.Background()), wantErr)
}
