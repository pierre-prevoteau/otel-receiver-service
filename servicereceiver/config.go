// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package servicereceiver // import "github.com/pierre-prevoteau/otel-receiver-service/servicereceiver"

import (
	"errors"
	"fmt"
	"path"

	"go.opentelemetry.io/collector/scraper/scraperhelper"
	"go.uber.org/multierr"

	"github.com/pierre-prevoteau/otel-receiver-service/servicereceiver/internal/metadata"
)

// Scopes of the systemd service manager to collect from.
const (
	scopeSystem = "system"
	scopeUser   = "user"
)

var errInvalidScope = errors.New(`"scope" must be one of "system" or "user"`)

// Config defines configuration for the system service receiver.
type Config struct {
	scraperhelper.ControllerConfig `mapstructure:",squash"`
	metadata.MetricsBuilderConfig  `mapstructure:",squash"`

	// IncludeServices lists glob patterns of service names to collect. An empty
	// list collects every service the host reports.
	IncludeServices []string `mapstructure:"include_services"`

	// ExcludeServices lists glob patterns of service names to skip. It is
	// applied after IncludeServices.
	ExcludeServices []string `mapstructure:"exclude_services"`

	// Scope selects the systemd service manager to collect from, either the
	// system manager or the calling user's manager. It has no effect on Windows.
	Scope string `mapstructure:"scope"`
}

// Validate checks that the receiver configuration is valid.
func (c Config) Validate() error {
	var err error

	if c.Scope != scopeSystem && c.Scope != scopeUser {
		err = multierr.Append(err, errInvalidScope)
	}

	err = multierr.Append(err, validatePatterns("include_services", c.IncludeServices))
	err = multierr.Append(err, validatePatterns("exclude_services", c.ExcludeServices))

	return err
}

func validatePatterns(field string, patterns []string) error {
	var err error

	for _, pattern := range patterns {
		if pattern == "" {
			err = multierr.Append(err, fmt.Errorf("%q contains an empty pattern", field))
			continue
		}
		if _, matchErr := path.Match(pattern, ""); matchErr != nil {
			err = multierr.Append(err, fmt.Errorf("%q contains invalid pattern %q: %w", field, pattern, matchErr))
		}
	}

	return err
}
