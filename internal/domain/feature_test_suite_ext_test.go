package domain

import (
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/zxzharmlesszxz/prometheus-domain-exporter/internal/domaincheck"
	"github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter/exportertest"
	"github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter/exportertest/featuretest"
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
							Success:    true,
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
			"--" + testFeatureName + ".domain=Example.COM.",
			"--" + testFeatureName + ".domain=example.net",
			"--" + testFeatureName + ".lookup-timeout=3s",
		},
		ContractRuntimeConfig: map[string]any{
			"lookup_timeout": 3 * time.Second,
			"domains":        []string{"example.com", "example.net"},
		},
		DefaultRuntimeConfig: map[string]any{
			"lookup_timeout": domaincheck.DefaultLookupTimeout,
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
	suite.Register("smoke_spec_includes_config_file", func(t *testing.T) { testSmokeSpecIncludesConfigFile(t, suite) })
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
					Success:    true,
				},
			},
		},
	}), testRefreshInterval, func() time.Time { return now })

	families := exportertest.RegisterAndGather(t, collector)
	labels := map[string]string{"domain": "example.com"}
	exportertest.AssertMetricValue(t, families, suite.MetricName(testFeatureName, "", metricDomainLookupSuccess), labels, 1)
	exportertest.AssertMetricValue(t, families, suite.MetricName(testFeatureName, "", metricDomainLookupTimestamp), labels, float64(now.Unix()))
	exportertest.AssertMetricValue(t, families, suite.MetricName(testFeatureName, "", metricDomainExpirationTimestamp), labels, float64(expiration.Unix()))
	exportertest.AssertMetricValue(t, families, suite.MetricName(testFeatureName, "", metricDomainExpirationRemaining), labels, expiration.Sub(now).Seconds())
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
					Success:    false,
					Err:        errors.New("rdap unavailable"),
				},
			},
			Err: errors.New("lookup example.com registration expiration: rdap unavailable"),
		},
	}), testRefreshInterval, func() time.Time { return now })

	families := exportertest.RegisterAndGather(t, collector)
	labels := map[string]string{"domain": "example.com"}
	exportertest.AssertMetricValue(t, families, suite.MetricName(testFeatureName, "", metricDomainLookupSuccess), labels, 0)
	exportertest.AssertMetricValue(t, families, suite.MetricName(testFeatureName, "", metricDomainLookupTimestamp), labels, float64(now.Unix()))
	if _, ok := exportertest.MetricValue(families, suite.MetricName(testFeatureName, "", metricDomainExpirationTimestamp), labels); ok {
		t.Fatal("expiration timestamp metric was emitted for failed lookup")
	}
	if _, ok := exportertest.MetricValue(families, suite.MetricName(testFeatureName, "", metricDomainExpirationRemaining), labels); ok {
		t.Fatal("expiration remaining metric was emitted for failed lookup")
	}
}

func testExporterReportsInvalidDomain(t *testing.T, suite *FeatureTestSuite) {
	exporter := suite.NewNamedFeature()
	suite.ParseFeatureFlags(t, exporter, []string{"--" + testFeatureName + ".domain=https://example.com"})

	if err := exporter.RegisterCollectors(suite.FeatureContext(), prometheus.NewRegistry()); err == nil {
		t.Fatal("RegisterCollectors() error = nil, want invalid domain error")
	}
}

func testExporterReportsInvalidDomainFromConfigFile(t *testing.T, suite *FeatureTestSuite) {
	exporter := suite.NewNamedFeature()
	suite.ParseFeatureFlags(t, exporter, []string{"--" + testFeatureName + ".config-file=" + suite.WriteConfig(t, `
domains:
  - https://example.com
`)})

	if err := exporter.RegisterCollectors(suite.FeatureContext(), prometheus.NewRegistry()); err == nil {
		t.Fatal("RegisterCollectors() error = nil, want invalid domain error")
	}
}

func testExporterRuntimeConfigNormalizesValues(t *testing.T, suite *FeatureTestSuite) {
	exporter := suite.NewNamedFeature()
	suite.ParseFeatureFlags(t, exporter, []string{
		"--" + testFeatureName + ".domain=Example.COM.",
		"--" + testFeatureName + ".refresh-interval=0s",
		"--" + testFeatureName + ".lookup-timeout=0s",
	})

	config := exporter.RuntimeConfig()
	if got := exportertest.RuntimeConfigValue(t, config, "refresh_interval"); got != DefaultRefreshInterval {
		t.Fatalf("refresh_interval = %v, want %v", got, DefaultRefreshInterval)
	}
	if got := exportertest.RuntimeConfigValue(t, config, "lookup_timeout"); got != domaincheck.DefaultLookupTimeout {
		t.Fatalf("lookup_timeout = %v, want %v", got, domaincheck.DefaultLookupTimeout)
	}
	domains, ok := exportertest.RuntimeConfigValue(t, config, "domains").([]string)
	if !ok {
		t.Fatalf("domains runtime config has type %T, want []string", exportertest.RuntimeConfigValue(t, config, "domains"))
	}
	if !featuretest.HasString(domains, "example.com") {
		t.Fatalf("domains = %v, want normalized example.com", domains)
	}
}

func testExporterRuntimeConfigLoadsConfigFile(t *testing.T, suite *FeatureTestSuite) {
	exporter := suite.NewNamedFeature()
	suite.ParseFeatureFlags(t, exporter, []string{"--" + testFeatureName + ".config-file=" + suite.WriteConfig(t, `
domains:
  - Example.COM.
  - example.net
lookup_timeout: 3s
`)})

	config := exporter.RuntimeConfig()
	if got := exportertest.RuntimeConfigValue(t, config, "config_file_loaded"); got != true {
		t.Fatalf("config_file_loaded = %v, want true", got)
	}
	if got := exportertest.RuntimeConfigValue(t, config, "lookup_timeout"); got != 3*time.Second {
		t.Fatalf("lookup_timeout = %v, want %v", got, 3*time.Second)
	}
	domains, ok := exportertest.RuntimeConfigValue(t, config, "domains").([]string)
	if !ok {
		t.Fatalf("domains runtime config has type %T, want []string", exportertest.RuntimeConfigValue(t, config, "domains"))
	}
	if !featuretest.HasString(domains, "example.com") || !featuretest.HasString(domains, "example.net") {
		t.Fatalf("domains = %v, want config file domains", domains)
	}
}

func testExporterCLITimeoutDominatesConfigFile(t *testing.T, suite *FeatureTestSuite) {
	exporter := suite.NewNamedFeature()
	suite.ParseFeatureFlags(t, exporter, []string{
		"--" + testFeatureName + ".lookup-timeout=5s",
		"--" + testFeatureName + ".config-file=" + suite.WriteConfig(t, `
lookup_timeout: 30s
`)})

	config := exporter.RuntimeConfig()
	if got := exportertest.RuntimeConfigValue(t, config, "lookup_timeout"); got != 5*time.Second {
		t.Fatalf("lookup_timeout = %v, want 5s", got)
	}
}

func testSmokeSpecIncludesConfigFile(t *testing.T, suite *FeatureTestSuite) {
	spec := suite.NewNamedFeature().SmokeSpec()
	want := "--" + testFeatureName + ".config-file=" + DefaultFeatureConfigPath
	if !featuretest.HasString(spec.ServerArgs, want) {
		t.Fatalf("SmokeSpec().ServerArgs = %v, want %q", spec.ServerArgs, want)
	}
	if len(spec.WantMetrics) != 0 {
		t.Fatalf("SmokeSpec().WantMetrics = %v, want no domain-specific wanted metrics", spec.WantMetrics)
	}
	reject := suite.MetricName(testFeatureName, "", metricDomainLookupSuccess)
	if !featuretest.HasString(spec.RejectMetrics, reject) {
		t.Fatalf("SmokeSpec().RejectMetrics = %v, want %q", spec.RejectMetrics, reject)
	}
}
