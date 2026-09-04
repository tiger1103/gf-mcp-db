package dialect_test

import (
	"testing"

	"github.com/tiger1103/gf-mcp-db/internal/mcp/dialect"
)

func TestSqliteDialectBasics(t *testing.T) {
	d, err := dialect.Get("sqlite")
	if err != nil {
		t.Fatalf("sqlite 方言未注册: %v", err)
	}
	if d.Name() != "sqlite" {
		t.Fatalf("方言名不符: %q", d.Name())
	}
	if got := d.QuoteIdent("users"); got != "`users`" {
		t.Fatalf("QuoteIdent = %q", got)
	}
	if got := d.Paginate("SELECT * FROM t", 5); got != "SELECT * FROM t LIMIT 5" {
		t.Fatalf("Paginate = %q", got)
	}
}
