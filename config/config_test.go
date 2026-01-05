package config

import (
	"errors"
	"reflect"
	"testing"
	"time"

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
				URI:    "https://my-host",
				APIKey: "my-api-key",
			},
			Manager: Manager{
				Jobs: Jobs{
					MovieIndex:     time.Minute * 15,
					MovieReconcile: time.Minute * 10,
				},
			},
		}

		if !reflect.DeepEqual(c, wantConfig) {
			t.Errorf("TestNew() config = %+v, want %+v", c, wantConfig)
		}
	})

	t.Run("success without file", func(t *testing.T) {
		cu := viper.New()
		cu.SetConfigFile("")
		cu.SetDefault("tmdb.uri", "https://api.themoviedb.org")
		cu.SetDefault("tmdb.apiKey", "fake-key")
		cu.SetDefault("manager.jobs.movieIndex", time.Minute*15)
		cu.SetDefault("manager.jobs.movieReconcile", time.Minute*10)
		c, err := New(cu)
		if err != nil {
			t.Errorf("TestNew() err = %v, want %v", err, nil)
		}

		wantConfig := Config{
			TMDB: TMDB{
				URI:    "https://api.themoviedb.org",
				APIKey: "fake-key",
			},
			Manager: Manager{
				Jobs: Jobs{
					MovieIndex:     time.Minute * 15,
					MovieReconcile: time.Minute * 10,
				},
			},
		}

		if !reflect.DeepEqual(c, wantConfig) {
			t.Errorf("TestNew() config = %+v, want %+v", c, wantConfig)
		}
	})
}
