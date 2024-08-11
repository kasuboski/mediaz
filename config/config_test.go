package config

import (
	"errors"
	"reflect"
	"testing"

	"github.com/kasuboski/mediaz/config/mocks"
	"github.com/spf13/viper"
	"go.uber.org/mock/gomock"
)

func TestNew(t *testing.T) {
	t.Run("fail to read in config", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		cu := mocks.NewMockConfigUnmarshaler(ctrl)

		wantErr := errors.New("expected testing error")
		cu.EXPECT().ConfigFileUsed().Times(1).Return("fake-config.yaml")
		cu.EXPECT().ReadInConfig().Times(1).Return(wantErr)
		c, err := New(cu)
		if err == nil {
			t.Errorf("TestNew() err = %v, want %v", err, wantErr)
		}

		wantConfig := Config{}
		if !reflect.DeepEqual(c, wantConfig) {
			t.Errorf("TestNew() config = %v, want %v", c, wantConfig)
		}
	})

	t.Run("success with file", func(t *testing.T) {
		cu := viper.New()
		cu.SetConfigFile("./testing/config.yaml")
		c, err := New(cu)
		if err != nil {
			t.Errorf("TestNew() err = %v, want %v", err, nil)
		}

		wantConfig := Config{
			TMDB: TMDB{
				Scheme: "https",
				Host:   "my-host",
				APIKey: "my-api-key",
			},
			Prowlarr: Prowlarr{
				Scheme: "https",
				Host:   "my-prowlarr-host",
				APIKey: "my-prowlarr-api-key",
			},
		}

		if !reflect.DeepEqual(c, wantConfig) {
			t.Errorf("TestNew() config = %+v, want %+v", c, wantConfig)
		}
	})

	t.Run("success without file", func(t *testing.T) {
		cu := viper.New()
		cu.SetConfigFile("")
		cu.SetDefault("tmdb.scheme", "https")
		cu.SetDefault("prowlarr.scheme", "http")
		c, err := New(cu)
		if err != nil {
			t.Errorf("TestNew() err = %v, want %v", err, nil)
		}

		wantConfig := Config{
			TMDB: TMDB{
				Scheme: "https",
			},
			Prowlarr: Prowlarr{
				Scheme: "http",
			},
		}

		if !reflect.DeepEqual(c, wantConfig) {
			t.Errorf("TestNew() config = %+v, want %+v", c, wantConfig)
		}
	})
}
