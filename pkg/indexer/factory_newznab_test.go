package indexer_test

import (
	"database/sql"
	"github.com/kasuboski/mediaz/pkg/indexer"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestFactoryRetainsNewznabSourceAndSeparatesConfigurations(t *testing.T) {
	f := indexer.NewIndexerSourceFactory()
	c := sqlcdb.IndexerSource{Implementation: "newznab", Enabled: true, Name: "one", Scheme: "https", Host: "indexer.example", ApiPath: sql.NullString{String: "/custom/api", Valid: true}}
	a, err := f.NewIndexerSource(c)
	require.NoError(t, err)
	b, err := f.NewIndexerSource(c)
	require.NoError(t, err)
	require.Same(t, a, b)
	c.ApiPath.String = "/other"
	d, err := f.NewIndexerSource(c)
	require.NoError(t, err)
	require.NotSame(t, a, d)
}
