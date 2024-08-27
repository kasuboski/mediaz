package config

import (
	"github.com/spf13/viper"
)

type Config struct {
	TMDB     TMDB     `json:"tmdb" yaml:"tmdb" mapstructure:"tmdb"`
	Prowlarr Prowlarr `json:"prowlarr" yaml:"prowlarr" mapstructure:"prowlarr"`
	Library  Library  `json:"library" yaml:"library" mapstructure:"library"`
	Storage  Storage  `json:"storage" yaml:"storage" mapstructure:"storage"`
	Server   Server   `json:"server" yaml:"server" mapstructure:"server"`
}

type TMDB struct {
	Scheme string `json:"scheme" yaml:"scheme" mapstructure:"scheme"`
	Host   string `json:"host" yaml:"host" mapstructure:"host"`
	APIKey string `json:"apiKey" yaml:"apiKey" mapstructure:"apiKey"`
}

type Server struct {
	Port int `json:"port" yaml:"port" mapstructure:"port"`
}

type Library struct {
	MovieDir string `json:"movie" yaml:"movie" mapstructure:"movie"`
	TVDir    string `json:"tv" yaml:"tv" mapstructure:"tv"`
}

type Prowlarr struct {
	Scheme string `json:"scheme" yaml:"scheme" mapstructure:"scheme"`
	Host   string `json:"host" yaml:"host" mapstructure:"host"`
	APIKey string `json:"apiKey" yaml:"apiKey" mapstructure:"apiKey"`
}

// Storage configuration is assumed to be for sqlite database only currently
type Storage struct {
	FilePath string `json:"filePath" yaml:"filePath" mapstructure:"filePath"`
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
