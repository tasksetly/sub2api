package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAccountSupplierMigrationAddsColumnAndIndex(t *testing.T) {
	content, err := FS.ReadFile("185_add_account_supplier.sql")
	require.NoError(t, err)

	sql := strings.Join(strings.Fields(string(content)), " ")
	require.Contains(t, sql, "ALTER TABLE accounts ADD COLUMN IF NOT EXISTS supplier VARCHAR(100) NOT NULL DEFAULT ''")
	require.Contains(t, sql, "CREATE INDEX IF NOT EXISTS account_supplier ON accounts (supplier)")
}
