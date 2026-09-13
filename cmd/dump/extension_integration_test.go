package dump

import (
	"context"
	"testing"

	"github.com/pgplex/pgschema/testutil"
	"github.com/stretchr/testify/require"
)

// Issue #595: pg_stat_statements' view was dumped while its required function
// was omitted. Neither definition belongs in an application schema dump.
func TestDumpExtensionMembers(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	pg := testutil.SetupPostgres(t)
	defer pg.Stop()
	db, host, port, name, user, password := testutil.ConnectToPostgres(t, pg)
	defer db.Close()
	ctx := context.Background()
	// Catalog inspection and view creation do not execute pg_stat_statements,
	// so the bundled extension needs no shared_preload_libraries modification.
	_, err := db.ExecContext(ctx, `
		CREATE EXTENSION pg_stat_statements;
		CREATE TABLE app_requests (id integer PRIMARY KEY);
		CREATE VIEW app_stats AS SELECT queryid FROM pg_stat_statements;
		GRANT SELECT ON app_requests TO PUBLIC;
		GRANT SELECT (queryid) ON pg_stat_statements TO PUBLIC;
	`)
	require.NoError(t, err)
	config := &DumpConfig{
		Host: host, Port: port, DB: name, User: user, Password: password,
		Schema: "public", NoComments: true, ConfigDir: t.TempDir(),
	}
	dumped, err := ExecuteDump(config)
	require.NoError(t, err)
	require.NotContains(t, dumped, "VIEW pg_stat_statements")
	require.NotContains(t, dumped, "CREATE OR REPLACE FUNCTION pg_stat_statements")
	require.NotContains(t, dumped, "ON TABLE pg_stat_statements")
	require.Contains(t, dumped, "CREATE OR REPLACE VIEW app_stats")
	require.Contains(t, dumped, "FROM pg_stat_statements")
	require.Contains(t, dumped, "GRANT SELECT ON TABLE app_requests TO PUBLIC")
	// Reload the unmodified native dump while the extension is still installed.
	_, err = db.ExecContext(ctx, `DROP VIEW app_stats; DROP TABLE app_requests;`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, dumped)
	require.NoError(t, err)
	roundtrip, err := ExecuteDump(config)
	require.NoError(t, err)
	require.Equal(t, dumped, roundtrip)
}
