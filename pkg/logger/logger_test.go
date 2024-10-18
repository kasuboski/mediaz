package logger

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGet(t *testing.T) {
	logger1 := Get()
	require.NotNil(t, logger1)

	logger2 := Get()
	assert.Same(t, logger1, logger2)
}

func TestFromCtx(t *testing.T) {
	ctx := WithCtx(context.Background(), Get())

	loggerFromCtx := FromCtx(ctx)

	assert.Same(t, Get(), loggerFromCtx)

	customLogger := Get().With("custom", "value")
	ctxWithCustomLogger := WithCtx(ctx, customLogger)

	loggerFromCustomCtx := FromCtx(ctxWithCustomLogger)

	assert.Same(t, customLogger, loggerFromCustomCtx)
}

func TestWithCtx(t *testing.T) {
	ctx := context.Background()
	logger := Get()

	newCtx := WithCtx(ctx, logger)

	assert.Same(t, logger, FromCtx(newCtx))
}

func TestWithSameLogger(t *testing.T) {
	ctx := context.Background()
	logger := Get()

	newCtx := WithCtx(ctx, logger)

	assert.Same(t, newCtx, WithCtx(newCtx, logger))
}
