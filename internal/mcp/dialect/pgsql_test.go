package dialect_test

import (
	"testing"

	"github.com/tiger1103/gf-mcp-db/internal/mcp/dialect"
)

func TestPgsqlDialectBasics(t *testing.T) {
	d, err := dialect.Get("pgsql")
	if err != nil {
		t.Fatalf("pgsql 方言未注册: %v", err)
	}
	if got := d.QuoteIdent("users"); got != `"users"` {
		t.Fatalf("QuoteIdent = %q", got)
	}
	if got := d.Paginate("SELECT * FROM t", 5); got != "SELECT * FROM t LIMIT 5" {
		t.Fatalf("Paginate = %q", got)
	}
	if d.Name() != "pgsql" {
		t.Fatalf("方言名不符: %q", d.Name())
	}
	if _, err := dialect.Get("PGSQL"); err != nil {
		t.Fatalf("Get 应大小写归一: %v", err)
	}
	if got := d.Paginate("SELECT 1;", 0); got != "SELECT 1 LIMIT 100" {
		t.Fatalf("Paginate 应去尾分号并回落默认: %q", got)
	}
}
