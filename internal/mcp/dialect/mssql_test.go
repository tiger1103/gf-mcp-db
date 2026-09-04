package dialect_test

import (
	"testing"

	"github.com/tiger1103/gf-mcp-db/internal/mcp/dialect"
)

func TestMssqlDialectBasics(t *testing.T) {
	d, err := dialect.Get("mssql")
	if err != nil {
		t.Fatalf("mssql 方言未注册: %v", err)
	}
	if d.Name() != "mssql" {
		t.Fatalf("方言名不符: %q", d.Name())
	}
	if got := d.QuoteIdent("users"); got != `"users"` {
		t.Fatalf("QuoteIdent = %q", got)
	}
	cases := []struct {
		in    string
		limit int
		want  string
	}{
		{"SELECT * FROM t", 10, "SELECT TOP 10 * FROM t"},
		{"SELECT DISTINCT a FROM t", 10, "SELECT DISTINCT TOP 10 a FROM t"},
		{"SELECT a FROM t ORDER BY b", 1, "SELECT TOP 1 a FROM t ORDER BY b"},
		{"SELECT * FROM t", 0, "SELECT TOP 100 * FROM t"},
		{"SELECT 1;", 0, "SELECT TOP 100 1"},
		{"WITH cte AS (SELECT 1 AS x) SELECT x FROM cte", 5, "WITH cte AS (SELECT 1 AS x) SELECT x FROM cte"},
		{"SELECT DISTINCT a FROM t;", 5, "SELECT DISTINCT TOP 5 a FROM t"},
	}
	for _, c := range cases {
		if got := d.Paginate(c.in, c.limit); got != c.want {
			t.Fatalf("Paginate(%q,%d) = %q, want %q", c.in, c.limit, got, c.want)
		}
	}
}
