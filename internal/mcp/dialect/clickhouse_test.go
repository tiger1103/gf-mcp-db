package dialect_test

import (
	"testing"

	"github.com/tiger1103/gf-mcp-db/internal/mcp/dialect"
)

func TestClickhouseDialectBasics(t *testing.T) {
	d, err := dialect.Get("clickhouse")
	if err != nil {
		t.Fatalf("clickhouse 方言未注册: %v", err)
	}
	if d.Name() != "clickhouse" {
		t.Fatalf("方言名不符: %q", d.Name())
	}
	if got := d.QuoteIdent("events"); got != "`events`" {
		t.Fatalf("QuoteIdent = %q", got)
	}
	if got := d.Paginate("SELECT * FROM t", 5); got != "SELECT * FROM t LIMIT 5" {
		t.Fatalf("Paginate = %q", got)
	}
	if got := d.Paginate("SELECT * FROM t;", 0); got != "SELECT * FROM t LIMIT 100" {
		t.Fatalf("Paginate 默认值 = %q", got)
	}
}
