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
	ConfigFile           string
	Targets              []string
	Timeout              time.Duration
	MaxConcurrentTargets int
}

const (
	DefaultRefreshInterval      = 5 * time.Minute
	DefaultTimeout              = domaincheck.DefaultTimeout
	DefaultMaxConcurrentTargets = domaincheck.DefaultMaxConcurrentTargets
)

var DefaultFeatureConfigFileName = "prometheus-domain-exporter.yml"

type featureConfigFile struct {
	Targets              []string `yaml:"targets"`
	Timeout              string   `yaml:"timeout"`
	MaxConcurrentTargets int      `yaml:"max_concurrent_targets"`
}

var featureConfigFlagSpecs = []featurekit.FeatureConfigFlagSpec[Config]{
	{
		Name:        "target",
		Help:        "Domain name to check for registration expiration; repeat for multiple domains",
		Placeholder: "example.com",
		Bind: func(flag *kingpin.FlagClause, config *Config) {
			flag.StringsVar(&config.Targets)
		},
	},
	{
		Name: "timeout",
		Help: "Timeout for each domain registration lookup (default: 10s)",
		Bind: func(flag *kingpin.FlagClause, config *Config) {
			flag.DurationVar(&config.Timeout)
		},
	},
	{
		Name:    "max-concurrent-targets",
		Help:    "Maximum number of domains checked at the same time",
		Default: fmt.Sprint(DefaultMaxConcurrentTargets),
		Bind: func(flag *kingpin.FlagClause, config *Config) {
			flag.IntVar(&config.MaxConcurrentTargets)
		},
	},
}

func NewDefaultConfig() Config {
	return Config{
		Timeout:              DefaultTimeout,
		MaxConcurrentTargets: DefaultMaxConcurrentTargets,
	}
}

func FeatureConfigFile(config *Config) *string {
	return &config.ConfigFile
}

func ValidateFeatureConfig(config Config) error {
	if _, _, err := domaincheck.ParseDomains(config.Targets); err != nil {
		return fmt.Errorf("parse domain: %w", err)
	}
	return nil
}

func FeatureRuntimeConfigEntries(_ featurekit.RuntimeConfigContext[Config], config Config) []any {
	return []any{
		"targets", config.Targets,
		"timeout", framework.NormalizeDuration(config.Timeout, DefaultTimeout),
		"max_concurrent_targets", normalizePositiveInt(config.MaxConcurrentTargets, DefaultMaxConcurrentTargets),
	}
}

func normalizePositiveInt(value int, fallback int) int {
	if value <= 0 {
		return fallback
	}
	return value
}

func ResolveFeatureConfig(featureName string, config Config) (Config, string, bool, error) {
	if config.Timeout < 0 {
		config.Timeout = 0
	}
	if config.MaxConcurrentTargets < 0 {
		config.MaxConcurrentTargets = 0
	}

	var fileConfig featureConfigFile
	cfgFile, loaded, err := featurekit.LoadFeatureConfigFile(featureName, config.ConfigFile, &fileConfig)
	if err != nil {
		return config, cfgFile, false, err
	}

	if loaded {
		if fileConfig.Timeout != "" && (config.Timeout == DefaultTimeout || config.Timeout <= 0) {
			t, err := time.ParseDuration(fileConfig.Timeout)
			if err != nil {
				return config, cfgFile, true, fmt.Errorf("parse timeout from %q: %w", cfgFile, err)
			}
			config.Timeout = t
		}
		if fileConfig.MaxConcurrentTargets > 0 && (config.MaxConcurrentTargets == DefaultMaxConcurrentTargets || config.MaxConcurrentTargets <= 0) {
			config.MaxConcurrentTargets = fileConfig.MaxConcurrentTargets
		}
		config.Targets = append(fileConfig.Targets, config.Targets...)
	}

	if config.Timeout <= 0 {
		config.Timeout = DefaultTimeout
	}
	if config.MaxConcurrentTargets <= 0 {
		config.MaxConcurrentTargets = DefaultMaxConcurrentTargets
	}

	parsed, _, err := domaincheck.ParseDomains(config.Targets)
	if err != nil {
		return config, cfgFile, loaded, fmt.Errorf("resolve domain: %w", err)
	}
	config.Targets = parsed
	return config, cfgFile, loaded, nil
}
