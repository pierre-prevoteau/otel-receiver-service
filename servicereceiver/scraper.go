// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package servicereceiver // import "github.com/pierre-prevoteau/otel-receiver-service/servicereceiver"

import (
	"context"
	"path"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/scraper/scrapererror"
	"go.uber.org/multierr"

	"github.com/pierre-prevoteau/otel-receiver-service/servicereceiver/internal/metadata"
)

type serviceScraper struct {
	cfg       *Config
	telemetry component.TelemetrySettings
	mb        *metadata.MetricsBuilder

	// provider reads the state of the host's services. It is a field so tests
	// can inject a fake in place of the platform implementation.
	provider serviceProvider
}

func newScraper(cfg *Config, settings receiver.Settings) *serviceScraper {
	return &serviceScraper{
		cfg:       cfg,
		telemetry: settings.TelemetrySettings,
		mb:        metadata.NewMetricsBuilder(cfg.MetricsBuilderConfig, settings),
		provider:  newProvider(cfg, settings.Logger),
	}
}

func (s *serviceScraper) start(ctx context.Context, _ component.Host) error {
	return s.provider.start(ctx)
}

func (s *serviceScraper) shutdown(ctx context.Context) error {
	return s.provider.shutdown(ctx)
}

func (s *serviceScraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
	services, err := s.provider.list(ctx)
	if err != nil && len(services) == 0 {
		return pmetric.NewMetrics(), err
	}

	var errs scrapererror.ScrapeErrors
	if err != nil {
		// Some services could not be read, the rest are still reported.
		errs.AddPartial(len(multierr.Errors(err)), err)
	}

	now := pcommon.NewTimestampFromTime(time.Now())
	for _, service := range services {
		if !s.included(service) {
			continue
		}
		s.mb.RecordSystemServiceStateDataPoint(now, int64(service.State), service.Name)
	}

	return s.mb.Emit(), errs.Combine()
}

func (s *serviceScraper) included(service serviceInfo) bool {
	if len(s.cfg.IncludeServices) > 0 && !matchesAny(s.cfg.IncludeServices, service) {
		return false
	}

	return !matchesAny(s.cfg.ExcludeServices, service)
}

// matchesAny reports whether any pattern matches the service. Both the
// normalized and the platform native name are tried, so that a Linux unit is
// matched by "nginx" as well as by "nginx.service". Patterns are validated by
// [Config.Validate], so match errors cannot occur here.
func matchesAny(patterns []string, service serviceInfo) bool {
	for _, pattern := range patterns {
		if matched, _ := path.Match(pattern, service.Name); matched {
			return true
		}
		if service.NativeName != service.Name {
			if matched, _ := path.Match(pattern, service.NativeName); matched {
				return true
			}
		}
	}

	return false
}
