package config

import (
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	TMDB     TMDB     `json:"tmdb" yaml:"tmdb" mapstructure:"tmdb"`
	Prowlarr Prowlarr `json:"prowlarr" yaml:"prowlarr" mapstructure:"prowlarr"`
	Library  Library  `json:"library" yaml:"library" mapstructure:"library"`
	Storage  Storage  `json:"storage" yaml:"storage" mapstructure:"storage"`
	Server   Server   `json:"server" yaml:"server" mapstructure:"server"`
	Manager  Manager  `json:"manager" yaml:"manager" mapstructure:"manager"`
}

type TMDB struct {
	Scheme      string        `json:"scheme" yaml:"scheme" mapstructure:"scheme"`
	Host        string        `json:"host" yaml:"host" mapstructure:"host"`
	APIKey      string        `json:"apiKey" yaml:"apiKey" mapstructure:"apiKey"`
	BaseBackoff time.Duration `json:"backoff" yaml:"backoff" mapstructure:"backoff"`
	MaxRetries  int           `json:"maxRetries" yaml:"maxRetries" mapstructure:"maxRetries"`
}

type Server struct {
	Port int `json:"port" yaml:"port" mapstructure:"port"`
}

type Library struct {
	MovieDir         string `json:"movie" yaml:"movie" mapstructure:"movie"`
	TVDir            string `json:"tv" yaml:"tv" mapstructure:"tv"`
	DownloadMountDir string `json:"downloadMountDir" yaml:"downloadMountDir" mapstructure:"downloadMountDir"`
}

type Prowlarr struct {
	Scheme string `json:"scheme" yaml:"scheme" mapstructure:"scheme"`
	Host   string `json:"host" yaml:"host" mapstructure:"host"`
	APIKey string `json:"apiKey" yaml:"apiKey" mapstructure:"apiKey"`
}

// Storage configuration is assumed to be for sqlite database only currently
type Storage struct {
	FilePath          string   `json:"filePath" yaml:"filePath" mapstructure:"filePath"`
	Schemas           []string `json:"schemas"  yaml:"schemas" mapstructure:"schemas"`
	TableValueSchemas []string `json:"tableValueSchemas" yaml:"tableValueSchemas" mapstructure:"tableValueSchemas"`
}

// Manager houses configuration related to the manager and reconcillation
type Manager struct {
	Jobs Jobs `json:"jobs" yaml:"jobs" mapstructure:"jobs"`
}

type Jobs struct {
	MovieReconcile  time.Duration `json:"movieReconcile" yaml:"movieReconcile" mapstructure:"movieReconcile"`
	MovieIndex      time.Duration `json:"movieIndex" yaml:"movieIndex" mapstructure:"movieIndex"`
	SeriesReconcile time.Duration `json:"seriesReconcile" yaml:"seriesReconcile" mapstructure:"seriesReconcile"`
	SeriesIndex     time.Duration `json:"seriesIndex" yaml:"seriesIndex" mapstructure:"seriesIndex"`
}

type ConfigUnmarshaler interface {
	ReadInConfig() error
	Unmarshal(any, ...viper.DecoderConfigOption) error
	ConfigFileUsed() string
}

// New reads a new configuration
func New(cu ConfigUnmarshaler) (Config, error) {
	var c Config

	if cu.ConfigFileUsed() != "" {
		err := cu.ReadInConfig()
		if err != nil {
			return c, err
		}
	}

	err := cu.Unmarshal(&c)
	return c, err
}
