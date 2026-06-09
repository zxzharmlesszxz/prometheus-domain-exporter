package domain

import (
	"fmt"
	"time"

	"github.com/alecthomas/kingpin/v2"
	framework "github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter"
	"github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter/featurekit"

	"github.com/zxzharmlesszxz/prometheus-domain-exporter/internal/domaincheck"
)

type Config struct {
	ConfigFile    string
	LookupTimeout time.Duration
	Domains       []string
}

const (
	DefaultRefreshInterval = 5 * time.Minute
	DefaultLookupTimeout   = domaincheck.DefaultLookupTimeout
)

var DefaultFeatureConfigFileName = "prometheus-domain-exporter.yml"
var DefaultFeatureConfigPath = "../examples/" + DefaultFeatureConfigFileName

type featureConfigFile struct {
	Domains       []string `yaml:"domains"`
	LookupTimeout string   `yaml:"lookup_timeout"`
}

var featureConfigFlagSpecs = []featurekit.FeatureConfigFlagSpec[Config]{
	{
		Name:        "domain",
		Help:        "Domain name to check for registration expiration; repeat for multiple domains",
		Placeholder: "example.com",
		Bind: func(flag *kingpin.FlagClause, config *Config) {
			flag.StringsVar(&config.Domains)
		},
	},
	{
		Name: "lookup-timeout",
		Help: "Timeout for each domain registration lookup (default: 10s)",
		Bind: func(flag *kingpin.FlagClause, config *Config) {
			flag.DurationVar(&config.LookupTimeout)
		},
	},
}

func NewDefaultConfig() Config {
	return Config{}
}

func FeatureConfigFile(config *Config) *string {
	return &config.ConfigFile
}

func ValidateFeatureConfig(config Config) error {
	if _, _, err := domaincheck.ParseDomains(config.Domains); err != nil {
		return fmt.Errorf("parse domain: %w", err)
	}
	return nil
}

func FeatureRuntimeConfigEntries(_ featurekit.RuntimeConfigContext[Config], config Config) []any {
	return []any{
		"lookup_timeout", framework.NormalizeDuration(config.LookupTimeout, DefaultLookupTimeout),
		"domains", config.Domains,
	}
}

func ResolveFeatureConfig(featureName string, config Config) (Config, string, bool, error) {
	if config.LookupTimeout < 0 {
		config.LookupTimeout = 0
	}

	var fileConfig featureConfigFile
	cfgFile, loaded, err := featurekit.LoadFeatureConfigFile(featureName, config.ConfigFile, &fileConfig)
	if err != nil {
		return config, cfgFile, false, err
	}

	if loaded {
		if fileConfig.LookupTimeout != "" && (config.LookupTimeout == 0) {
			lookupTimeout, err := time.ParseDuration(fileConfig.LookupTimeout)
			if err != nil {
				return config, cfgFile, true, fmt.Errorf("parse lookup_timeout from %q: %w", cfgFile, err)
			}
			config.LookupTimeout = lookupTimeout
		}
		config.Domains = append(fileConfig.Domains, config.Domains...)
	}

	if config.LookupTimeout <= 0 {
		config.LookupTimeout = DefaultLookupTimeout
	}

	parsed, _, err := domaincheck.ParseDomains(config.Domains)
	if err != nil {
		return config, cfgFile, loaded, fmt.Errorf("resolve domain: %w", err)
	}
	config.Domains = parsed
	return config, cfgFile, loaded, nil
}
