package domain

import (
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	framework "github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter"
	"github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter/exportertest"
	"github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter/exportertest/featuretest"
	"github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter/featurekit"

	"github.com/zxzharmlesszxz/prometheus-domain-exporter/internal/domaincheck"
)

func TestFeatureContract(t *testing.T) {
	suite := NewFeatureTestSuite(NewFeatureTestSpec())
	RegisterFeatureTests(suite)
	suite.RunTests(t)
}

func NewFeatureTestSpec() FeatureTestSpec {
	return FeatureTestSpec{
		SuccessfulSnapshot: func(at time.Time) Snapshot {
			return Snapshot{
				domain: domaincheck.Snapshot{
					AttemptTime: at,
					Success:     true,
					Domains: []domaincheck.Result{
						{
							Name:       "example.com",
							LookupTime: at,
							Expiration: at.Add(24 * time.Hour),
							Source:     domaincheck.SourceRDAP,
							Success:    true,
							Verified:   true,
						},
					},
				},
			}
		},
		FailedSnapshot: func(at time.Time, err error) Snapshot {
			return Snapshot{
				domain: domaincheck.Snapshot{
					AttemptTime: at,
					Success:     false,
					Err:         err,
				},
			}
		},
		ContractFlagArgs: []string{
			"--" + testFeatureName + ".target=Example.COM.",
			"--" + testFeatureName + ".target=example.net",
			"--" + testFeatureName + ".timeout=3s",
		},
		ContractRuntimeConfig: map[string]any{
			"timeout": 3 * time.Second,
			"targets": []string{"example.com", "example.net"},
		},
		DefaultRuntimeConfig: map[string]any{
			"timeout": domaincheck.DefaultTimeout,
		},
		CheckDefaultSnapshotter: true,
	}
}

func RegisterFeatureTests(suite *FeatureTestSuite) {
	suite.Register("collector_exports_snapshot", func(t *testing.T) { testCollectorExportsSnapshot(t, suite) })
	suite.Register("collector_exports_failed_domain_lookup", func(t *testing.T) { testCollectorExportsFailedDomainLookup(t, suite) })
	suite.Register("exporter_reports_invalid_domain", func(t *testing.T) { testExporterReportsInvalidDomain(t, suite) })
	suite.Register("exporter_reports_invalid_domain_from_config_file", func(t *testing.T) { testExporterReportsInvalidDomainFromConfigFile(t, suite) })
	suite.Register("exporter_runtime_config_normalizes_values", func(t *testing.T) { testExporterRuntimeConfigNormalizesValues(t, suite) })
	suite.Register("exporter_runtime_config_loads_config_file", func(t *testing.T) { testExporterRuntimeConfigLoadsConfigFile(t, suite) })
	suite.Register("exporter_cli_timeout_dominates_config_file", func(t *testing.T) { testExporterCLITimeoutDominatesConfigFile(t, suite) })
}

func testCollectorExportsSnapshot(t *testing.T, suite *FeatureTestSuite) {
	now := time.Unix(1_700_000_000, 0)
	expiration := now.Add(45 * 24 * time.Hour)
	collector := suite.NewCollectorWithNow(testFeatureName, testMetricNamespace, slog.New(slog.NewTextHandler(io.Discard, nil)), suite.NewFakeSnapshotter(Snapshot{
		domain: domaincheck.Snapshot{
			AttemptTime: now,
			Success:     true,
			Domains: []domaincheck.Result{
				{
					Name:       "example.com",
					LookupTime: now,
					Expiration: expiration,
					Source:     domaincheck.SourceRDAP,
					Success:    true,
					Verified:   true,
				},
				{
					Name:       "example.ws",
					LookupTime: now,
					Expiration: expiration,
					Source:     domaincheck.SourceWHOIS,
					Success:    true,
					Verified:   true,
				},
			},
		},
		RDAPResult: framework.FileScrapeResult{
			Path:                  "rdap",
			Up:                    true,
			ReadErrorsTotal:       0,
			ParseErrorsTotal:      0,
			ScrapeDurationSeconds: 0.25,
		},
		RDAPValid: true,
		WHOISResult: framework.FileScrapeResult{
			Path:                  "whois",
			Up:                    true,
			ReadErrorsTotal:       0,
			ParseErrorsTotal:      0,
			ScrapeDurationSeconds: 0.25,
		},
		WHOISValid: true,
	}), testRefreshInterval, func() time.Time { return now })

	families := exportertest.RegisterAndGather(t, collector)
	labels := map[string]string{"domain": "example.com"}
	exportertest.AssertMetricValue(t, families, suite.MetricName(testFeatureName, "", metricDomainLookupSuccess), labels, 1)
	exportertest.AssertMetricValue(t, families, suite.MetricName(testFeatureName, "", metricDomainLookupTimestamp), labels, float64(now.Unix()))
	exportertest.AssertMetricValue(t, families, suite.MetricName(testFeatureName, "", metricDomainExpirationTimestamp), labels, float64(expiration.Unix()))
	exportertest.AssertMetricValue(t, families, suite.MetricName(testFeatureName, "", metricDomainExpirationRemaining), labels, expiration.Sub(now).Seconds())
	for _, source := range []struct {
		name      string
		metricIDs featurekit.FileScrapeMetricIDs
	}{
		{name: domaincheck.SourceRDAP, metricIDs: rdapMetricIDs},
		{name: domaincheck.SourceWHOIS, metricIDs: whoisMetricIDs},
	} {
		sourceLabels := map[string]string{"source": source.name}
		exportertest.AssertMetricValue(t, families, suite.MetricName(testFeatureName, "", source.metricIDs.Up), sourceLabels, 1)
		exportertest.AssertMetricValue(t, families, suite.MetricName(testFeatureName, "", source.metricIDs.Valid), sourceLabels, 1)
		exportertest.AssertMetricValue(t, families, suite.MetricName(testFeatureName, "", source.metricIDs.ReadErrorsTotal), sourceLabels, 0)
		exportertest.AssertMetricValue(t, families, suite.MetricName(testFeatureName, "", source.metricIDs.ParseErrorsTotal), sourceLabels, 0)
	}
	exportertest.AssertMetricValue(t, families, testLastSuccess, nil, 1)
	exportertest.AssertMetricValue(t, families, testLastTimestamp, nil, float64(now.Unix()))
	exportertest.AssertMetricValue(t, families, testLastSuccessfulTS, nil, float64(now.Unix()))
}

func testCollectorExportsFailedDomainLookup(t *testing.T, suite *FeatureTestSuite) {
	now := time.Unix(1_700_000_000, 0)
	collector := suite.NewCollectorWithNow(testFeatureName, testMetricNamespace, slog.New(slog.NewTextHandler(io.Discard, nil)), suite.NewFakeSnapshotter(Snapshot{
		domain: domaincheck.Snapshot{
			AttemptTime: now,
			Success:     false,
			Domains: []domaincheck.Result{
				{
					Name:       "example.com",
					LookupTime: now,
					Source:     domaincheck.SourceRDAP,
					Success:    false,
					Err:        errors.New("rdap unavailable"),
				},
			},
			Err: errors.New("lookup example.com registration expiration: rdap unavailable"),
		},
		RDAPResult: framework.FileScrapeResult{
			Path:                  "rdap",
			Up:                    false,
			ReadErrorsTotal:       1,
			ParseErrorsTotal:      0,
			ScrapeDurationSeconds: 0.25,
		},
		RDAPValid: false,
	}), testRefreshInterval, func() time.Time { return now })

	families := exportertest.RegisterAndGather(t, collector)
	labels := map[string]string{"domain": "example.com"}
	exportertest.AssertMetricValue(t, families, suite.MetricName(testFeatureName, "", metricDomainLookupSuccess), labels, 0)
	exportertest.AssertMetricValue(t, families, suite.MetricName(testFeatureName, "", metricDomainLookupTimestamp), labels, float64(now.Unix()))
	sourceLabels := map[string]string{"source": "rdap"}
	exportertest.AssertMetricValue(t, families, suite.MetricName(testFeatureName, "", rdapMetricIDs.Up), sourceLabels, 0)
	exportertest.AssertMetricValue(t, families, suite.MetricName(testFeatureName, "", rdapMetricIDs.Valid), sourceLabels, 0)
	exportertest.AssertMetricValue(t, families, suite.MetricName(testFeatureName, "", rdapMetricIDs.ReadErrorsTotal), sourceLabels, 1)
	if _, ok := exportertest.MetricValue(families, suite.MetricName(testFeatureName, "", metricDomainExpirationTimestamp), labels); ok {
		t.Fatal("expiration timestamp metric was emitted for failed lookup")
	}
	if _, ok := exportertest.MetricValue(families, suite.MetricName(testFeatureName, "", metricDomainExpirationRemaining), labels); ok {
		t.Fatal("expiration remaining metric was emitted for failed lookup")
	}
}

func testExporterReportsInvalidDomain(t *testing.T, suite *FeatureTestSuite) {
	exporter := suite.NewNamedFeature()
	suite.ParseFeatureFlags(t, exporter, []string{"--" + testFeatureName + ".target=https://example.com"})

	if err := exporter.RegisterCollectors(suite.FeatureContext(), prometheus.NewRegistry()); err == nil {
		t.Fatal("RegisterCollectors() error = nil, want invalid domain error")
	}
}

func testExporterReportsInvalidDomainFromConfigFile(t *testing.T, suite *FeatureTestSuite) {
	exporter := suite.NewNamedFeature()
	suite.ParseFeatureFlags(t, exporter, []string{"--" + testFeatureName + ".config-file=" + suite.WriteConfig(t, `
targets:
  - https://example.com
`)})

	if err := exporter.RegisterCollectors(suite.FeatureContext(), prometheus.NewRegistry()); err == nil {
		t.Fatal("RegisterCollectors() error = nil, want invalid domain error")
	}
}

func testExporterRuntimeConfigNormalizesValues(t *testing.T, suite *FeatureTestSuite) {
	exporter := suite.NewNamedFeature()
	suite.ParseFeatureFlags(t, exporter, []string{
		"--" + testFeatureName + ".target=Example.COM.",
		"--" + testFeatureName + ".refresh-interval=0s",
		"--" + testFeatureName + ".timeout=0s",
	})

	config := exporter.RuntimeConfig()
	if got := exportertest.RuntimeConfigValue(t, config, "refresh_interval"); got != DefaultRefreshInterval {
		t.Fatalf("refresh_interval = %v, want %v", got, DefaultRefreshInterval)
	}
	if got := exportertest.RuntimeConfigValue(t, config, "timeout"); got != domaincheck.DefaultTimeout {
		t.Fatalf("timeout = %v, want %v", got, domaincheck.DefaultTimeout)
	}
	targets, ok := exportertest.RuntimeConfigValue(t, config, "targets").([]string)
	if !ok {
		t.Fatalf("targets runtime config has type %T, want []string", exportertest.RuntimeConfigValue(t, config, "targets"))
	}
	if !featuretest.HasString(targets, "example.com") {
		t.Fatalf("targets = %v, want normalized example.com", targets)
	}
}

func testExporterRuntimeConfigLoadsConfigFile(t *testing.T, suite *FeatureTestSuite) {
	exporter := suite.NewNamedFeature()
	suite.ParseFeatureFlags(t, exporter, []string{"--" + testFeatureName + ".config-file=" + suite.WriteConfig(t, `
targets:
  - Example.COM.
  - example.net
timeout: 3s
`)})

	config := exporter.RuntimeConfig()
	if got := exportertest.RuntimeConfigValue(t, config, "config_file_loaded"); got != true {
		t.Fatalf("config_file_loaded = %v, want true", got)
	}
	if got := exportertest.RuntimeConfigValue(t, config, "timeout"); got != 3*time.Second {
		t.Fatalf("timeout = %v, want %v", got, 3*time.Second)
	}
	targets, ok := exportertest.RuntimeConfigValue(t, config, "targets").([]string)
	if !ok {
		t.Fatalf("targets runtime config has type %T, want []string", exportertest.RuntimeConfigValue(t, config, "targets"))
	}
	if !featuretest.HasString(targets, "example.com") || !featuretest.HasString(targets, "example.net") {
		t.Fatalf("targets = %v, want config file targets", targets)
	}
}

func testExporterCLITimeoutDominatesConfigFile(t *testing.T, suite *FeatureTestSuite) {
	exporter := suite.NewNamedFeature()
	suite.ParseFeatureFlags(t, exporter, []string{
		"--" + testFeatureName + ".timeout=5s",
		"--" + testFeatureName + ".config-file=" + suite.WriteConfig(t, `
timeout: 30s
`)})

	config := exporter.RuntimeConfig()
	if got := exportertest.RuntimeConfigValue(t, config, "timeout"); got != 5*time.Second {
		t.Fatalf("timeout = %v, want 5s", got)
	}
}
