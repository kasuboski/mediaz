package manager

import (
	"context"
	"database/sql"
	"testing"

	downloadMocks "github.com/kasuboski/mediaz/pkg/download/mocks"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func openSABTestStore(t *testing.T) *sqlite.SQLite {
	t.Helper()
	raw, err := sqlite.New(context.Background(), ":memory:")
	require.NoError(t, err)
	return raw.(*sqlite.SQLite)
}

func TestSABConfigCreateUpdateAndTest(t *testing.T) {
	ctx := context.Background()
	t.Run("create defaults are persisted and returned; custom path round trips", func(t *testing.T) {
		store := openSABTestStore(t)
		svc := NewDownloadClientService(store, nil)
		for _, tc := range []struct {
			inputPath, wantPath string
			valid               bool
		}{{"", "/api", false}, {"/custom/api", "/custom/api", true}} {
			input := sqlcdb.DownloadClient{Type: "usenet", Implementation: "sabnzbd", Scheme: "https", Host: "sab.example", Port: 8080, ApiKey: sql.NullString{String: "key", Valid: true}, ApiPath: sql.NullString{String: tc.inputPath, Valid: tc.valid}}
			row, err := svc.CreateDownloadClient(ctx, AddDownloadClientRequest{DownloadClient: input})
			require.NoError(t, err)
			want := input
			want.ApiPath = sql.NullString{String: tc.wantPath, Valid: true}
			want.ID = row.ID
			require.Equal(t, want, row)
			got, err := svc.GetDownloadClient(ctx, row.ID)
			require.NoError(t, err)
			require.Equal(t, want, got)
		}
	})

	t.Run("legacy null path survives omitted update; key rotates and blank key preserves", func(t *testing.T) {
		store := openSABTestStore(t)
		legacy := sqlcdb.DownloadClient{Type: "usenet", Implementation: "sabnzbd", Scheme: "https", Host: "sab.example", Port: 8080, ApiKey: sql.NullString{String: "old", Valid: true}}
		id, err := store.CreateDownloadClient(ctx, legacy)
		require.NoError(t, err)
		svc := NewDownloadClientService(store, nil)
		rotationInput := sqlcdb.DownloadClient{ID: id, Type: "usenet", Implementation: "sabnzbd", Scheme: "https", Host: "sab.example", Port: 8080, ApiKey: sql.NullString{String: "rotated", Valid: true}}
		rotationWant := rotationInput
		rotationWant.ApiPath = sql.NullString{}
		updated, err := svc.UpdateDownloadClient(ctx, id, UpdateDownloadClientRequest{DownloadClient: rotationInput})
		require.NoError(t, err)
		rotationWant.ID = id
		require.Equal(t, rotationWant, updated)
		got, err := svc.GetDownloadClient(ctx, id)
		require.NoError(t, err)
		require.Equal(t, rotationWant, got)

		blankInput := sqlcdb.DownloadClient{Type: "usenet", Implementation: "sabnzbd", Scheme: "https", Host: "sab.example", Port: 8080, ApiKey: sql.NullString{String: "", Valid: true}}
		blankWant := blankInput
		blankWant.ID = id
		blankWant.ApiKey = rotationWant.ApiKey
		blankWant.ApiPath = sql.NullString{}
		updated, err = svc.UpdateDownloadClient(ctx, id, UpdateDownloadClientRequest{DownloadClient: blankInput})
		require.NoError(t, err)
		require.Equal(t, blankWant, updated)
		got, err = svc.GetDownloadClient(ctx, id)
		require.NoError(t, err)
		require.Equal(t, blankWant, got)
	})

	t.Run("invalid creates fail validation without writing", func(t *testing.T) {
		store := openSABTestStore(t)
		svc := NewDownloadClientService(store, nil)
		for _, bad := range []sqlcdb.DownloadClient{
			{Type: "usenet", Implementation: "sabnzbd", Scheme: "ftp", Host: "sab.example", Port: 80, ApiKey: sql.NullString{String: "key", Valid: true}, ApiPath: sql.NullString{String: "/api", Valid: true}},
			{Type: "usenet", Implementation: "sabnzbd", Scheme: "https", Host: "bad/path", Port: 443, ApiKey: sql.NullString{String: "key", Valid: true}, ApiPath: sql.NullString{String: "/api", Valid: true}},
			{Type: "usenet", Implementation: "sabnzbd", Scheme: "https", Host: "sab.example", Port: 70000, ApiKey: sql.NullString{String: "key", Valid: true}, ApiPath: sql.NullString{String: "/api", Valid: true}},
			{Type: "usenet", Implementation: "sabnzbd", Scheme: "https", Host: "sab.example", Port: 443, ApiKey: sql.NullString{String: "key", Valid: true}, ApiPath: sql.NullString{String: "relative", Valid: true}},
		} {
			_, err := svc.CreateDownloadClient(ctx, AddDownloadClientRequest{DownloadClient: bad})
			require.ErrorIs(t, err, ErrValidation)
		}
		rows, err := store.ListDownloadClients(ctx)
		require.NoError(t, err)
		require.Empty(t, rows)
	})

	t.Run("update blank path defaults; invalid configs do not write", func(t *testing.T) {
		store := openSABTestStore(t)
		id, err := store.CreateDownloadClient(ctx, sqlcdb.DownloadClient{Type: "usenet", Implementation: "sabnzbd", Scheme: "https", Host: "sab.example", Port: 8080, ApiKey: sql.NullString{String: "key", Valid: true}, ApiPath: sql.NullString{String: "/keep", Valid: true}})
		require.NoError(t, err)
		svc := NewDownloadClientService(store, nil)
		blankInput := sqlcdb.DownloadClient{Type: "usenet", Implementation: "sabnzbd", Scheme: "https", Host: "sab.example", Port: 8080, ApiPath: sql.NullString{String: "", Valid: true}}
		want := blankInput
		want.ID = id
		want.ApiKey = sql.NullString{String: "key", Valid: true}
		want.ApiPath = sql.NullString{String: "/api", Valid: true}
		got, err := svc.UpdateDownloadClient(ctx, id, UpdateDownloadClientRequest{DownloadClient: blankInput})
		require.NoError(t, err)
		require.Equal(t, want, got)
		before, err := svc.GetDownloadClient(ctx, id)
		require.NoError(t, err)
		require.Equal(t, want, before)
		require.NoError(t, err)
		for _, bad := range []sqlcdb.DownloadClient{{Scheme: "ftp", Host: "sab.example", Port: 80, ApiPath: sql.NullString{String: "/api", Valid: true}}, {Scheme: "https", Host: "bad/path", Port: 443, ApiPath: sql.NullString{String: "/api", Valid: true}}, {Scheme: "https", Host: "sab.example", Port: 70000, ApiPath: sql.NullString{String: "/api", Valid: true}}, {Scheme: "https", Host: "sab.example", Port: 443, ApiPath: sql.NullString{String: "relative", Valid: true}}} {
			bad.Type = "usenet"
			bad.Implementation = "sabnzbd"
			bad.ApiKey = sql.NullString{String: "key", Valid: true}
			_, err = svc.UpdateDownloadClient(ctx, id, UpdateDownloadClientRequest{DownloadClient: bad})
			require.ErrorIs(t, err, ErrValidation)
			after, e := svc.GetDownloadClient(ctx, id)
			require.NoError(t, e)
			require.Equal(t, before, after)
		}
	})

	t.Run("Test reuses stored credentials and path; unsaved test defaults path", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		factory := downloadMocks.NewMockFactory(ctrl)
		client := downloadMocks.NewMockDownloadClient(ctrl)
		store := openSABTestStore(t)
		stored := sqlcdb.DownloadClient{Type: "usenet", Implementation: "sabnzbd", Scheme: "https", Host: "sab.example", Port: 8080, ApiKey: sql.NullString{String: "secret", Valid: true}, ApiPath: sql.NullString{String: "/proxy/api", Valid: true}}
		id, err := store.CreateDownloadClient(ctx, stored)
		require.NoError(t, err)
		stored.ID = id
		factory.EXPECT().NewDownloadClient(stored).Return(client, nil)
		client.EXPECT().List(ctx).Return(nil, nil)
		svc := NewDownloadClientService(store, factory)
		testInput := sqlcdb.DownloadClient{ID: id, Type: "usenet", Implementation: "sabnzbd", Scheme: "https", Host: "sab.example", Port: 8080}
		require.NoError(t, svc.TestDownloadClient(ctx, AddDownloadClientRequest{DownloadClient: testInput}))
		persisted, err := svc.GetDownloadClient(ctx, id)
		require.NoError(t, err)
		require.Equal(t, stored, persisted)
		unsaved := sqlcdb.DownloadClient{Type: "usenet", Implementation: "sabnzbd", Scheme: "https", Host: "sab.example", Port: 8080, ApiKey: sql.NullString{String: "new", Valid: true}, ApiPath: sql.NullString{String: "/api", Valid: true}}
		factory.EXPECT().NewDownloadClient(unsaved).Return(client, nil)
		client.EXPECT().List(ctx).Return(nil, nil)
		beforeClients, err := store.ListDownloadClients(ctx)
		require.NoError(t, err)
		require.NoError(t, svc.TestDownloadClient(ctx, AddDownloadClientRequest{DownloadClient: sqlcdb.DownloadClient{Type: "usenet", Implementation: "sabnzbd", Scheme: "https", Host: "sab.example", Port: 8080, ApiKey: unsaved.ApiKey}}))
		afterClients, err := store.ListDownloadClients(ctx)
		require.NoError(t, err)
		require.Equal(t, beforeClients, afterClients)
		require.Len(t, afterClients, 1)
	})

	t.Run("Test legacy null path remains null while factory applies runtime legacy default", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		factory := downloadMocks.NewMockFactory(ctrl)
		client := downloadMocks.NewMockDownloadClient(ctrl)
		store := openSABTestStore(t)
		legacy := sqlcdb.DownloadClient{Type: "usenet", Implementation: "sabnzbd", Scheme: "https", Host: "sab.example", Port: 8080, ApiKey: sql.NullString{String: "legacy-secret", Valid: true}}
		id, err := store.CreateDownloadClient(ctx, legacy)
		require.NoError(t, err)
		legacy.ID = id
		factory.EXPECT().NewDownloadClient(legacy).Return(client, nil)
		client.EXPECT().List(ctx).Return(nil, nil)
		svc := NewDownloadClientService(store, factory)
		request := sqlcdb.DownloadClient{ID: id, Type: "usenet", Implementation: "sabnzbd", Scheme: "https", Host: "sab.example", Port: 8080}
		require.NoError(t, svc.TestDownloadClient(ctx, AddDownloadClientRequest{DownloadClient: request}))
		persisted, err := svc.GetDownloadClient(ctx, id)
		require.NoError(t, err)
		require.Equal(t, legacy, persisted)
	})

}
