/*
 * @desc:dialect 基础能力单元测试
 */

package dialect_test

import (
	"testing"

	"github.com/gogf/gf/v2/database/gdb"

	"github.com/tiger1103/gf-mcp-db/internal/mcp/dialect"
)

// testDialect 仅用于测试基础能力
type testDialect struct {
	dialect.BaseDialect
}

func newTestDialect() *testDialect {
	return &testDialect{BaseDialect: dialect.NewBaseDialect("test", "`", "`")}
}

func TestGetUnknown(t *testing.T) {
	if _, err := dialect.Get("no-such-db"); err == nil {
		t.Fatal("未注册方言应返回错误")
	}
}

func TestQuoteIdent(t *testing.T) {
	d := newTestDialect()
	if got := d.QuoteIdent("users"); got != "`users`" {
		t.Fatalf("QuoteIdent(users) = %q", got)
	}
	if got := d.QuoteIdent("public.users"); got != "`public`.`users`" {
		t.Fatalf("QuoteIdent(public.users) = %q", got)
	}
	mustPanic(t, func() { d.QuoteIdent("") })
	mustPanic(t, func() { d.QuoteIdent("users; DROP TABLE x") })
	mustPanic(t, func() { d.QuoteIdent("user name") })
}

func mustPanic(t *testing.T, fn func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatal("期望 panic")
		}
	}()
	fn()
}

func TestBasePaginate(t *testing.T) {
	d := newTestDialect()
	if got := d.Paginate("SELECT * FROM t", 10); got != "SELECT * FROM t LIMIT 10" {
		t.Fatalf("Paginate = %q", got)
	}
	// 非法 limit 回落默认 100
	if got := d.Paginate("SELECT * FROM t", 0); got != "SELECT * FROM t LIMIT 100" {
		t.Fatalf("Paginate 默认值 = %q", got)
	}
}

func TestMatchPattern(t *testing.T) {
	cases := []struct {
		name, pattern string
		want          bool
	}{
		{"users", "", true},
		{"users", "user%", true},
		{"users", "user", false},
		{"USERS", "user%", true}, // 大小写不敏感（与 MySQL LIKE 行为一致）
		{"user_1", "user_1", true},
		{"userX1", "user_1", true}, // _ 匹配任意单字符
		{"orders", "gf_mcp_t_o%", false},
	}
	for _, c := range cases {
		if got := dialect.MatchPattern(c.name, c.pattern); got != c.want {
			t.Fatalf("MatchPattern(%q,%q) = %v, want %v", c.name, c.pattern, got, c.want)
		}
	}
}

func TestColumnsFromTableFields(t *testing.T) {
	fields := map[string]*gdb.TableField{
		"name": {Index: 1, Name: "name", Type: "varchar(64)", Null: true},
		"id":   {Index: 0, Name: "id", Type: "bigint", Null: false, Key: "pri", Extra: "auto_increment", Default: nil, Comment: "主键"},
	}
	cols := dialect.ColumnsFromTableFields(fields)
	if len(cols) != 2 {
		t.Fatalf("列数不符: %d", len(cols))
	}
	if cols[0]["column_name"] != "id" || cols[1]["column_name"] != "name" {
		t.Fatalf("应按 Index 排序: %+v", cols)
	}
	if cols[0]["primary_key"] != "true" || cols[0]["nullable"] != "false" {
		t.Fatalf("主键/可空标记不符: %+v", cols[0])
	}
}
