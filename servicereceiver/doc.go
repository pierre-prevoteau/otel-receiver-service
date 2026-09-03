// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate mdatagen metadata.yaml

// Package servicereceiver implements an OpenTelemetry receiver that reports the
// state of services running on the local host, using systemd on Linux and the
// Service Control Manager on Windows.
package servicereceiver // import "github.com/pierre-prevoteau/otel-receiver-service/servicereceiver"
