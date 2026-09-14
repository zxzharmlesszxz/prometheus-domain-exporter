package domain

import (
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
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
			expiration := at.Add(24 * time.Hour)
			return Snapshot{
				domain: domaincheck.Snapshot{
					AttemptTime: at,
					Success:     true,
					Domains:     []domaincheck.Result{successfulDomainResult("example.com", domaincheck.SourceRDAP, at, expiration)},
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

func successfulDomainResult(name, source string, at, expiration time.Time) domaincheck.Result {
	return domaincheck.Result{
		Name:       name,
		LookupTime: at,
		Expiration: expiration,
		Source:     source,
		Success:    true,
		Verified:   true,
		LastKnownGood: featurekit.LastKnownGoodResult[domaincheck.RegistrationData]{
			Value: domaincheck.RegistrationData{
				Expiration: expiration,
			},
			Available:   true,
			LastSuccess: at,
			LastAttempt: at,
		},
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
				successfulDomainResult("example.com", domaincheck.SourceRDAP, now, expiration),
				successfulDomainResult("example.ws", domaincheck.SourceWHOIS, now, expiration),
			},
		},
		CacheStats: featurekit.TTLCacheStats{
			Entries: 2,
			Hits:    11,
			Misses:  2,
			Sets:    4,
			Deletes: 1,
			Expired: 3,
			Clears:  1,
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
	rdapLabels := map[string]string{"domain": "example.com", "source": domaincheck.SourceRDAP}
	whoisLabels := map[string]string{"domain": "example.ws", "source": domaincheck.SourceWHOIS}
	exportertest.AssertMetricValue(t, families, suite.MetricName(testFeatureName, "", metricDomainLookupSuccess), labels, 1)
	exportertest.AssertMetricValue(t, families, suite.MetricName(testFeatureName, "", metricDomainLookupSourceInfo), rdapLabels, 1)
	exportertest.AssertMetricValue(t, families, suite.MetricName(testFeatureName, "", metricDomainLookupSourceInfo), whoisLabels, 1)
	exportertest.AssertMetricValue(t, families, suite.MetricName(testFeatureName, "", metricDomainLookupTimestamp), labels, float64(now.Unix()))
	assertRegistrationDataMetrics(t, suite, families, labels, expiration, now, now)
	exportertest.AssertMetricValue(t, families, suite.MetricName(testFeatureName, "", registrationLastKnownGoodMetricIDs.ConsecutiveFailures), labels, 0)
	exportertest.AssertMetricValue(t, families, suite.MetricName(testFeatureName, "", registrationLastKnownGoodMetricIDs.DataAvailable), labels, 1)
	exportertest.AssertMetricValue(t, families, suite.MetricName(testFeatureName, "", registrationLastKnownGoodMetricIDs.DataStale), labels, 0)
	cacheLabels := map[string]string{"cache": registrationCache}
	for metric, want := range map[string]float64{
		featurekit.TTLCacheMetricEntries: 2,
		featurekit.TTLCacheMetricHits:    11,
		featurekit.TTLCacheMetricMisses:  2,
		featurekit.TTLCacheMetricSets:    4,
		featurekit.TTLCacheMetricDeletes: 1,
		featurekit.TTLCacheMetricExpired: 3,
		featurekit.TTLCacheMetricClears:  1,
	} {
		exportertest.AssertMetricValue(t, families, suite.MetricName(testFeatureName, "", metric), cacheLabels, want)
	}
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
		metricName := suite.MetricName(testFeatureName, "", source.metricIDs.MTimeSeconds)
		if got := exportertest.MetricFamily(t, families, metricName).GetHelp(); got != "Unix timestamp represented by the current "+source.name+" source-health snapshot." {
			t.Fatalf("%s help = %q", metricName, got)
		}
	}
	exportertest.AssertMetricValue(t, families, testLastSuccess, nil, 1)
	exportertest.AssertMetricValue(t, families, testLastTimestamp, nil, float64(now.Unix()))
	exportertest.AssertMetricValue(t, families, testLastSuccessfulTS, nil, float64(now.Unix()))
}

func testCollectorExportsFailedDomainLookup(t *testing.T, suite *FeatureTestSuite) {
	now := time.Unix(1_700_000_000, 0)
	expiration := now.Add(45 * 24 * time.Hour)
	lastSuccess := now.Add(-2 * time.Hour)
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
					LastKnownGood: featurekit.LastKnownGoodResult[domaincheck.RegistrationData]{
						Value: domaincheck.RegistrationData{
							Expiration: expiration,
						},
						Available:           true,
						LastSuccess:         lastSuccess,
						LastAttempt:         now,
						ConsecutiveFailures: 2,
					},
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
	exportertest.AssertMetricValue(t, families, suite.MetricName(testFeatureName, "", metricDomainLookupSourceInfo), map[string]string{"domain": "example.com", "source": domaincheck.SourceRDAP}, 1)
	exportertest.AssertMetricValue(t, families, suite.MetricName(testFeatureName, "", metricDomainLookupTimestamp), labels, float64(now.Unix()))
	sourceLabels := map[string]string{"source": "rdap"}
	exportertest.AssertMetricValue(t, families, suite.MetricName(testFeatureName, "", rdapMetricIDs.Up), sourceLabels, 0)
	exportertest.AssertMetricValue(t, families, suite.MetricName(testFeatureName, "", rdapMetricIDs.Valid), sourceLabels, 0)
	exportertest.AssertMetricValue(t, families, suite.MetricName(testFeatureName, "", rdapMetricIDs.ReadErrorsTotal), sourceLabels, 1)
	assertRegistrationDataMetrics(t, suite, families, labels, expiration, now, lastSuccess)
	exportertest.AssertMetricValue(t, families, suite.MetricName(testFeatureName, "", registrationLastKnownGoodMetricIDs.ConsecutiveFailures), labels, 2)
	exportertest.AssertMetricValue(t, families, suite.MetricName(testFeatureName, "", registrationLastKnownGoodMetricIDs.DataAvailable), labels, 1)
	exportertest.AssertMetricValue(t, families, suite.MetricName(testFeatureName, "", registrationLastKnownGoodMetricIDs.DataStale), labels, 0)
}

func assertRegistrationDataMetrics(t *testing.T, suite *FeatureTestSuite, families []*dto.MetricFamily, labels map[string]string, expiration, now, lastSuccess time.Time) {
	t.Helper()
	for metric, want := range map[string]float64{
		metricDomainExpirationTimestamp:                                float64(expiration.Unix()),
		metricDomainExpirationRemaining:                                expiration.Sub(now).Seconds(),
		registrationLastKnownGoodMetricIDs.LastSuccessTimestampSeconds: float64(lastSuccess.Unix()),
	} {
		exportertest.AssertMetricValue(t, families, suite.MetricName(testFeatureName, "", metric), labels, want)
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
