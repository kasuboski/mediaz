package logger

import (
	"context"
	"fmt"
	"log"
	"os"
	"runtime/debug"
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type ctxKey struct{}

var once sync.Once

var logger *zap.SugaredLogger

// Get initializes a zap.SugaredLogger instance if it has not been initialized
// already and returns the same instance for subsequent calls.
func Get() *zap.SugaredLogger {
	once.Do(func() {
		stdout := zapcore.AddSync(os.Stdout)

		level := zap.InfoLevel
		levelEnv := os.Getenv("LOG_LEVEL")
		if levelEnv != "" {
			levelFromEnv, err := zapcore.ParseLevel(levelEnv)
			if err != nil {
				log.Println(
					fmt.Errorf("invalid level, defaulting to INFO: %w", err),
				)
			}

			level = levelFromEnv
		}

		logLevel := zap.NewAtomicLevelAt(level)

		productionCfg := zap.NewProductionEncoderConfig()
		productionCfg.TimeKey = "timestamp"
		productionCfg.EncodeTime = zapcore.ISO8601TimeEncoder

		developmentCfg := zap.NewDevelopmentEncoderConfig()
		developmentCfg.EncodeLevel = zapcore.CapitalColorLevelEncoder

		encoder := zapcore.NewConsoleEncoder(developmentCfg)
		isJSON := os.Getenv("JSON_LOG")
		if isJSON != "" {
			encoder = zapcore.NewJSONEncoder(productionCfg)
		}

		core := zapcore.NewCore(encoder, stdout, logLevel)

		var gitRevision string

		buildInfo, ok := debug.ReadBuildInfo()
		if ok {
			var fields []zapcore.Field
			fields = append(fields, zap.String("go_version", buildInfo.GoVersion))
			for _, v := range buildInfo.Settings {
				if v.Key == "vcs.revision" {
					gitRevision = v.Value[0:7]
					fields = append(fields, zap.String("git_revision", gitRevision))
					break
				}
			}

			core = core.With(fields)
		}

		logger = zap.New(core).Sugar()
	})

	return logger
}

// FromCtx returns the Logger associated with the ctx. If no logger
// is associated, the default logger is returned, unless it is nil
// in which case a disabled logger is returned.
func FromCtx(ctx context.Context, with ...any) *zap.SugaredLogger {
	if l, ok := ctx.Value(ctxKey{}).(*zap.SugaredLogger); ok {
		return l.With(with)
	} else if l := logger; l != nil {
		return l.With(with)
	}

	return Get().With(with)
}

// WithCtx returns a copy of ctx with the Logger attached.
func WithCtx(ctx context.Context, l *zap.SugaredLogger) context.Context {
	if lp, ok := ctx.Value(ctxKey{}).(*zap.SugaredLogger); ok {
		if lp == l {
			// Do not store same logger.
			return ctx
		}
	}

	return context.WithValue(ctx, ctxKey{}, l)
}
