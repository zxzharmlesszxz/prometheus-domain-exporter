package domain

import (
	framework "github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter"
	"github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter/featurekit"

	"github.com/zxzharmlesszxz/prometheus-domain-exporter/internal/domaincheck"
)

type Snapshot struct {
	domain      domaincheck.Snapshot
	CacheStats  featurekit.TTLCacheStats
	RDAPResult  framework.FileScrapeResult
	RDAPValid   bool
	WHOISResult framework.FileScrapeResult
	WHOISValid  bool
}
