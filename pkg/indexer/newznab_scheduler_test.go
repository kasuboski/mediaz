package indexer_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/kasuboski/mediaz/pkg/indexer"
	"github.com/kasuboski/mediaz/pkg/indexer/mocks"
	"github.com/kasuboski/mediaz/pkg/newznab"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestNewznabRetryAfterCooldownFailFastAndBounded(t *testing.T) {
	for _, tc := range []struct {
		name string
		pe   *newznab.ProtocolError
		min  time.Duration
	}{
		{"retry-after", &newznab.ProtocolError{Status: 429, RetryAfter: 4 * time.Minute}, 3 * time.Minute},
		{"long-retry-after-is-bounded", &newznab.ProtocolError{Status: 429, RetryAfter: 24 * time.Hour}, 4 * time.Minute},
		{"request-limit-code-500", &newznab.ProtocolError{Code: "500"}, 4 * time.Minute},
		{"download-limit-code-501", &newznab.ProtocolError{Code: "501"}, 4 * time.Minute},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			c := mocks.NewMockNewznabClient(ctrl)
			src := indexer.NewNewznabIndexerSourceWithClient(c, config())
			c.EXPECT().Caps(gomock.Any()).Return(capsFixture(), nil)
			c.EXPECT().Search(gomock.Any(), gomock.Any()).Return(nil, tc.pe)
			_, err := src.Search(context.Background(), 1, nil, indexer.SearchOptions{})
			require.Error(t, err)
			start := time.Now()
			_, err = src.Search(context.Background(), 1, nil, indexer.SearchOptions{})
			var cooldown *indexer.RetryableCooldownError
			require.ErrorAs(t, err, &cooldown)
			require.GreaterOrEqual(t, cooldown.RetryAfter, tc.min)
			require.Less(t, time.Since(start), 100*time.Millisecond)
		})
	}
}

func TestNewznabRequestGateWaitHonorsContext(t *testing.T) {
	ctrl := gomock.NewController(t)
	c := mocks.NewMockNewznabClient(ctrl)
	src := indexer.NewNewznabIndexerSourceWithClient(c, config())
	c.EXPECT().Caps(gomock.Any()).Return(capsFixture(), nil)
	entered := make(chan struct{})
	release := make(chan struct{})
	c.EXPECT().Search(gomock.Any(), gomock.Any()).DoAndReturn(func(context.Context, newznab.SearchRequest) (*newznab.Page, error) {
		close(entered)
		<-release
		return &newznab.Page{}, nil
	})
	done := make(chan error, 1)
	go func() { _, err := src.Search(context.Background(), 1, nil, indexer.SearchOptions{}); done <- err }()
	<-entered
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := src.Search(ctx, 1, nil, indexer.SearchOptions{})
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Less(t, time.Since(start), 200*time.Millisecond)
	close(release)
	require.NoError(t, <-done)
}

func TestNewznabUnverifiedXMLCodesDoNotTriggerCooldown(t *testing.T) {
	for _, code := range []string{"202", "300", "301", "910", "429", "503"} {
		t.Run(code, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			c := mocks.NewMockNewznabClient(ctrl)
			src := indexer.NewNewznabIndexerSourceWithClient(c, config())
			c.EXPECT().Caps(gomock.Any()).Return(capsFixture(), nil)
			c.EXPECT().Search(gomock.Any(), gomock.Any()).Return(nil, &newznab.ProtocolError{Code: code})
			c.EXPECT().Search(gomock.Any(), gomock.Any()).Return(&newznab.Page{}, nil)
			_, err := src.Search(context.Background(), 1, nil, indexer.SearchOptions{})
			require.Error(t, err)
			_, err = src.Search(context.Background(), 1, nil, indexer.SearchOptions{})
			require.NoError(t, err)
		})
	}
}

func TestNewznabCooldownErrorDoesNotLeakProtocolDetails(t *testing.T) {
	var e error = &indexer.RetryableCooldownError{RetryAfter: time.Minute}
	var typed *indexer.RetryableCooldownError
	require.True(t, errors.As(e, &typed))
	require.NotContains(t, e.Error(), "api")
}
