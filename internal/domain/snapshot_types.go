package domain

import "github.com/zxzharmlesszxz/prometheus-domain-exporter/internal/domaincheck"

type Snapshot struct {
	domain domaincheck.Snapshot
}
