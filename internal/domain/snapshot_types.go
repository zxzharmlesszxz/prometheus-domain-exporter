package domain

import (
	framework "github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter"

	"github.com/zxzharmlesszxz/prometheus-domain-exporter/internal/domaincheck"
)

type Snapshot struct {
	domain     domaincheck.Snapshot
	RDAPResult framework.FileScrapeResult
}
