package dialect_test

import (
	"testing"

	"github.com/tiger1103/gf-mcp-db/internal/mcp/dialect"
)

func TestOracleDialectBasics(t *testing.T) {
	d, err := dialect.Get("oracle")
	if err != nil {
		t.Fatalf("oracle 方言未注册: %v", err)
	}
	if d.Name() != "oracle" {
		t.Fatalf("方言名不符: %q", d.Name())
	}
	if got := d.QuoteIdent("USERS"); got != `"USERS"` {
		t.Fatalf("QuoteIdent = %q", got)
	}
	if got := d.QuoteIdent("users"); got != `"USERS"` {
		t.Fatalf("QuoteIdent 小写应归一为大写: %q", got)
	}
	if got := d.Paginate("SELECT * FROM t", 5); got != "SELECT * FROM t FETCH FIRST 5 ROWS ONLY" {
		t.Fatalf("Paginate = %q", got)
	}
	if got := d.Paginate("SELECT * FROM t;", 5); got != "SELECT * FROM t FETCH FIRST 5 ROWS ONLY" {
		t.Fatalf("Paginate 应去尾分号: %q", got)
	}
}

func TestDmDialectBasics(t *testing.T) {
	d, err := dialect.Get("dm")
	if err != nil {
		t.Fatalf("dm 方言未注册: %v", err)
	}
	if d.Name() != "dm" {
		t.Fatalf("方言名不符: %q", d.Name())
	}
	if got := d.QuoteIdent("USERS"); got != `"USERS"` {
		t.Fatalf("QuoteIdent = %q", got)
	}
	if got := d.QuoteIdent("users"); got != `"USERS"` {
		t.Fatalf("QuoteIdent 小写应归一为大写: %q", got)
	}
	// DM8 兼容 MySQL 的 LIMIT 语法
	if got := d.Paginate("SELECT * FROM t", 5); got != "SELECT * FROM t LIMIT 5" {
		t.Fatalf("Paginate = %q", got)
	}
	if got := d.Paginate("SELECT * FROM t;", 0); got != "SELECT * FROM t LIMIT 100" {
		t.Fatalf("Paginate 默认值 = %q", got)
	}
}
