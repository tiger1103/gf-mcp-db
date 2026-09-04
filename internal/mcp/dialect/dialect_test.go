/*
 * @desc:dialect 基础能力单元测试
 */

package dialect_test

import (
	"context"
	"testing"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

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
		r := recover()
		if r == nil {
			t.Fatal("期望 panic")
		}
		if _, ok := r.(*gerror.Error); !ok {
			t.Fatalf("期望 *gerror.Error，实际 %T", r)
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
	cols := dialect.ColumnsFromTableFields(fields, map[string]bool{"id": true})
	if len(cols) != 2 {
		t.Fatalf("列数不符: %d", len(cols))
	}
	if cols[0]["column_name"] != "id" || cols[1]["column_name"] != "name" {
		t.Fatalf("应按 Index 排序: %+v", cols)
	}
	if cols[0]["primary_key"] != "true" || cols[0]["nullable"] != "false" {
		t.Fatalf("主键/可空标记不符: %+v", cols[0])
	}
	// primaryKeys 传 nil 时所有列 primary_key 均为 false
	cols2 := dialect.ColumnsFromTableFields(fields, nil)
	if cols2[0]["primary_key"] != "false" {
		t.Fatalf("primaryKeys 为 nil 时 primary_key 应为 false: %+v", cols2[0])
	}
}

func TestQuoteIdentEdgeCases(t *testing.T) {
	d := newTestDialect()
	// 合法：$ # 结尾允许
	if got := d.QuoteIdent("col$1#"); got != "`col$1#`" {
		t.Fatalf("QuoteIdent(col$1#) = %q", got)
	}
	// 非法：空段、尾部点、反引号注入、非 ASCII
	mustPanic(t, func() { d.QuoteIdent("a..b") })
	mustPanic(t, func() { d.QuoteIdent("a.") })
	mustPanic(t, func() { d.QuoteIdent("a`b") })
	mustPanic(t, func() { d.QuoteIdent("用户") })
}

func TestMatchPatternMetachars(t *testing.T) {
	// 正则元字符必须按字面匹配
	if !dialect.MatchPattern("a.b", "a.b") {
		t.Fatal("字面量 a.b 应匹配 a.b")
	}
	if dialect.MatchPattern("aXb", "a.b") {
		t.Fatal("a.b 模式中的点不应匹配任意字符")
	}
	if dialect.MatchPattern("a+b", "a+b") != true || dialect.MatchPattern("ab", "a+b") != false {
		t.Fatal("加号应按字面处理")
	}
}

type registeredDialect struct {
	dialect.BaseDialect
}

// Indexes/ForeignKeys/TableStat 桩实现：BaseDialect 不提供这三者的默认实现，
// 这里仅为满足 IDialect 接口以测试注册表（测试不会调用它们）。
func (d *registeredDialect) Indexes(ctx context.Context, db gdb.DB, table string) (gdb.Result, error) {
	return nil, nil
}

func (d *registeredDialect) ForeignKeys(ctx context.Context, db gdb.DB, table string) (gdb.Result, error) {
	return nil, nil
}

func (d *registeredDialect) TableStat(ctx context.Context, db gdb.DB, table string) (gdb.Record, error) {
	return nil, nil
}

func TestRegisterAndGet(t *testing.T) {
	dialect.Register(&registeredDialect{dialect.NewBaseDialect("test-reg-dialect", "`", "`")})
	got, err := dialect.Get("TEST-REG-DIALECT") // Get 应做大小写归一
	if err != nil {
		t.Fatalf("Get 应命中注册的方言: %v", err)
	}
	if got.Name() != "test-reg-dialect" {
		t.Fatalf("方言名不符: %q", got.Name())
	}
	mustPanic(t, func() {
		dialect.Register(&registeredDialect{dialect.NewBaseDialect("test-reg-dialect", "`", "`")})
	})
}
