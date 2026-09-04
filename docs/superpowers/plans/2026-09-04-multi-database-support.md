# 多数据库支持实施计划（SQLite / PostgreSQL / SQL Server / Oracle / DM）

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** gf-mcp-db 从仅 MySQL 实际可用扩展为 6 库统一支持（MySQL/PostgreSQL/SQLite/SQL Server/Oracle/DM），7 个 MCP 工具全部可用，输出统一跨库格式，全部驱动零 CGO。

**Architecture:** 新增 `internal/dbconn`（统一配置解析/校验/连接初始化，直接填充 `gdb.ConfigNode` 字段，绕开 gf Link 正则）与 `internal/mcp/dialect`（方言层：引用符/LIMIT 语法/索引/外键/表状态，6 实现 + 注册表）。工具层改用 `db.Tables()`/`db.TableFields()` 元数据 API + 方言层，删除全部 MySQL 专有 SQL；`cmd.go` 与 `router.go` 去重为共用 dbconn。设计依据：`docs/superpowers/specs/2026-09-04-multi-database-support-design.md`。

**Tech Stack:** Go 1.23 / GoFrame v2.9.0（contrib 驱动同版本）/ mcp-go v0.41.1。已核实：v2.9.0 的 sqlite 驱动 Open 使用 `config.Name`（文件路径），mysql/mssql/oracle/dm 的 Open 只消费 ConfigNode 字段；pgsql `TableFields.Key` 为小写 `pri`；`gcache.Adapter.Clear(ctx)` 存在；mcp-go 请求结构为 `mcp.CallToolRequest{Params: mcp.CallToolParams{Name, Arguments}}`。

**统一输出键约定（所有方言一致）：**
- 列：`column_name / data_type / nullable / primary_key / default / extra / comment`
- 索引：`index_name / column_name / is_unique`
- 外键：`constraint_name / column_name / referenced_table_name / referenced_column_name`
- 表状态：`rows_estimate / table_comment / engine / data_length / index_length`（按库可得性，缺则省略键）

**环境注意：** 工作目录 `F:\project\goProject\p2026\gf-mcp-db`；命令用 pwsh 运行；git.exe 偶发启动被拒（杀软瞬时锁），重试即可。集成测试凭据只经环境变量传入，不写入仓库。MySQL extra 可用 `loc=Local` 保留旧版时区行为（旧 DSN 有 `loc=Local`，新版默认不带）。

---

### Task 1: 引入 4 个 contrib 驱动并接入 main.go

**Files:**
- Modify: `go.mod`（经 go get）
- Modify: `main.go`

- [ ] **Step 1: 添加依赖（与 gf 主库同版本 v2.9.0）**

```bash
go get github.com/gogf/gf/contrib/drivers/sqlite/v2@v2.9.0
go get github.com/gogf/gf/contrib/drivers/mssql/v2@v2.9.0
go get github.com/gogf/gf/contrib/drivers/oracle/v2@v2.9.0
go get github.com/gogf/gf/contrib/drivers/dm/v2@v2.9.0
```

Expected: go.sum 新增多条记录；无版本冲突（contrib v2.9.0 依赖 gf/v2 v2.9.0）。

- [ ] **Step 2: main.go 导入全部驱动**

`main.go` 完整替换为：

```go
package main

import (
	_ "github.com/gogf/gf/contrib/drivers/dm/v2"
	_ "github.com/gogf/gf/contrib/drivers/mssql/v2"
	_ "github.com/gogf/gf/contrib/drivers/mysql/v2"
	_ "github.com/gogf/gf/contrib/drivers/oracle/v2"
	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	_ "github.com/gogf/gf/contrib/drivers/sqlite/v2"
	"github.com/gogf/gf/v2/os/gctx"

	_ "github.com/tiger1103/gf-mcp-db/internal/boot"
	"github.com/tiger1103/gf-mcp-db/internal/cmd"
	_ "github.com/tiger1103/gf-mcp-db/internal/packed"
)

func main() {
	cmd.Main.Run(gctx.GetInitCtx())
}
```

- [ ] **Step 3: 编译验证（证明全链路纯 Go、零 CGO）**

```bash
$env:CGO_ENABLED="0"; go build ./...
```

Expected: 退出码 0。

- [ ] **Step 4: Commit**

```bash
git add go.mod go.sum main.go
git commit -m "feat: 引入 sqlite/mssql/oracle/dm 官方 contrib 驱动（纯 Go，零 CGO）"
```

---

### Task 2: internal/dbconn 包（配置归一/校验/连接初始化）

**Files:**
- Create: `internal/dbconn/dbconn.go`
- Test: `internal/dbconn/dbconn_test.go`

- [ ] **Step 1: 先写失败测试**

创建 `internal/dbconn/dbconn_test.go`：

```go
/*
 * @desc:dbconn 单元测试
 */

package dbconn_test

import (
	"testing"

	"github.com/tiger1103/gf-mcp-db/internal/dbconn"
)

func TestNormalizeType(t *testing.T) {
	cases := []struct {
		in     string
		want   string
		wantOK bool
	}{
		{"mysql", "mysql", true},
		{"MySQL", "mysql", true},
		{"postgres", "pgsql", true},
		{"postgresql", "pgsql", true},
		{"pgsql", "pgsql", true},
		{"sqlite", "sqlite", true},
		{"sqlite3", "sqlite", true},
		{"sqlserver", "mssql", true},
		{"mssql", "mssql", true},
		{"oracle", "oracle", true},
		{"dm", "dm", true},
		{"", "", false},
		{"foo", "", false},
	}
	for _, c := range cases {
		got, ok := dbconn.NormalizeType(c.in)
		if got != c.want || ok != c.wantOK {
			t.Fatalf("NormalizeType(%q) = (%q,%v), want (%q,%v)", c.in, got, ok, c.want, c.wantOK)
		}
	}
}

func TestBuildConfigNode(t *testing.T) {
	t.Run("sqlite 缺少文件路径报错", func(t *testing.T) {
		_, err := dbconn.BuildConfigNode(&dbconn.Config{DBType: "sqlite"})
		if err == nil {
			t.Fatal("期望报错")
		}
	})

	t.Run("mysql 缺少 host 报错", func(t *testing.T) {
		_, err := dbconn.BuildConfigNode(&dbconn.Config{DBType: "mysql", Username: "root", Database: "test"})
		if err == nil {
			t.Fatal("期望报错")
		}
	})

	t.Run("不支持类型报错", func(t *testing.T) {
		_, err := dbconn.BuildConfigNode(&dbconn.Config{DBType: "oracle19c"})
		if err == nil {
			t.Fatal("期望报错")
		}
	})

	t.Run("extra 格式非法报错", func(t *testing.T) {
		_, err := dbconn.BuildConfigNode(&dbconn.Config{DBType: "mysql", Host: "h", Username: "u", Database: "d", Extra: "not-a-kv"})
		if err == nil {
			t.Fatal("期望报错")
		}
	})

	t.Run("mysql 完整配置与默认值", func(t *testing.T) {
		node, err := dbconn.BuildConfigNode(&dbconn.Config{
			DBType: "mysql", Host: "127.0.0.1", Username: "root", Password: "p", Database: "test",
		})
		if err != nil {
			t.Fatalf("意外错误: %v", err)
		}
		if node.Type != "mysql" || node.Host != "127.0.0.1" || node.Port != "3306" {
			t.Fatalf("字段不符: %+v", node)
		}
		if node.Charset != "utf8mb4" {
			t.Fatalf("mysql 默认字符集应为 utf8mb4: %q", node.Charset)
		}
		if node.User != "root" || node.Name != "test" {
			t.Fatalf("用户/库名不符: %+v", node)
		}
	})

	t.Run("postgres 别名与默认端口", func(t *testing.T) {
		node, err := dbconn.BuildConfigNode(&dbconn.Config{
			DBType: "postgres", Host: "192.168.0.214", Username: "postgres", Database: "test",
		})
		if err != nil {
			t.Fatalf("意外错误: %v", err)
		}
		if node.Type != "pgsql" || node.Port != "5432" {
			t.Fatalf("pgsql 归一/端口不符: %+v", node)
		}
	})

	t.Run("oracle 默认端口", func(t *testing.T) {
		node, err := dbconn.BuildConfigNode(&dbconn.Config{
			DBType: "oracle", Host: "h", Username: "u", Password: "p", Database: "ORCL",
		})
		if err != nil {
			t.Fatalf("意外错误: %v", err)
		}
		if node.Type != "oracle" || node.Port != "1521" || node.Name != "ORCL" {
			t.Fatalf("oracle 字段不符: %+v", node)
		}
	})

	t.Run("dm 默认端口与字符集", func(t *testing.T) {
		node, err := dbconn.BuildConfigNode(&dbconn.Config{
			DBType: "dm", Host: "h", Username: "u", Password: "p", Database: "DMSERVER",
		})
		if err != nil {
			t.Fatalf("意外错误: %v", err)
		}
		if node.Type != "dm" || node.Port != "5236" || node.Charset != "UTF-8" {
			t.Fatalf("dm 字段不符: %+v", node)
		}
	})

	t.Run("mssql 默认端口", func(t *testing.T) {
		node, err := dbconn.BuildConfigNode(&dbconn.Config{
			DBType: "sqlserver", Host: "h", Username: "sa", Password: "p", Database: "master",
		})
		if err != nil {
			t.Fatalf("意外错误: %v", err)
		}
		if node.Type != "mssql" || node.Port != "1433" {
			t.Fatalf("mssql 字段不符: %+v", node)
		}
	})

	t.Run("sqlite 合法配置（无端口要求）", func(t *testing.T) {
		node, err := dbconn.BuildConfigNode(&dbconn.Config{DBType: "sqlite", Database: "/tmp/a.db"})
		if err != nil {
			t.Fatalf("意外错误: %v", err)
		}
		if node.Type != "sqlite" || node.Name != "/tmp/a.db" || node.Port != "" {
			t.Fatalf("sqlite 字段不符: %+v", node)
		}
	})

	t.Run("extra 合法透传", func(t *testing.T) {
		node, err := dbconn.BuildConfigNode(&dbconn.Config{
			DBType: "mysql", Host: "h", Username: "u", Database: "d", Extra: "loc=Local",
		})
		if err != nil {
			t.Fatalf("意外错误: %v", err)
		}
		if node.Extra != "loc=Local" {
			t.Fatalf("extra 应透传: %+v", node)
		}
	})
}
```

- [ ] **Step 2: 运行测试确认失败（包不存在）**

```bash
go test ./internal/dbconn/ -v
```

Expected: FAIL（no required module provides package 或 no Go files）。

- [ ] **Step 3: 实现 dbconn**

创建 `internal/dbconn/dbconn.go`：

```go
/*
 * @desc:统一数据库连接配置解析、校验与初始化（stdio / HTTP 共用）
 */

package dbconn

import (
	"context"
	"fmt"
	"strings"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/text/gstr"

	"github.com/tiger1103/gf-mcp-db/internal/consts"
	"github.com/tiger1103/gf-mcp-db/library/liberr"
)

// Config 数据库连接配置（三种入口共用：CLI 参数 / URL query / HTTP header）
type Config struct {
	DBType   string // mysql | postgres/pgsql | sqlite | sqlserver/mssql | oracle | dm
	Host     string
	Port     string
	Username string
	Password string
	Database string // 数据库名；sqlite 时为文件路径
	Charset  string
	Extra    string // 透传驱动的额外参数，格式 k1=v1&k2=v2
	Debug    bool
}

// typeAliases 类型别名归一表
var typeAliases = map[string]string{
	"mysql":      "mysql",
	"pgsql":      "pgsql",
	"postgres":   "pgsql",
	"postgresql": "pgsql",
	"pg":         "pgsql",
	"sqlite":     "sqlite",
	"sqlite3":    "sqlite",
	"mssql":      "mssql",
	"sqlserver":  "mssql",
	"oracle":     "oracle",
	"dm":         "dm",
}

// defaultPorts 各数据库默认端口
var defaultPorts = map[string]string{
	"mysql":  "3306",
	"pgsql":  "5432",
	"mssql":  "1433",
	"oracle": "1521",
	"dm":     "5236",
}

// NormalizeType 归一数据库类型别名
func NormalizeType(dbType string) (string, bool) {
	t, ok := typeAliases[strings.ToLower(strings.TrimSpace(dbType))]
	return t, ok
}

// BuildConfigNode 归一/校验配置并构建 gdb.ConfigNode。
// 不使用 Link 字符串：直接填充字段，与各 contrib 驱动 Open() 的消费方式一致。
func BuildConfigNode(cfg *Config) (*gdb.ConfigNode, error) {
	dbType, ok := NormalizeType(cfg.DBType)
	if !ok {
		return nil, liberr.NewCode(consts.CodeInfo, fmt.Sprintf(
			"不支持的数据库类型：%q（支持：mysql、postgres/pgsql、sqlite、sqlserver/mssql、oracle、dm）", cfg.DBType))
	}
	if cfg.Extra != "" {
		if _, err := gstr.Parse(cfg.Extra); err != nil {
			return nil, liberr.NewCode(consts.CodeInfo, "extra 参数格式非法，应为 k1=v1&k2=v2："+err.Error())
		}
	}
	node := &gdb.ConfigNode{
		Type:    dbType,
		Host:    strings.TrimSpace(cfg.Host),
		Port:    strings.TrimSpace(cfg.Port),
		User:    cfg.Username,
		Pass:    cfg.Password,
		Name:    cfg.Database,
		Charset: cfg.Charset,
		Extra:   cfg.Extra,
		Debug:   cfg.Debug,
	}
	if node.Port == "" {
		node.Port = defaultPorts[dbType]
	}
	if node.Charset == "" {
		switch dbType {
		case "mysql":
			node.Charset = "utf8mb4"
		case "dm":
			node.Charset = "UTF-8"
		}
	}
	if dbType == "sqlite" {
		if node.Name == "" {
			return nil, liberr.NewCode(consts.CodeInfo, "SQLite 需要提供 database 参数（文件路径）")
		}
	} else {
		if node.Host == "" || node.User == "" || node.Name == "" {
			return nil, liberr.NewCode(consts.CodeInfo,
				"数据库配置不完整，需要：type, host, user, database（port 可省略，使用各库默认端口）")
		}
	}
	return node, nil
}

// Init 构建配置、写入全局默认连接组并 Ping 验证连通性
func Init(ctx context.Context, cfg *Config) error {
	node, err := BuildConfigNode(cfg)
	if err != nil {
		return err
	}
	if err = gdb.SetConfig(gdb.Config{
		gdb.DefaultGroupName: gdb.ConfigGroup{*node},
	}); err != nil {
		return err
	}
	db, err := gdb.Instance(gdb.DefaultGroupName)
	if err != nil {
		return err
	}
	if err = db.PingMaster(); err != nil {
		return err
	}
	return nil
}
```

- [ ] **Step 4: 运行测试确认通过**

```bash
go test ./internal/dbconn/ -v
```

Expected: 全部 PASS。

- [ ] **Step 5: Commit**

```bash
git add internal/dbconn
git commit -m "feat: 新增 dbconn 包（类型归一/按库校验/ConfigNode 直填/连接初始化）"
```

---

### Task 3: dialect 包骨架（接口/注册表/基础能力）

**Files:**
- Create: `internal/mcp/dialect/dialect.go`
- Test: `internal/mcp/dialect/dialect_test.go`

- [ ] **Step 1: 先写失败测试**

创建 `internal/mcp/dialect/dialect_test.go`：

```go
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
```

- [ ] **Step 2: 运行测试确认失败**

```bash
go test ./internal/mcp/dialect/ -v
```

Expected: FAIL（包不存在）。

- [ ] **Step 3: 实现 dialect.go**

创建 `internal/mcp/dialect/dialect.go`：

```go
/*
 * @desc:数据库方言层：封装 gdb 元数据 API 之外的库差异
 * （标识符引用符、LIMIT 语法、索引/外键/表状态查询）
 */

package dialect

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gcode"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/util/gconv"
)

// IDialect 数据库方言接口。
// 约定：Indexes/ForeignKeys/TableStat 返回统一键（见包注释）；
// 查询 SQL 一律使用 ? 占位符（各 contrib 驱动的 DoFilter 负责转换为 $n/@pN/:vN）。
type IDialect interface {
	// Name 方言名（与 gdb 驱动类型一致）
	Name() string
	// QuoteIdent 引用标识符（表名/列名），非法字符直接 panic（工具层在 g.Try 内调用）
	QuoteIdent(name string) string
	// Paginate 为 SELECT 语句施加条数限制
	Paginate(selectSQL string, limit int) string
	// Indexes 表索引列表
	Indexes(ctx context.Context, db gdb.DB, table string) (gdb.Result, error)
	// ForeignKeys 表外键列表
	ForeignKeys(ctx context.Context, db gdb.DB, table string) (gdb.Result, error)
	// TableStat 表统计信息（行数估算/引擎/注释），无对应能力返回 (nil, nil)
	TableStat(ctx context.Context, db gdb.DB, table string) (gdb.Record, error)
}

// 注册表
var dialects = map[string]IDialect{}

// Register 注册方言（各方言文件 init 时调用）
func Register(d IDialect) {
	dialects[d.Name()] = d
}

// Get 获取指定类型的方言
func Get(dbType string) (IDialect, error) {
	if d, ok := dialects[dbType]; ok {
		return d, nil
	}
	return nil, gerror.NewCodef(gcode.CodeInvalidParameter, "数据库类型 %q 没有可用方言", dbType)
}

// identReg 合法标识符（支持 $、#，如 Oracle 派生名）
var identReg = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_$#]*$`)

// BaseDialect 方言基类：默认 LIMIT 分页 + 按引用符引用标识符
type BaseDialect struct {
	name       string
	quoteLeft  string
	quoteRight string
}

// NewBaseDialect 构造基类
func NewBaseDialect(name, quoteLeft, quoteRight string) BaseDialect {
	return BaseDialect{name: name, quoteLeft: quoteLeft, quoteRight: quoteRight}
}

// Name 返回方言名
func (d *BaseDialect) Name() string { return d.name }

// QuoteIdent 引用标识符，支持 a.b 形式（逐段引用）
func (d *BaseDialect) QuoteIdent(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		panic(gerror.NewCode(gcode.CodeInvalidParameter, "标识符不能为空"))
	}
	parts := strings.Split(name, ".")
	quoted := make([]string, 0, len(parts))
	for _, p := range parts {
		if !identReg.MatchString(p) {
			panic(gerror.NewCodef(gcode.CodeInvalidParameter, "非法标识符：%q", p))
		}
		quoted = append(quoted, d.quoteLeft+p+d.quoteRight)
	}
	return strings.Join(quoted, ".")
}

// Paginate 默认实现：LIMIT n
func (d *BaseDialect) Paginate(selectSQL string, limit int) string {
	if limit <= 0 {
		limit = 100
	}
	return fmt.Sprintf("%s LIMIT %d", strings.TrimSpace(selectSQL), limit)
}

// MatchPattern 判断表名是否匹配 SQL LIKE 风格模式（% 任意串、_ 单字符，大小写不敏感）
func MatchPattern(name, pattern string) bool {
	if strings.TrimSpace(pattern) == "" {
		return true
	}
	var sb strings.Builder
	for _, r := range pattern {
		switch r {
		case '%':
			sb.WriteString(".*")
		case '_':
			sb.WriteString(".")
		default:
			sb.WriteString(regexp.QuoteMeta(string(r)))
		}
	}
	re, err := regexp.Compile(`(?i)^` + sb.String() + `$`)
	if err != nil {
		return true
	}
	return re.MatchString(name)
}

// ColumnsFromTableFields 将 gdb.TableFields 转为统一列结构（按列序输出）
func ColumnsFromTableFields(fields map[string]*gdb.TableField) []map[string]string {
	type indexedCol struct {
		index int
		col   map[string]string
	}
	cols := make([]indexedCol, 0, len(fields))
	for _, f := range fields {
		cols = append(cols, indexedCol{
			index: f.Index,
			col: map[string]string{
				"column_name": f.Name,
				"data_type":   f.Type,
				"nullable":    strconv.FormatBool(f.Null),
				"primary_key": strconv.FormatBool(strings.EqualFold(f.Key, "PRI")),
				"default":     gconv.String(f.Default),
				"extra":       f.Extra,
				"comment":     f.Comment,
			},
		})
	}
	sort.Slice(cols, func(i, j int) bool { return cols[i].index < cols[j].index })
	out := make([]map[string]string, 0, len(cols))
	for _, c := range cols {
		out = append(out, c.col)
	}
	return out
}
```

- [ ] **Step 4: 运行测试确认通过**

```bash
go test ./internal/mcp/dialect/ -v
```

Expected: 全部 PASS。

- [ ] **Step 5: Commit**

```bash
git add internal/mcp/dialect
git commit -m "feat: 新增 dialect 方言层骨架（接口/注册表/引用符/LIMIT/模式匹配/统一列结构）"
```

---

### Task 4: MySQL 方言

**Files:**
- Create: `internal/mcp/dialect/mysql.go`
- Test: `internal/mcp/dialect/mysql_test.go`

- [ ] **Step 1: 先写失败测试**

创建 `internal/mcp/dialect/mysql_test.go`：

```go
package dialect_test

import (
	"testing"

	"github.com/tiger1103/gf-mcp-db/internal/mcp/dialect"
)

func TestMysqlDialectBasics(t *testing.T) {
	d, err := dialect.Get("mysql")
	if err != nil {
		t.Fatalf("mysql 方言未注册: %v", err)
	}
	if got := d.QuoteIdent("users"); got != "`users`" {
		t.Fatalf("QuoteIdent = %q", got)
	}
	if got := d.Paginate("SELECT * FROM t", 5); got != "SELECT * FROM t LIMIT 5" {
		t.Fatalf("Paginate = %q", got)
	}
}
```

- [ ] **Step 2: 运行确认失败**

```bash
go test ./internal/mcp/dialect/ -run TestMysqlDialectBasics -v
```

Expected: FAIL（mysql 方言未注册）。

- [ ] **Step 3: 实现 mysql.go**

创建 `internal/mcp/dialect/mysql.go`：

```go
/*
 * @desc:MySQL 方言
 */

package dialect

import (
	"context"
	"fmt"

	"github.com/gogf/gf/v2/database/gdb"
)

func init() {
	Register(NewMysqlDialect())
}

// NewMysqlDialect 构造 MySQL 方言
func NewMysqlDialect() IDialect {
	return &mysqlDialect{BaseDialect: NewBaseDialect("mysql", "`", "`")}
}

type mysqlDialect struct {
	BaseDialect
}

// Indexes 索引列表（SHOW INDEX）
func (d *mysqlDialect) Indexes(ctx context.Context, db gdb.DB, table string) (gdb.Result, error) {
	res, err := db.Query(ctx, fmt.Sprintf("SHOW INDEX FROM %s", d.QuoteIdent(table)))
	if err != nil {
		return nil, err
	}
	out := make(gdb.Result, 0, len(res))
	for _, r := range res {
		out = append(out, gdb.Record{
			"index_name": r["Key_name"],
			"column_name": r["Column_name"],
			"is_unique":  !r["Non_unique"].Bool(),
		})
	}
	return out, nil
}

// ForeignKeys 外键列表（information_schema，输出键统一小写）
func (d *mysqlDialect) ForeignKeys(ctx context.Context, db gdb.DB, table string) (gdb.Result, error) {
	res, err := db.Query(ctx, `
		SELECT CONSTRAINT_NAME AS constraint_name, COLUMN_NAME AS column_name,
		       REFERENCED_TABLE_NAME AS referenced_table_name, REFERENCED_COLUMN_NAME AS referenced_column_name
		FROM information_schema.KEY_COLUMN_USAGE
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? AND REFERENCED_TABLE_NAME IS NOT NULL`, table)
	if err != nil {
		return nil, err
	}
	return res, nil
}

// TableStat 表状态（SHOW TABLE STATUS）
func (d *mysqlDialect) TableStat(ctx context.Context, db gdb.DB, table string) (gdb.Record, error) {
	res, err := db.Query(ctx, "SHOW TABLE STATUS WHERE Name = ?", table)
	if err != nil {
		return nil, err
	}
	if len(res) == 0 {
		return nil, nil
	}
	r := res[0]
	return gdb.Record{
		"engine":        r["Engine"],
		"rows_estimate": r["Rows"],
		"table_comment": r["Comment"],
		"data_length":   r["Data_length"],
		"index_length":  r["Index_length"],
	}, nil
}
```

- [ ] **Step 4: 运行测试确认通过**

```bash
go test ./internal/mcp/dialect/ -v
```

Expected: 全部 PASS。

- [ ] **Step 5: Commit**

```bash
git add internal/mcp/dialect
git commit -m "feat: MySQL 方言（SHOW INDEX/KEY_COLUMN_USAGE/TABLE STATUS，统一输出键）"
```

---

### Task 5: PostgreSQL 方言

**Files:**
- Create: `internal/mcp/dialect/pgsql.go`
- Test: `internal/mcp/dialect/pgsql_test.go`

- [ ] **Step 1: 先写失败测试**

创建 `internal/mcp/dialect/pgsql_test.go`：

```go
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
}
```

- [ ] **Step 2: 运行确认失败**

```bash
go test ./internal/mcp/dialect/ -run TestPgsqlDialectBasics -v
```

Expected: FAIL。

- [ ] **Step 3: 实现 pgsql.go**

创建 `internal/mcp/dialect/pgsql.go`：

```go
/*
 * @desc:PostgreSQL 方言
 */

package dialect

import (
	"context"
	"fmt"
	"strings"

	"github.com/gogf/gf/v2/database/gdb"
)

func init() {
	Register(NewPgsqlDialect())
}

// NewPgsqlDialect 构造 PostgreSQL 方言
func NewPgsqlDialect() IDialect {
	return &pgsqlDialect{BaseDialect: NewBaseDialect("pgsql", `"`, `"`)}
}

type pgsqlDialect struct {
	BaseDialect
}

// Indexes 索引列表（pg_index / pg_attribute）
func (d *pgsqlDialect) Indexes(ctx context.Context, db gdb.DB, table string) (gdb.Result, error) {
	res, err := db.Query(ctx, `
		SELECT i.relname AS index_name, a.attname AS column_name, ix.indisunique AS is_unique
		FROM pg_class t
		JOIN pg_index ix ON ix.indrelid = t.oid
		JOIN pg_class i ON i.oid = ix.indexrelid
		JOIN pg_attribute a ON a.attrelid = t.oid AND a.attnum = ANY(ix.indkey)
		JOIN pg_namespace n ON n.oid = t.relnamespace
		WHERE t.relname = ? AND n.nspname = current_schema() AND i.relname NOT LIKE 'pg\_%'`, table)
	if err != nil {
		return nil, err
	}
	out := make(gdb.Result, 0, len(res))
	for _, r := range res {
		out = append(out, gdb.Record{
			"index_name":  r["index_name"],
			"column_name": r["column_name"],
			"is_unique":   r["is_unique"].Bool(),
		})
	}
	return out, nil
}

// ForeignKeys 外键列表（information_schema）
func (d *pgsqlDialect) ForeignKeys(ctx context.Context, db gdb.DB, table string) (gdb.Result, error) {
	res, err := db.Query(ctx, `
		SELECT tc.constraint_name AS constraint_name, kcu.column_name AS column_name,
		       ccu.table_name AS referenced_table_name, ccu.column_name AS referenced_column_name
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu
		  ON tc.constraint_name = kcu.constraint_name AND tc.constraint_schema = kcu.constraint_schema
		JOIN information_schema.constraint_column_usage ccu
		  ON ccu.constraint_name = tc.constraint_name AND ccu.constraint_schema = tc.constraint_schema
		WHERE tc.constraint_type = 'FOREIGN KEY' AND tc.table_name = ?`, table)
	if err != nil {
		return nil, err
	}
	return res, nil
}

// TableStat 行数估算与表注释（pg_class）；reltuples < 0 表示未 ANALYZE，省略该键
func (d *pgsqlDialect) TableStat(ctx context.Context, db gdb.DB, table string) (gdb.Record, error) {
	res, err := db.Query(ctx, `
		SELECT c.reltuples::bigint AS rows_estimate, obj_description(c.oid, 'pg_class') AS table_comment
		FROM pg_class c
		WHERE c.relname = ? AND c.relkind IN ('r','p')`, table)
	if err != nil {
		return nil, err
	}
	if len(res) == 0 {
		return nil, nil
	}
	r := res[0]
	stat := gdb.Record{"table_comment": r["table_comment"]}
	if rows := r["rows_estimate"].Int(); rows >= 0 {
		stat["rows_estimate"] = rows
	}
	return stat, nil
}

// 编译期确认 strings 被使用（is_unique 等映射保留扩展点）
var _ = strings.TrimSpace
```

注意：`var _ = strings.TrimSpace` 是占位扩展点，若实现中未用到 strings 也可直接删除该行与 import。**实现时优先删除未用 import，保持 go vet 干净。**

- [ ] **Step 4: 运行测试确认通过**

```bash
go test ./internal/mcp/dialect/ -v
```

Expected: 全部 PASS。若 strings 未使用报编译错误，删除 `import "strings"` 与占位行。

- [ ] **Step 5: Commit**

```bash
git add internal/mcp/dialect
git commit -m "feat: PostgreSQL 方言（pg_index/constraint_column_usage/reltuples）"
```

---

### Task 6: SQLite 方言

**Files:**
- Create: `internal/mcp/dialect/sqlite.go`
- Test: `internal/mcp/dialect/sqlite_test.go`

- [ ] **Step 1: 先写失败测试**

创建 `internal/mcp/dialect/sqlite_test.go`：

```go
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
	if got := d.QuoteIdent("users"); got != "`users`" {
		t.Fatalf("QuoteIdent = %q", got)
	}
	if got := d.Paginate("SELECT * FROM t", 5); got != "SELECT * FROM t LIMIT 5" {
		t.Fatalf("Paginate = %q", got)
	}
}
```

- [ ] **Step 2: 运行确认失败**

```bash
go test ./internal/mcp/dialect/ -run TestSqliteDialectBasics -v
```

Expected: FAIL。

- [ ] **Step 3: 实现 sqlite.go**

创建 `internal/mcp/dialect/sqlite.go`：

```go
/*
 * @desc:SQLite 方言（PRAGMA 不支持参数绑定，标识符经 QuoteIdent 白名单校验后拼接）
 */

package dialect

import (
	"context"
	"fmt"

	"github.com/gogf/gf/v2/database/gdb"
)

func init() {
	Register(NewSqliteDialect())
}

// NewSqliteDialect 构造 SQLite 方言
func NewSqliteDialect() IDialect {
	return &sqliteDialect{BaseDialect: NewBaseDialect("sqlite", "`", "`")}
}

type sqliteDialect struct {
	BaseDialect
}

// Indexes 索引列表（PRAGMA index_list + index_info）
func (d *sqliteDialect) Indexes(ctx context.Context, db gdb.DB, table string) (gdb.Result, error) {
	res, err := db.Query(ctx, fmt.Sprintf("PRAGMA index_list(%s)", d.QuoteIdent(table)))
	if err != nil {
		return nil, err
	}
	out := gdb.Result{}
	for _, r := range res {
		indexName := r["name"].String()
		isUnique := r["unique"].Bool()
		colRes, colErr := db.Query(ctx, fmt.Sprintf("PRAGMA index_info(%s)", d.QuoteIdent(indexName)))
		if colErr != nil {
			return nil, colErr
		}
		for _, cr := range colRes {
			out = append(out, gdb.Record{
				"index_name":  indexName,
				"column_name": cr["name"],
				"is_unique":   isUnique,
			})
		}
	}
	return out, nil
}

// ForeignKeys 外键列表（PRAGMA foreign_key_list）
func (d *sqliteDialect) ForeignKeys(ctx context.Context, db gdb.DB, table string) (gdb.Result, error) {
	res, err := db.Query(ctx, fmt.Sprintf("PRAGMA foreign_key_list(%s)", d.QuoteIdent(table)))
	if err != nil {
		return nil, err
	}
	out := gdb.Result{}
	for _, r := range res {
		out = append(out, gdb.Record{
			"constraint_name":        fmt.Sprintf("fk_%s_%s", table, r["id"].String()),
			"column_name":            r["from"],
			"referenced_table_name":  r["table"],
			"referenced_column_name": r["to"],
		})
	}
	return out, nil
}

// TableStat SQLite 无行数估算能力
func (d *sqliteDialect) TableStat(ctx context.Context, db gdb.DB, table string) (gdb.Record, error) {
	return nil, nil
}
```

- [ ] **Step 4: 运行测试确认通过**

```bash
go test ./internal/mcp/dialect/ -v
```

Expected: 全部 PASS。

- [ ] **Step 5: Commit**

```bash
git add internal/mcp/dialect
git commit -m "feat: SQLite 方言（PRAGMA index_list/index_info/foreign_key_list）"
```

---

### Task 7: SQL Server 方言

**Files:**
- Create: `internal/mcp/dialect/mssql.go`
- Test: `internal/mcp/dialect/mssql_test.go`

- [ ] **Step 1: 先写失败测试**

创建 `internal/mcp/dialect/mssql_test.go`：

```go
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
	if got := d.QuoteIdent("users"); got != `"users"` {
		t.Fatalf("QuoteIdent = %q", got)
	}
	cases := []struct{ in string; limit int; want string }{
		{"SELECT * FROM t", 10, "SELECT TOP 10 * FROM t"},
		{"SELECT DISTINCT a FROM t", 10, "SELECT DISTINCT TOP 10 a FROM t"},
		{"SELECT a FROM t ORDER BY b", 1, "SELECT TOP 1 a FROM t ORDER BY b"},
		{"SELECT * FROM t", 0, "SELECT TOP 100 * FROM t"},
	}
	for _, c := range cases {
		if got := d.Paginate(c.in, c.limit); got != c.want {
			t.Fatalf("Paginate(%q,%d) = %q, want %q", c.in, c.limit, got, c.want)
		}
	}
}
```

- [ ] **Step 2: 运行确认失败**

```bash
go test ./internal/mcp/dialect/ -run TestMssqlDialectBasics -v
```

Expected: FAIL。

- [ ] **Step 3: 实现 mssql.go**

创建 `internal/mcp/dialect/mssql.go`：

```go
/*
 * @desc:SQL Server 方言
 */

package dialect

import (
	"context"
	"strconv"
	"strings"

	"github.com/gogf/gf/v2/database/gdb"
)

func init() {
	Register(NewMssqlDialect())
}

// NewMssqlDialect 构造 SQL Server 方言（引用符与 gf mssql 驱动一致，为双引号）
func NewMssqlDialect() IDialect {
	return &mssqlDialect{BaseDialect: NewBaseDialect("mssql", `"`, `"`)}
}

type mssqlDialect struct {
	BaseDialect
}

// Paginate TOP 语法（兼容 SELECT DISTINCT）
func (d *mssqlDialect) Paginate(selectSQL string, limit int) string {
	if limit <= 0 {
		limit = 100
	}
	s := strings.TrimSpace(selectSQL)
	upper := strings.ToUpper(s)
	if strings.HasPrefix(upper, "SELECT DISTINCT") {
		return "SELECT DISTINCT TOP " + strconv.Itoa(limit) + s[len("SELECT DISTINCT"):]
	}
	if strings.HasPrefix(upper, "SELECT") {
		return "SELECT TOP " + strconv.Itoa(limit) + s[len("SELECT"):]
	}
	return s
}

// Indexes 索引列表（sys.indexes / sys.index_columns）
func (d *mssqlDialect) Indexes(ctx context.Context, db gdb.DB, table string) (gdb.Result, error) {
	res, err := db.Query(ctx, `
		SELECT i.name AS index_name, c.name AS column_name, i.is_unique AS is_unique
		FROM sys.indexes i
		JOIN sys.tables t ON t.object_id = i.object_id
		JOIN sys.index_columns ic ON ic.object_id = i.object_id AND ic.index_id = i.index_id
		JOIN sys.columns c ON c.object_id = ic.object_id AND c.column_id = ic.column_id
		WHERE t.name = ? AND i.name IS NOT NULL
		ORDER BY i.name, ic.key_ordinal`, table)
	if err != nil {
		return nil, err
	}
	out := make(gdb.Result, 0, len(res))
	for _, r := range res {
		out = append(out, gdb.Record{
			"index_name":  r["index_name"],
			"column_name": r["column_name"],
			"is_unique":   r["is_unique"].Bool(),
		})
	}
	return out, nil
}

// ForeignKeys 外键列表（sys.foreign_keys）
func (d *mssqlDialect) ForeignKeys(ctx context.Context, db gdb.DB, table string) (gdb.Result, error) {
	res, err := db.Query(ctx, `
		SELECT fk.name AS constraint_name, cp.name AS column_name,
		       tr.name AS referenced_table_name, cr.name AS referenced_column_name
		FROM sys.foreign_keys fk
		JOIN sys.tables tp ON tp.object_id = fk.parent_object_id
		JOIN sys.foreign_key_columns fkc ON fkc.constraint_object_id = fk.object_id
		JOIN sys.columns cp ON cp.object_id = fkc.parent_object_id AND cp.column_id = fkc.parent_column_id
		JOIN sys.tables tr ON tr.object_id = fkc.referenced_object_id
		JOIN sys.columns cr ON cr.object_id = fkc.referenced_object_id AND cr.column_id = fkc.referenced_column_id
		WHERE tp.name = ?`, table)
	if err != nil {
		return nil, err
	}
	return res, nil
}

// TableStat 行数估算（sys.partitions）
func (d *mssqlDialect) TableStat(ctx context.Context, db gdb.DB, table string) (gdb.Record, error) {
	res, err := db.Query(ctx, `
		SELECT SUM(p.rows) AS rows_estimate
		FROM sys.partitions p
		JOIN sys.tables t ON t.object_id = p.object_id
		WHERE t.name = ? AND p.index_id IN (0,1)`, table)
	if err != nil {
		return nil, err
	}
	if len(res) == 0 {
		return nil, nil
	}
	return gdb.Record{"rows_estimate": res[0]["rows_estimate"]}, nil
}
```

- [ ] **Step 4: 运行测试确认通过**

```bash
go test ./internal/mcp/dialect/ -v
```

Expected: 全部 PASS。

- [ ] **Step 5: Commit**

```bash
git add internal/mcp/dialect
git commit -m "feat: SQL Server 方言（TOP 分页/sys 目录视图）"
```

---

### Task 8: Oracle 与 DM 方言（共享 Oracle 兼容实现）

**Files:**
- Create: `internal/mcp/dialect/oracle.go`
- Create: `internal/mcp/dialect/dm.go`
- Test: `internal/mcp/dialect/oracle_dm_test.go`

- [ ] **Step 1: 先写失败测试**

创建 `internal/mcp/dialect/oracle_dm_test.go`：

```go
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
	if got := d.QuoteIdent("USERS"); got != `"USERS"` {
		t.Fatalf("QuoteIdent = %q", got)
	}
	if got := d.Paginate("SELECT * FROM t", 5); got != "SELECT * FROM t FETCH FIRST 5 ROWS ONLY" {
		t.Fatalf("Paginate = %q", got)
	}
}

func TestDmDialectBasics(t *testing.T) {
	d, err := dialect.Get("dm")
	if err != nil {
		t.Fatalf("dm 方言未注册: %v", err)
	}
	if got := d.QuoteIdent("USERS"); got != `"USERS"` {
		t.Fatalf("QuoteIdent = %q", got)
	}
	// DM8 兼容 MySQL 的 LIMIT 语法
	if got := d.Paginate("SELECT * FROM t", 5); got != "SELECT * FROM t LIMIT 5" {
		t.Fatalf("Paginate = %q", got)
	}
}
```

- [ ] **Step 2: 运行确认失败**

```bash
go test ./internal/mcp/dialect/ -run "TestOracleDialectBasics|TestDmDialectBasics" -v
```

Expected: FAIL。

- [ ] **Step 3: 实现 oracle.go（含 oracleLike 共享逻辑）**

创建 `internal/mcp/dialect/oracle.go`：

```go
/*
 * @desc:Oracle 方言（与 DM 共享 Oracle 兼容目录视图实现）
 */

package dialect

import (
	"context"
	"fmt"
	"strings"

	"github.com/gogf/gf/v2/database/gdb"
)

func init() {
	Register(NewOracleDialect())
}

// NewOracleDialect 构造 Oracle 方言
func NewOracleDialect() IDialect {
	return &oracleDialect{oracleLikeDialect{BaseDialect: NewBaseDialect("oracle", `"`, `"`)}}
}

type oracleDialect struct {
	oracleLikeDialect
}

// Paginate FETCH FIRST（Oracle 12c+）
func (d *oracleDialect) Paginate(selectSQL string, limit int) string {
	if limit <= 0 {
		limit = 100
	}
	return fmt.Sprintf("%s FETCH FIRST %d ROWS ONLY", strings.TrimSpace(selectSQL), limit)
}

// oracleLikeDialect Oracle 兼容目录视图共享实现（Oracle / DM）
type oracleLikeDialect struct {
	BaseDialect
}

// Indexes 索引列表（ALL_INDEXES + ALL_IND_COLUMNS，限当前用户）
func (d *oracleLikeDialect) Indexes(ctx context.Context, db gdb.DB, table string) (gdb.Result, error) {
	res, err := db.Query(ctx, `
		SELECT ic.INDEX_NAME, ic.COLUMN_NAME, ix.UNIQUENESS
		FROM ALL_INDEXES ix
		JOIN ALL_IND_COLUMNS ic
		  ON ic.INDEX_OWNER = ix.OWNER AND ic.INDEX_NAME = ix.INDEX_NAME
		WHERE ix.TABLE_NAME = ? AND ix.OWNER = USER
		ORDER BY ic.INDEX_NAME, ic.COLUMN_POSITION`, table)
	if err != nil {
		return nil, err
	}
	out := make(gdb.Result, 0, len(res))
	for _, r := range res {
		out = append(out, gdb.Record{
			"index_name":  r["INDEX_NAME"],
			"column_name": r["COLUMN_NAME"],
			"is_unique":   strings.EqualFold(r["UNIQUENESS"].String(), "UNIQUE"),
		})
	}
	return out, nil
}

// ForeignKeys 外键列表（ALL_CONSTRAINTS 约束类型 R）
func (d *oracleLikeDialect) ForeignKeys(ctx context.Context, db gdb.DB, table string) (gdb.Result, error) {
	res, err := db.Query(ctx, `
		SELECT ac.CONSTRAINT_NAME, acc.COLUMN_NAME,
		       rc.TABLE_NAME AS REFERENCED_TABLE_NAME, rcc.COLUMN_NAME AS REFERENCED_COLUMN_NAME
		FROM ALL_CONSTRAINTS ac
		JOIN ALL_CONS_COLUMNS acc
		  ON acc.OWNER = ac.OWNER AND acc.CONSTRAINT_NAME = ac.CONSTRAINT_NAME
		JOIN ALL_CONSTRAINTS rc
		  ON rc.OWNER = ac.R_OWNER AND rc.CONSTRAINT_NAME = ac.R_CONSTRAINT_NAME
		JOIN ALL_CONS_COLUMNS rcc
		  ON rcc.OWNER = rc.OWNER AND rcc.CONSTRAINT_NAME = rc.CONSTRAINT_NAME AND rcc.POSITION = acc.POSITION
		WHERE ac.CONSTRAINT_TYPE = 'R' AND ac.TABLE_NAME = ? AND ac.OWNER = USER
		ORDER BY ac.CONSTRAINT_NAME, acc.POSITION`, table)
	if err != nil {
		return nil, err
	}
	out := make(gdb.Result, 0, len(res))
	for _, r := range res {
		out = append(out, gdb.Record{
			"constraint_name":        r["CONSTRAINT_NAME"],
			"column_name":            r["COLUMN_NAME"],
			"referenced_table_name":  r["REFERENCED_TABLE_NAME"],
			"referenced_column_name": r["REFERENCED_COLUMN_NAME"],
		})
	}
	return out, nil
}

// TableStat 行数估算与表注释（ALL_TABLES + ALL_TAB_COMMENTS）；NUM_ROWS < 0 省略
func (d *oracleLikeDialect) TableStat(ctx context.Context, db gdb.DB, table string) (gdb.Record, error) {
	res, err := db.Query(ctx, `
		SELECT t.NUM_ROWS AS ROWS_ESTIMATE, tc.COMMENTS AS TABLE_COMMENT
		FROM ALL_TABLES t
		LEFT JOIN ALL_TAB_COMMENTS tc
		  ON tc.OWNER = t.OWNER AND tc.TABLE_NAME = t.TABLE_NAME
		WHERE t.TABLE_NAME = ? AND t.OWNER = USER`, table)
	if err != nil {
		return nil, err
	}
	if len(res) == 0 {
		return nil, nil
	}
	r := res[0]
	stat := gdb.Record{"table_comment": r["TABLE_COMMENT"]}
	if rows := r["ROWS_ESTIMATE"].Int(); rows >= 0 {
		stat["rows_estimate"] = rows
	}
	return stat, nil
}
```

- [ ] **Step 4: 实现 dm.go**

创建 `internal/mcp/dialect/dm.go`：

```go
/*
 * @desc:达梦（DM8）方言：复用 Oracle 兼容目录视图；分页用 DM8 支持的 LIMIT 语法
 */

package dialect

func init() {
	Register(NewDmDialect())
}

// NewDmDialect 构造 DM 方言
func NewDmDialect() IDialect {
	return &dmDialect{oracleLikeDialect{BaseDialect: NewBaseDialect("dm", `"`, `"`)}}
}

type dmDialect struct {
	oracleLikeDialect
}
```

注意：DM 元数据查询按大写表名匹配（DM 将未加引号的标识符存为大写），`get_table_info` 传小写表名时需以大写重试 —— 该重试逻辑统一放在 Task 11 的 `get_table_info` 实现中（`oracleLikeDialect` 系方言的小写表名会被 `?` 绑定原样传递，若首次结果为空则用 `strings.ToUpper(table)` 重试一次）。

- [ ] **Step 5: 运行测试确认通过**

```bash
go test ./internal/mcp/dialect/ -v
```

Expected: 全部 PASS。

- [ ] **Step 6: Commit**

```bash
git add internal/mcp/dialect
git commit -m "feat: Oracle/DM 方言（共享 all_* 目录视图，FETCH FIRST/LIMIT 分页）"
```

---

### Task 9: 工具层公共助手（helpers.go）与查询判定

**Files:**
- Create: `internal/mcp/tools/helpers.go`
- Create: `internal/mcp/tools/execute_query_test.go`（先只放 isQuerySQL 测试）

- [ ] **Step 1: 先写失败测试**

创建 `internal/mcp/tools/execute_query_test.go`：

```go
/*
 * @desc:execute_query 单元测试（纯函数部分）
 */

package tools

import "testing"

func TestIsQuerySQL(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"SELECT * FROM t", true},
		{"  select 1", true},
		{"WITH c AS (SELECT 1) SELECT * FROM c", true},
		{"SHOW TABLES", true},
		{"EXPLAIN SELECT 1", true},
		{"DESC users", true},
		{"DESCRIBE users", true},
		{"PRAGMA table_info(users)", true},
		{"INSERT INTO t VALUES (1)", false},
		{"UPDATE t SET a=1", false},
		{"DELETE FROM t", false},
		{"CREATE TABLE t (id INT)", false},
		{"DROP TABLE t", false},
	}
	for _, c := range cases {
		if got := isQuerySQL(c.in); got != c.want {
			t.Fatalf("isQuerySQL(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}
```

- [ ] **Step 2: 运行确认失败**

```bash
go test ./internal/mcp/tools/ -run TestIsQuerySQL -v
```

Expected: FAIL（isQuerySQL 未定义）。

- [ ] **Step 3: 实现 helpers.go（含 isQuerySQL）**

创建 `internal/mcp/tools/helpers.go`：

```go
/*
 * @desc:工具层公共助手：连接获取、方言获取、参数读取、SQL 类型判定
 */

package tools

import (
	"context"
	"errors"
	"strings"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/util/gconv"

	"github.com/tiger1103/gf-mcp-db/internal/consts"
	"github.com/tiger1103/gf-mcp-db/internal/mcp/dialect"
	"github.com/tiger1103/gf-mcp-db/library/liberr"
)

// errNoDatabase 未连接数据库的统一错误文案
var errNoDatabase = errors.New("请先连接数据库，在建立 MCP 连接时提供数据库配置参数")

// getDB 获取默认数据库连接（未配置时经 liberr 抛出可读错误）
func getDB(ctx context.Context) gdb.DB {
	var db gdb.DB
	g.TryCatch(ctx, func(ctx context.Context) {
		db = g.DB("default")
	}, func(ctx context.Context, exception error) {
		g.Log().Error(ctx, exception.Error())
		liberr.ErrIsNilCode(ctx, errNoDatabase, consts.CodeInfo)
	})
	if db == nil {
		liberr.ErrIsNilCode(ctx, errNoDatabase, consts.CodeInfo)
	}
	return db
}

// currentDialect 依据当前连接的数据库类型获取方言实现
func currentDialect(ctx context.Context, db gdb.DB) dialect.IDialect {
	d, err := dialect.Get(db.GetConfig().Type)
	liberr.ErrIsNilCode(ctx, err, consts.CodeInfo)
	return d
}

// argString 读取字符串参数（缺省为空串）
func argString(args map[string]any, key string) string {
	v, _ := args[key].(string)
	return v
}

// requireArgString 读取必填字符串参数（空值 panic，由 g.Try 统一恢复）
func requireArgString(args map[string]any, key string) string {
	v := argString(args, key)
	if v == "" {
		panic(liberr.NewCode(consts.CodeInfo, key+" 参数必须是非空字符串"))
	}
	return v
}

// argInt 读取正整数参数（MCP JSON 数值可能为 float64，统一经 gconv 转换；非法回落默认值）
func argInt(args map[string]any, key string, def int) int {
	v, ok := args[key]
	if !ok || v == nil {
		return def
	}
	if n := gconv.Int(v); n > 0 {
		return n
	}
	return def
}

// isQuerySQL 判断 SQL 是否为返回结果集的语句（前缀判定，供 execute_query 使用）
func isQuerySQL(sql string) bool {
	upper := strings.TrimSpace(strings.ToUpper(sql))
	for _, p := range []string{"SELECT", "WITH", "SHOW", "EXPLAIN", "DESCRIBE", "DESC", "PRAGMA"} {
		if strings.HasPrefix(upper, p) {
			return true
		}
	}
	return false
}
```

- [ ] **Step 4: 运行测试确认通过**

```bash
go test ./internal/mcp/tools/ -run TestIsQuerySQL -v
```

Expected: PASS。

- [ ] **Step 5: Commit**

```bash
git add internal/mcp/tools
git commit -m "feat: 工具层公共助手（getDB/方言获取/参数读取兼容 JSON 数值/isQuerySQL）"
```

---

### Task 10: 重构 get_table_list 与 get_schema

**Files:**
- Modify: `internal/mcp/tools/get_table_list.go`（整体替换）
- Modify: `internal/mcp/tools/get_schema.go`（整体替换）

- [ ] **Step 1: 替换 get_table_list.go**

```go
/*
 * @desc:列出数据库表工具
 */

package tools

import (
	"context"
	"fmt"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/util/gconv"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/tiger1103/gf-mcp-db/internal/mcp/dialect"
	"github.com/tiger1103/gf-mcp-db/internal/mcp/register"
	"github.com/tiger1103/gf-mcp-db/library/liberr"
)

// ListTables 列出数据库表工具结构
type ListTables struct{}

// ReturnTool 返回工具定义
func (t *ListTables) ReturnTool() mcp.Tool {
	return mcp.NewTool("get_table_list",
		mcp.WithDescription(`# 📋 列出数据库表

## 🎯 工具功能
列出当前数据库中的所有表或匹配特定模式的表。

## 💡 使用示例
列出所有表:
{}

列出匹配 user 的表:
{
  "pattern": "user%"
}`),
		mcp.WithString("pattern",
			mcp.Description("表名匹配模式，支持通配符 %，例如 'user%' 匹配所有以 user 开头的表名")),
	)
}

// Handler 工具处理函数
func (t *ListTables) Handler(r *Reg) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var result string
		err := g.Try(ctx, func(ctx context.Context) {
			db := getDB(ctx)

			tables, tablesErr := db.Tables(ctx)
			liberr.ErrIsNil(ctx, tablesErr)

			pattern := argString(request.GetArguments(), "pattern")
			names := make([]string, 0, len(tables))
			for _, name := range tables {
				if dialect.MatchPattern(name, pattern) {
					names = append(names, name)
				}
			}

			result = fmt.Sprintf("当前数据库中共有 %d 个表，表名列表：%s", len(names), gconv.String(names))
		})

		if err != nil {
			return r.returnRes(err)
		}

		return mcp.NewToolResultText(result), nil
	}
}

// RegisterListTables 注册列出表工具
func (r *Reg) RegisterListTables() {
	register.AddHandler(func(mcpServer *server.MCPServer) {
		tool := new(ListTables)
		mcpServer.AddTool(tool.ReturnTool(), tool.Handler(r))
	})
}
```

- [ ] **Step 2: 替换 get_schema.go**

```go
/*
 * @desc:获取数据库结构信息工具
 */

package tools

import (
	"context"
	"fmt"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/util/gconv"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/tiger1103/gf-mcp-db/internal/mcp/dialect"
	"github.com/tiger1103/gf-mcp-db/internal/mcp/register"
	"github.com/tiger1103/gf-mcp-db/library/liberr"
)

// GetSchema 获取数据库结构信息工具结构
type GetSchema struct{}

// ReturnTool 返回工具定义
func (t *GetSchema) ReturnTool() mcp.Tool {
	return mcp.NewTool("get_schema",
		mcp.WithDescription(`# 📐 获取数据库结构信息

## 🎯 工具功能
获取数据库的完整结构信息，包括所有表名、列名、数据类型、主键、索引等元数据。

## 💡 使用示例
获取所有表结构:
{}

获取匹配 user 的表结构:
{
  "pattern": "user%"
}`),
		mcp.WithString("pattern",
			mcp.Description("表名匹配模式，支持通配符 %，例如 'user%' 匹配所有以 user 开头的表名")),
	)
}

// Handler 工具处理函数
func (t *GetSchema) Handler(r *Reg) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var result string
		err := g.Try(ctx, func(ctx context.Context) {
			db := getDB(ctx)
			d := currentDialect(ctx, db)

			tables, tablesErr := db.Tables(ctx)
			liberr.ErrIsNil(ctx, tablesErr)

			pattern := argString(request.GetArguments(), "pattern")
			schemaInfo := make([]map[string]any, 0, len(tables))
			for _, tableName := range tables {
				if !dialect.MatchPattern(tableName, pattern) {
					continue
				}
				entry := map[string]any{"table_name": tableName}

				fields, fieldsErr := db.TableFields(ctx, tableName)
				if fieldsErr == nil {
					entry["columns"] = dialect.ColumnsFromTableFields(fields)
				} else {
					entry["columns_error"] = fieldsErr.Error()
				}

				if indexes, indexesErr := d.Indexes(ctx, db, tableName); indexesErr == nil {
					entry["indexes"] = indexes
				}

				schemaInfo = append(schemaInfo, entry)
			}

			result = fmt.Sprintf("数据库结构信息：共 %d 个表，详细信息：%s", len(schemaInfo), gconv.String(schemaInfo))
		})

		if err != nil {
			return r.returnRes(err)
		}

		return mcp.NewToolResultText(result), nil
	}
}

// RegisterGetSchema 注册获取数据库结构信息工具
func (r *Reg) RegisterGetSchema() {
	register.AddHandler(func(mcpServer *server.MCPServer) {
		tool := new(GetSchema)
		mcpServer.AddTool(tool.ReturnTool(), tool.Handler(r))
	})
}
```

- [ ] **Step 3: 编译验证**

```bash
go build ./...
```

Expected: 退出码 0（get_table_list/get_schema 不再引用 SHOW 语句）。

- [ ] **Step 4: Commit**

```bash
git add internal/mcp/tools/get_table_list.go internal/mcp/tools/get_schema.go
git commit -m "refactor: get_table_list/get_schema 改用 gdb 元数据 API + 方言索引（统一跨库输出）"
```

---

### Task 11: 重构 get_table_info / get_enum_values / get_sample_data

**Files:**
- Modify: `internal/mcp/tools/get_table_info.go`（整体替换）
- Modify: `internal/mcp/tools/get_enum_values.go`（整体替换）
- Modify: `internal/mcp/tools/get_sample_data.go`（整体替换）

- [ ] **Step 1: 替换 get_table_info.go**

```go
/*
 * @desc:获取表详细信息工具
 */

package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/util/gconv"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/tiger1103/gf-mcp-db/internal/consts"
	"github.com/tiger1103/gf-mcp-db/internal/mcp/dialect"
	"github.com/tiger1103/gf-mcp-db/internal/mcp/register"
	"github.com/tiger1103/gf-mcp-db/library/liberr"
)

// GetTableInfo 获取表详细信息工具结构
type GetTableInfo struct{}

// ReturnTool 返回工具定义
func (t *GetTableInfo) ReturnTool() mcp.Tool {
	return mcp.NewTool("get_table_info",
		mcp.WithDescription(`# 📊 获取表详细信息

## 🎯 工具功能
获取指定表的详细信息，包括列定义、索引、外键、表注释、预估行数等。

## 💡 使用示例
获取 users 表信息:
{
  "table": "users"
}`),
		mcp.WithString("table",
			mcp.Required(),
			mcp.Description("表名")),
	)
}

// Handler 工具处理函数
func (t *GetTableInfo) Handler(r *Reg) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var result string
		err := g.Try(ctx, func(ctx context.Context) {
			table := requireArgString(request.GetArguments(), "table")
			db := getDB(ctx)
			d := currentDialect(ctx, db)

			fields, fieldsErr := db.TableFields(ctx, table)
			if fieldsErr != nil && isCaseInsensitiveCatalog(d.Name()) && table != strings.ToUpper(table) {
				// Oracle/DM 目录视图按大写匹配：小写表名自动用大写重试一次
				fields, fieldsErr = db.TableFields(ctx, strings.ToUpper(table))
			}
			liberr.ErrIsNil(ctx, fieldsErr)
			if len(fields) == 0 {
				panic(liberr.NewCode(consts.CodeInfo, "表不存在或没有列信息："+table))
			}
			lookupTable := table
			if isCaseInsensitiveCatalog(d.Name()) {
				lookupTable = strings.ToUpper(table)
			}

			tableInfo := map[string]any{
				"table_name": table,
				"columns":    dialect.ColumnsFromTableFields(fields),
			}
			if indexes, err := d.Indexes(ctx, db, lookupTable); err == nil {
				tableInfo["indexes"] = indexes
			}
			if fks, err := d.ForeignKeys(ctx, db, lookupTable); err == nil && len(fks) > 0 {
				tableInfo["foreign_keys"] = fks
			}
			if stat, err := d.TableStat(ctx, db, lookupTable); err == nil && stat != nil {
				tableInfo["table_stat"] = stat
			}

			result = fmt.Sprintf("表 %s 的详细信息：%s", table, gconv.String(tableInfo))
		})

		if err != nil {
			return r.returnRes(err)
		}

		return mcp.NewToolResultText(result), nil
	}
}

// isCaseInsensitiveCatalog 目录视图按大写存储标识符的库（Oracle/DM）
func isCaseInsensitiveCatalog(dbType string) bool {
	return dbType == "oracle" || dbType == "dm"
}

// RegisterGetTableInfo 注册获取表详细信息工具
func (r *Reg) RegisterGetTableInfo() {
	register.AddHandler(func(mcpServer *server.MCPServer) {
		tool := new(GetTableInfo)
		mcpServer.AddTool(tool.ReturnTool(), tool.Handler(r))
	})
}
```

- [ ] **Step 2: 替换 get_enum_values.go**

```go
/*
 * @desc:获取列的唯一值工具
 */

package tools

import (
	"context"
	"fmt"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/util/gconv"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/tiger1103/gf-mcp-db/internal/mcp/register"
	"github.com/tiger1103/gf-mcp-db/library/liberr"
)

// GetEnumValues 获取列的唯一值工具结构
type GetEnumValues struct{}

// ReturnTool 返回工具定义
func (t *GetEnumValues) ReturnTool() mcp.Tool {
	return mcp.NewTool("get_enum_values",
		mcp.WithDescription(`# 🔢 获取列的唯一值

## 🎯 工具功能
获取指定列的所有唯一值，用于了解 status、type 等枚举类型字段的可能取值。

## 💡 使用示例
获取 users 表的 status 列唯一值:
{
  "table": "users",
  "column": "status"
}

带条件获取唯一值:
{
  "table": "orders",
  "column": "status",
  "where": "created_at > '2024-01-01'"
}`),
		mcp.WithString("table",
			mcp.Required(),
			mcp.Description("表名")),
		mcp.WithString("column",
			mcp.Required(),
			mcp.Description("列名")),
		mcp.WithString("where",
			mcp.Description("可选的 WHERE 条件，用于过滤数据")),
		mcp.WithNumber("limit",
			mcp.Description("结果限制条数，默认 1000")),
	)
}

// Handler 工具处理函数
func (t *GetEnumValues) Handler(r *Reg) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var result string
		err := g.Try(ctx, func(ctx context.Context) {
			args := request.GetArguments()
			table := requireArgString(args, "table")
			column := requireArgString(args, "column")
			where := argString(args, "where")
			limit := argInt(args, "limit", 1000)

			db := getDB(ctx)
			d := currentDialect(ctx, db)

			querySQL := fmt.Sprintf("SELECT DISTINCT %s FROM %s", d.QuoteIdent(column), d.QuoteIdent(table))
			if where != "" {
				querySQL += " WHERE " + where
			}
			querySQL = d.Paginate(querySQL, limit)

			queryResult, queryErr := db.Query(ctx, querySQL)
			liberr.ErrIsNil(ctx, queryErr)

			uniqueValues := make([]string, 0, len(queryResult))
			for _, row := range queryResult {
				for _, value := range row {
					uniqueValues = append(uniqueValues, gconv.String(value))
				}
			}

			columnType := ""
			if fields, fieldsErr := db.TableFields(ctx, table); fieldsErr == nil {
				if f, ok := fields[column]; ok {
					columnType = f.Type
				}
			}

			result = fmt.Sprintf("列 %s.%s 的唯一值（类型：%s）：共 %d 个，值为：%s",
				table, column, columnType, len(uniqueValues), gconv.String(uniqueValues))
		})

		if err != nil {
			return r.returnRes(err)
		}

		return mcp.NewToolResultText(result), nil
	}
}

// RegisterGetEnumValues 注册获取列唯一值工具
func (r *Reg) RegisterGetEnumValues() {
	register.AddHandler(func(mcpServer *server.MCPServer) {
		tool := new(GetEnumValues)
		mcpServer.AddTool(tool.ReturnTool(), tool.Handler(r))
	})
}
```

- [ ] **Step 3: 替换 get_sample_data.go**

```go
/*
 * @desc:获取表示例数据工具
 */

package tools

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/util/gconv"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/tiger1103/gf-mcp-db/internal/consts"
	"github.com/tiger1103/gf-mcp-db/internal/mcp/register"
	"github.com/tiger1103/gf-mcp-db/library/liberr"
)

// GetSampleData 获取表示例数据工具结构
type GetSampleData struct{}

// ReturnTool 返回工具定义
func (t *GetSampleData) ReturnTool() mcp.Tool {
	return mcp.NewTool("get_sample_data",
		mcp.WithDescription(`# 📊 获取表示例数据

## 🎯 工具功能
获取表的示例数据，已自动脱敏敏感信息（如密码、手机号、邮箱等），用于了解数据格式和结构。

## 💡 使用示例
获取 users 表示例数据:
{
  "table": "users"
}

带条件获取示例数据:
{
  "table": "orders",
  "where": "status = 'completed'",
  "limit": 5
}`),
		mcp.WithString("table",
			mcp.Required(),
			mcp.Description("表名")),
		mcp.WithString("where",
			mcp.Description("可选的 WHERE 条件，用于过滤数据")),
		mcp.WithString("order",
			mcp.Description("可选的 ORDER BY 子句，例如 'id DESC'")),
		mcp.WithNumber("limit",
			mcp.Description("结果限制条数，默认 10")),
	)
}

// Handler 工具处理函数
func (t *GetSampleData) Handler(r *Reg) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var result string
		err := g.Try(ctx, func(ctx context.Context) {
			args := request.GetArguments()
			table := requireArgString(args, "table")
			where := argString(args, "where")
			order := argString(args, "order")
			limit := argInt(args, "limit", 10)

			db := getDB(ctx)
			d := currentDialect(ctx, db)

			fields, fieldsErr := db.TableFields(ctx, table)
			liberr.ErrIsNil(ctx, fieldsErr)
			if len(fields) == 0 {
				panic(liberr.NewCode(consts.CodeInfo, "表不存在或没有列信息："+table))
			}

			// 识别需要脱敏的列（按列名模式匹配）
			sensitiveColumns := make(map[string]bool)
			sensitivePatterns := []string{
				"password", "passwd", "pwd", "secret", "token", "key",
				"phone", "mobile", "tel", "email", "wechat", "qq",
				"id_card", "idcard", "identity", "card_no", "cardno",
				"address", "bank_card", "bankcard", "credit_card",
			}
			for _, f := range fields {
				colName := strings.ToLower(f.Name)
				for _, pattern := range sensitivePatterns {
					if strings.Contains(colName, pattern) {
						sensitiveColumns[colName] = true
						break
					}
				}
			}

			// 构建查询语句（where/order 为调用方 SQL 片段，按现状原样拼接）
			querySQL := fmt.Sprintf("SELECT * FROM %s", d.QuoteIdent(table))
			if where != "" {
				querySQL += " WHERE " + where
			}
			if order != "" {
				querySQL += " ORDER BY " + order
			}
			querySQL = d.Paginate(querySQL, limit)

			queryResult, queryErr := db.Query(ctx, querySQL)
			liberr.ErrIsNil(ctx, queryErr)

			// 对敏感数据进行脱敏
			for i := range queryResult {
				for colName := range queryResult[i] {
					colNameLower := strings.ToLower(colName)
					if sensitiveColumns[colNameLower] {
						originalValue := gconv.String(queryResult[i][colName])
						queryResult[i][colName] = g.NewVar(maskSensitiveData(originalValue))
					}
				}
			}

			result = fmt.Sprintf("表 %s 的示例数据（已脱敏）：共 %d 条记录，结果为：%s",
				table, len(queryResult), gconv.String(queryResult))

			// 添加脱敏说明
			if len(sensitiveColumns) > 0 {
				var maskedCols []string
				for col := range sensitiveColumns {
					maskedCols = append(maskedCols, col)
				}
				result += fmt.Sprintf("\n\n已脱敏的敏感字段：%s", strings.Join(maskedCols, ", "))
			}
		})

		if err != nil {
			return r.returnRes(err)
		}

		return mcp.NewToolResultText(result), nil
	}
}

// maskSensitiveData 脱敏敏感数据
func maskSensitiveData(data string) string {
	if data == "" {
		return "***"
	}

	length := len(data)

	// 手机号脱敏（11 位）
	if matched, _ := regexp.MatchString(`^1[3-9]\d{9}$`, data); matched {
		return data[:3] + "****" + data[7:]
	}

	// 邮箱脱敏
	if matched, _ := regexp.MatchString(`^[^\s@]+@[^\s@]+\.[^\s@]+$`, data); matched {
		atIndex := strings.Index(data, "@")
		if atIndex > 2 {
			return data[:2] + "***" + data[atIndex:]
		}
		return "***" + data[atIndex:]
	}

	// 身份证号脱敏（18 位）
	if length == 18 {
		if matched, _ := regexp.MatchString(`^\d{17}[\dXx]$`, data); matched {
			return data[:6] + "********" + data[14:]
		}
	}

	// 银行卡号脱敏
	if length >= 16 && length <= 19 {
		if matched, _ := regexp.MatchString(`^\d+$`, data); matched {
			return data[:6] + "******" + data[len(data)-4:]
		}
	}

	// 密码等敏感字段直接隐藏
	if length <= 20 {
		return strings.Repeat("*", min(length, 10))
	}

	// 其他长文本截断显示
	if length > 50 {
		return data[:20] + "..." + "***"
	}

	return "***"
}

// min 返回两个整数中的较小值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// RegisterGetSampleData 注册获取表示例数据工具
func (r *Reg) RegisterGetSampleData() {
	register.AddHandler(func(mcpServer *server.MCPServer) {
		tool := new(GetSampleData)
		mcpServer.AddTool(tool.ReturnTool(), tool.Handler(r))
	})
}
```

- [ ] **Step 4: 编译验证**

```bash
go build ./...
```

Expected: 退出码 0。

- [ ] **Step 5: Commit**

```bash
git add internal/mcp/tools/get_table_info.go internal/mcp/tools/get_enum_values.go internal/mcp/tools/get_sample_data.go
git commit -m "refactor: get_table_info/get_enum_values/get_sample_data 接入方言层（统一输出、跨库分页、Oracle/DM 大写重试）"
```

---

### Task 12: 重构 execute_query 与 clear_cache

**Files:**
- Modify: `internal/mcp/tools/execute_query.go`（整体替换）
- Modify: `internal/mcp/tools/clear_cache.go`（整体替换）

- [ ] **Step 1: 替换 execute_query.go**

```go
/*
 * @desc:执行 SQL 查询工具
 */

package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/util/gconv"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/tiger1103/gf-mcp-db/internal/mcp/register"
	"github.com/tiger1103/gf-mcp-db/library/liberr"
)

// ExecuteQuery 执行 SQL 查询工具结构
type ExecuteQuery struct{}

// ReturnTool 返回工具定义
func (t *ExecuteQuery) ReturnTool() mcp.Tool {
	return mcp.NewTool("execute_query",
		mcp.WithDescription(`# 🗃️ 执行 SQL 查询

## 🎯 工具功能
执行 SQL 查询或数据库命令，支持 SELECT、INSERT、UPDATE、DELETE 等操作。
注意：SQL 语法需与当前连接的数据库类型匹配。

## 📋 支持的操作
- SELECT 查询
- INSERT 插入数据
- UPDATE 更新数据
- DELETE 删除数据
- DDL 语句（CREATE、ALTER、DROP 等）

## 💡 使用示例
查询用户:
{
  "sql": "SELECT * FROM users LIMIT 10"
}

插入数据:
{
  "sql": "INSERT INTO users (name, email) VALUES ('John', 'john@example.com')
}`),
		mcp.WithString("sql",
			mcp.Required(),
			mcp.Description("SQL 查询语句")),
		mcp.WithNumber("limit",
			mcp.Description("结果限制条数，默认 100，仅对 SELECT 查询有效")),
	)
}

// Handler 工具处理函数
func (t *ExecuteQuery) Handler(r *Reg) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var result string
		err := g.Try(ctx, func(ctx context.Context) {
			args := request.GetArguments()
			sqlStr := requireArgString(args, "sql")
			limit := argInt(args, "limit", 100)

			db := getDB(ctx)

			if isQuerySQL(sqlStr) {
				queryResult, queryErr := db.Query(ctx, sqlStr)
				liberr.ErrIsNil(ctx, queryErr)

				if len(queryResult) > limit {
					queryResult = queryResult[:limit]
				}

				result = fmt.Sprintf("查询成功，返回 %d 条记录，结果为：%s", len(queryResult), gconv.String(queryResult))
			} else {
				execResult, execErr := db.Exec(ctx, sqlStr)
				liberr.ErrIsNil(ctx, execErr)

				rowsAffected, _ := execResult.RowsAffected()
				lastInsertId, _ := execResult.LastInsertId()
				result = fmt.Sprintf("SQL 执行成功，影响行数：%d，最后插入 ID: %d", rowsAffected, lastInsertId)
			}
		})

		if err != nil {
			return r.returnRes(err)
		}

		return mcp.NewToolResultText(result), nil
	}
}

// strings 引用保留（isQuerySQL 已迁移至 helpers.go，如无其他使用可删除此行）
var _ = strings.TrimSpace

// RegisterExecuteQuery 注册执行 SQL 查询工具
func (r *Reg) RegisterExecuteQuery() {
	register.AddHandler(func(mcpServer *server.MCPServer) {
		tool := new(ExecuteQuery)
		mcpServer.AddTool(tool.ReturnTool(), tool.Handler(r))
	})
}
```

注意：`import "strings"` 与 `var _ = strings.TrimSpace` 占位行若无实际使用请一并删除（保持 vet 干净）。

- [ ] **Step 2: 替换 clear_cache.go**

```go
/*
 * @desc:清除 Schema 缓存工具
 */

package tools

import (
	"context"
	"fmt"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/tiger1103/gf-mcp-db/internal/mcp/register"
	"github.com/tiger1103/gf-mcp-db/library/liberr"
)

// ClearCache 清除 Schema 缓存工具结构
type ClearCache struct{}

// ReturnTool 返回工具定义
func (t *ClearCache) ReturnTool() mcp.Tool {
	return mcp.NewTool("clear_cache",
		mcp.WithDescription(`# 🗑️ 清除 Schema 缓存

## 🎯 工具功能
清除服务端缓存的数据库元数据（表/列信息），当数据库结构发生变化后使用此工具刷新缓存。
适用于所有数据库类型。

## 💡 使用示例
清除所有缓存:
{}

清除指定表的缓存:
{
  "table": "users"
}`),
		mcp.WithString("table",
			mcp.Description("可选，指定要清除缓存的表名，不传则清除所有缓存")),
	)
}

// Handler 工具处理函数
func (t *ClearCache) Handler(r *Reg) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var result string
		err := g.Try(ctx, func(ctx context.Context) {
			table, _ := request.GetArguments()["table"].(string)

			db := getDB(ctx)

			// 清除 gdb 内部元数据缓存（Tables/TableFields），全库一致，不再执行 MySQL 专有的 FLUSH TABLES
			cache := db.GetCore().GetInnerMemCache()
			if cacheErr := cache.Clear(ctx); cacheErr != nil {
				liberr.ErrIsNil(ctx, cacheErr)
			}

			if table == "" {
				result = "Schema 缓存已清除（所有表）"
			} else {
				result = fmt.Sprintf("表 %s 的 Schema 缓存已清除", table)
			}
		})

		if err != nil {
			return r.returnRes(err)
		}

		return mcp.NewToolResultText(result), nil
	}
}

// RegisterClearCache 注册清除 Schema 缓存工具
func (r *Reg) RegisterClearCache() {
	register.AddHandler(func(mcpServer *server.MCPServer) {
		tool := new(ClearCache)
		mcpServer.AddTool(tool.ReturnTool(), tool.Handler(r))
	})
}
```

- [ ] **Step 3: 运行既有单元测试与编译**

```bash
go build ./...
go test ./... -count=1
```

Expected: 编译通过，所有测试 PASS（含 dialect/dbconn/tools 单测）。

- [ ] **Step 4: Commit**

```bash
git add internal/mcp/tools/execute_query.go internal/mcp/tools/clear_cache.go
git commit -m "refactor: execute_query 跨库判定与结果截断；clear_cache 改清 gdb 元数据缓存"
```

---

### Task 13: cmd.go / router.go 去重接线

**Files:**
- Modify: `internal/cmd/cmd.go`（整体替换）
- Modify: `internal/mcp/router/router.go`（整体替换）

- [ ] **Step 1: 替换 router.go**

```go
/*
 * @desc:MCP 路由注册（HTTP SSE 模式）
 */

package router

import (
	"context"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/os/glog"
	"github.com/mark3labs/mcp-go/server"

	"github.com/tiger1103/gf-mcp-db/internal/dbconn"
	"github.com/tiger1103/gf-mcp-db/internal/mcp/register"
	"github.com/tiger1103/gf-mcp-db/internal/mcp/tools"
)

// Register 注册 MCP 服务（HTTP SSE 模式）
func Register(ctx context.Context, s *ghttp.Server) {
	mcpServer := server.NewMCPServer("universal-db-mcp", "1.0.0")

	register.DoRegister(new(tools.Reg))
	register.DoHandler(mcpServer)

	sseServer := server.NewSSEServer(mcpServer,
		server.WithSSEEndpoint("/mcp"),
	)
	ssePath := sseServer.CompleteSsePath()
	messagePath := sseServer.CompleteMessagePath()

	s.BindHandler(ssePath, func(r *ghttp.Request) {
		initDBForRequest(ctx, r)
		sseServer.ServeHTTP(r.Response.Writer, r.Request)
	})
	s.BindHandler(messagePath, func(r *ghttp.Request) {
		initDBForRequest(ctx, r)
		sseServer.ServeHTTP(r.Response.Writer, r.Request)
	})
}

// initDBForRequest 从 query/header 解析数据库配置并初始化连接
func initDBForRequest(ctx context.Context, r *ghttp.Request) {
	config := parseDatabaseConfigFromQuery(r)
	if config.DBType == "" {
		config = parseDatabaseConfigFromHeader(r)
	}
	if config.DBType == "" {
		g.Log().Debug(ctx, "未提供数据库配置，跳过数据库连接初始化")
		return
	}
	if err := dbconn.Init(ctx, config); err != nil {
		glog.Error(ctx, "初始化数据库连接失败:", err)
		return
	}
	glog.Info(ctx, "数据库连接初始化成功，类型："+config.DBType+", 数据库："+config.Database)
}

// parseDatabaseConfigFromQuery 从 URL 查询参数解析数据库配置
func parseDatabaseConfigFromQuery(r *ghttp.Request) *dbconn.Config {
	return &dbconn.Config{
		DBType:   r.Get("type").String(),
		Host:     r.Get("host").String(),
		Port:     r.Get("port").String(),
		Username: r.Get("user").String(),
		Password: r.Get("password").String(),
		Database: r.Get("database").String(),
		Charset:  r.Get("charset").String(),
		Extra:    r.Get("extra").String(),
		Debug:    r.Get("debug", "false").Bool(),
	}
}

// parseDatabaseConfigFromHeader 从请求头解析数据库配置
func parseDatabaseConfigFromHeader(r *ghttp.Request) *dbconn.Config {
	return &dbconn.Config{
		DBType:   r.GetHeader("X-DB-Type"),
		Host:     r.GetHeader("X-DB-Host"),
		Port:     r.GetHeader("X-DB-Port"),
		Username: r.GetHeader("X-DB-User"),
		Password: r.GetHeader("X-DB-Password"),
		Database: r.GetHeader("X-DB-Database"),
		Charset:  r.GetHeader("X-DB-Charset"),
		Extra:    r.GetHeader("X-DB-Extra"),
		Debug:    r.GetHeader("X-DB-Debug") == "true",
	}
}
```

- [ ] **Step 2: 替换 cmd.go**

```go
/*
 * @desc:命令行入口（stdio / HTTP 模式选择）
 */

package cmd

import (
	"context"
	"os"
	"strings"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gcmd"
	"github.com/gogf/gf/v2/os/gctx"
	"github.com/gogf/gf/v2/os/glog"
	"github.com/mark3labs/mcp-go/server"

	"github.com/tiger1103/gf-mcp-db/internal/dbconn"
	"github.com/tiger1103/gf-mcp-db/internal/mcp/register"
	"github.com/tiger1103/gf-mcp-db/internal/mcp/router"
	"github.com/tiger1103/gf-mcp-db/internal/mcp/tools"
)

var (
	Main = gcmd.Command{
		Name:  "main",
		Usage: "main",
		Brief: "start universal database mcp server",
		Func: func(ctx context.Context, parser *gcmd.Parser) (err error) {
			// 检查是否应该使用 stdio 模式
			if isStdioMode() {
				return runStdioMode(ctx)
			}

			// HTTP SSE 模式
			return runHttpMode(ctx, parser)
		},
	}
)

// runStdioMode 运行 stdio 模式
func runStdioMode(ctx context.Context) error {
	g.Log().SetFlags(glog.F_ASYNC | glog.F_TIME_DATE | glog.F_TIME_TIME | glog.F_FILE_LONG)
	g.Log().Info(ctx, "Universal Database MCP server starting with stdio mode")

	// 从命令行参数解析数据库配置
	dbConfig := parseDbConfigFromArgs(os.Args[1:])

	// 如果提供了数据库配置，则初始化连接
	if dbConfig != nil && dbConfig.DBType != "" {
		g.Log().Info(ctx, "初始化数据库连接，类型："+dbConfig.DBType+", 数据库："+dbConfig.Database)
		if err := dbconn.Init(ctx, dbConfig); err != nil {
			g.Log().Error(ctx, "初始化数据库连接失败:", err)
			return err
		}
	} else {
		g.Log().Warning(ctx, "未提供数据库配置，将在工具调用时处理")
	}

	// 创建 MCPServer
	mcpServer := server.NewMCPServer("universal-db-mcp", "1.0.0")

	// 工具注册
	register.DoRegister(&tools.Reg{})
	register.DoHandler(mcpServer)

	// 创建 StdioServer 并启动
	stdioServer := server.NewStdioServer(mcpServer)
	return stdioServer.Listen(ctx, os.Stdin, os.Stdout)
}

// runHttpMode 运行 HTTP SSE 模式（路由与连接初始化统一由 router 包完成）
func runHttpMode(ctx context.Context, parser *gcmd.Parser) error {
	g.Log().SetFlags(glog.F_ASYNC | glog.F_TIME_DATE | glog.F_TIME_TIME | glog.F_FILE_LONG)
	g.Log().Info(ctx, "Universal Database MCP server for sse starting")
	s := g.Server()
	router.Register(ctx, s)
	s.Run()
	return nil
}

// isStdioMode 检查是否应该使用 stdio 模式
func isStdioMode() bool {
	// 检查是否有 --stdio 标志
	for _, arg := range os.Args {
		if arg == "--stdio" || arg == "-s" {
			return true
		}
	}
	// 检查是否有数据库连接参数（stdio 模式特征）
	for _, arg := range os.Args {
		if strings.HasPrefix(arg, "--type=") || strings.HasPrefix(arg, "--host=") ||
			strings.HasPrefix(arg, "--database=") || arg == "--type" || arg == "--host" || arg == "--database" {
			return true
		}
	}
	return false
}

// parseDbConfigFromArgs 从命令行参数解析数据库配置
func parseDbConfigFromArgs(args []string) *dbconn.Config {
	config := &dbconn.Config{}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--type":
			if i+1 < len(args) {
				config.DBType = args[i+1]
				i++
			}
		case "--host":
			if i+1 < len(args) {
				config.Host = args[i+1]
				i++
			}
		case "--port":
			if i+1 < len(args) {
				config.Port = args[i+1]
				i++
			}
		case "--user":
			if i+1 < len(args) {
				config.Username = args[i+1]
				i++
			}
		case "--password":
			if i+1 < len(args) {
				config.Password = args[i+1]
				i++
			}
		case "--database":
			if i+1 < len(args) {
				config.Database = args[i+1]
				i++
			}
		case "--charset":
			if i+1 < len(args) {
				config.Charset = args[i+1]
				i++
			}
		case "--extra":
			if i+1 < len(args) {
				config.Extra = args[i+1]
				i++
			}
		case "--debug":
			config.Debug = true
		}
	}
	return config
}

// 确保在包初始化时设置上下文
func init() {
	_ = gctx.GetInitCtx()
}
```

行为说明（写入提交信息即可）：原 `buildDSN/initDatabaseConnection/handleDatabaseConnection/parseDatabaseConfigFromQuery/parseDatabaseConfigFromHeader/importRouter*` 全部删除，统一走 `dbconn.Init` 与 `router.Register`；HTTP 模式不再包 `Group/MiddlewareHandlerResponse`（SSE 端点自管响应）；多会话不同连接配置仍共享 default 组（与原实现一致）。

- [ ] **Step 3: stdio 冒烟（SQLite）**

```bash
$env:CGO_ENABLED="0"; go build -o ./bin/gf-mcp-db.exe .
$job = Start-Process -FilePath ".\bin\gf-mcp-db.exe" -ArgumentList "--type","sqlite","--database","$env:TEMP\gf_mcp_smoke.db" -NoNewWindow -PassThru -RedirectStandardInput "$env:TEMP\in.txt" -RedirectStandardOutput "$env:TEMP\out.txt" -RedirectStandardError "$env:TEMP\err.txt"
```

向 `$env:TEMP\in.txt` 先写入 MCP initialize 握手 JSON（一行）再观察 out.txt。Expected: 进程不退出、stderr 出现 "Universal Database MCP server starting with stdio mode"；随后 Stop-Process 清理。若握手验证不便，至少验证二进制能启动且打印启动日志（完整协议冒烟由 Task 14 集成测试覆盖）。

- [ ] **Step 4: Commit**

```bash
git add internal/cmd/cmd.go internal/mcp/router/router.go
git commit -m "refactor: cmd/router 去重，连接初始化统一走 dbconn；新增 --extra 参数"
```

---

### Task 14: 集成测试（SQLite 常跑；MySQL/PG 环境变量门控）

**Files:**
- Create: `internal/mcp/tools/integration_test.go`

- [ ] **Step 1: 编写集成测试**

创建 `internal/mcp/tools/integration_test.go`：

```go
/*
 * @desc:7 个工具的跨库集成测试
 * SQLite 本地常跑；MySQL/PG 由环境变量门控：
 *   GF_MCP_TEST_MYSQL = host|port|user|password|database
 *   GF_MCP_TEST_PG    = host|port|user|password|database
 */

package tools_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/mark3labs/mcp-go/mcp"

	"github.com/tiger1103/gf-mcp-db/internal/dbconn"
	"github.com/tiger1103/gf-mcp-db/internal/mcp/tools"
)

func newToolRequest(args map[string]any) mcp.CallToolRequest {
	return mcp.CallToolRequest{Params: mcp.CallToolParams{Name: "test", Arguments: args}}
}

func callTool(t *testing.T, handler func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error), args map[string]any) string {
	t.Helper()
	res, err := handler(context.Background(), newToolRequest(args))
	if err != nil {
		t.Fatalf("工具调用返回错误: %v", err)
	}
	if res == nil {
		t.Fatal("工具调用返回空结果")
	}
	var text string
	for _, item := range res.Content {
		if c, ok := item.(mcp.TextContent); ok {
			text += c.Text
		}
	}
	return text
}

func mustDB(t *testing.T) gdb.DB {
	t.Helper()
	db := g.DB("default")
	if db == nil {
		t.Fatal("数据库未初始化")
	}
	return db
}

func execDDL(t *testing.T, db gdb.DB, sql string) {
	t.Helper()
	if _, err := db.Exec(context.Background(), sql); err != nil {
		t.Fatalf("执行 %q 失败: %v", sql, err)
	}
}

func envConfig(t *testing.T, envVar, dbType string) *dbconn.Config {
	t.Helper()
	v := os.Getenv(envVar)
	if v == "" {
		t.Skipf("跳过集成测试：未设置 %s（格式 host|port|user|password|database）", envVar)
	}
	parts := strings.SplitN(v, "|", 5)
	if len(parts) != 5 {
		t.Fatalf("%s 格式错误，应为 host|port|user|password|database", envVar)
	}
	return &dbconn.Config{
		DBType: dbType, Host: parts[0], Port: parts[1],
		Username: parts[2], Password: parts[3], Database: parts[4],
	}
}

// exerciseTools 假定 gf_mcp_t_users / gf_mcp_t_orders 已建好（含索引/外键/两行数据），逐个跑 7 个工具
func exerciseTools(t *testing.T) {
	t.Helper()
	reg := &tools.Reg{}

	// 1. get_table_list
	list := callTool(t, (&tools.ListTables{}).Handler(reg), map[string]any{})
	if !strings.Contains(list, "gf_mcp_t_users") || !strings.Contains(list, "gf_mcp_t_orders") {
		t.Fatalf("get_table_list 缺少期望表: %s", list)
	}
	// 模式过滤
	filtered := callTool(t, (&tools.ListTables{}).Handler(reg), map[string]any{"pattern": "gf_mcp_t_o%"})
	if strings.Contains(filtered, "gf_mcp_t_users") || !strings.Contains(filtered, "gf_mcp_t_orders") {
		t.Fatalf("get_table_list 模式过滤失效: %s", filtered)
	}

	// 2. get_table_info
	info := callTool(t, (&tools.GetTableInfo{}).Handler(reg), map[string]any{"table": "gf_mcp_t_users"})
	for _, want := range []string{"column_name", "email", "indexes"} {
		if !strings.Contains(info, want) {
			t.Fatalf("get_table_info 缺少 %q: %s", want, info)
		}
	}

	// 3. get_schema
	schema := callTool(t, (&tools.GetSchema{}).Handler(reg), map[string]any{"pattern": "gf_mcp_t_o%"})
	if !strings.Contains(schema, "gf_mcp_t_orders") || !strings.Contains(schema, "column_name") {
		t.Fatalf("get_schema 输出异常: %s", schema)
	}

	// 4. get_enum_values
	enum := callTool(t, (&tools.GetEnumValues{}).Handler(reg),
		map[string]any{"table": "gf_mcp_t_users", "column": "status"})
	if !strings.Contains(enum, "active") || !strings.Contains(enum, "disabled") {
		t.Fatalf("get_enum_values 缺少枚举值: %s", enum)
	}

	// 5. get_sample_data（email 应被脱敏）
	sample := callTool(t, (&tools.GetSampleData{}).Handler(reg),
		map[string]any{"table": "gf_mcp_t_users", "limit": 2})
	if !strings.Contains(sample, "已脱敏") || !strings.Contains(sample, "***@example.com") {
		t.Fatalf("get_sample_data 脱敏异常: %s", sample)
	}

	// 6. execute_query（查询 + 命令）
	q := callTool(t, (&tools.ExecuteQuery{}).Handler(reg),
		map[string]any{"sql": "SELECT COUNT(*) AS cnt FROM gf_mcp_t_users"})
	if !strings.Contains(q, "查询成功") {
		t.Fatalf("execute_query SELECT 异常: %s", q)
	}
	e := callTool(t, (&tools.ExecuteQuery{}).Handler(reg),
		map[string]any{"sql": "INSERT INTO gf_mcp_t_orders (user_id, amount) VALUES (1, 9.9)"})
	if !strings.Contains(e, "执行成功") {
		t.Fatalf("execute_query INSERT 异常: %s", e)
	}

	// 7. clear_cache
	cc := callTool(t, (&tools.ClearCache{}).Handler(reg), map[string]any{})
	if !strings.Contains(cc, "缓存已清除") {
		t.Fatalf("clear_cache 异常: %s", cc)
	}
}

func TestIntegrationSQLite(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "gf_mcp_test.db")
	if err := dbconn.Init(context.Background(), &dbconn.Config{DBType: "sqlite", Database: dbPath}); err != nil {
		t.Fatalf("初始化 SQLite 失败: %v", err)
	}
	db := mustDB(t)
	execDDL(t, db, `DROP TABLE IF EXISTS gf_mcp_t_orders`)
	execDDL(t, db, `DROP TABLE IF EXISTS gf_mcp_t_users`)
	execDDL(t, db, `CREATE TABLE gf_mcp_t_users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT,
		email TEXT,
		status TEXT DEFAULT 'active',
		created_at DATETIME
	)`)
	execDDL(t, db, `CREATE INDEX idx_gf_mcp_users_status ON gf_mcp_t_users(status)`)
	execDDL(t, db, `CREATE TABLE gf_mcp_t_orders (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER REFERENCES gf_mcp_t_users(id),
		amount REAL
	)`)
	execDDL(t, db, `INSERT INTO gf_mcp_t_users (name, email, status) VALUES ('alice','alice@example.com','active'),('bob','bob@example.com','disabled')`)
	exerciseTools(t)
}

func TestIntegrationMySQL(t *testing.T) {
	cfg := envConfig(t, "GF_MCP_TEST_MYSQL", "mysql")
	if err := dbconn.Init(context.Background(), cfg); err != nil {
		t.Fatalf("初始化 MySQL 失败: %v", err)
	}
	db := mustDB(t)
	execDDL(t, db, `DROP TABLE IF EXISTS gf_mcp_t_orders`)
	execDDL(t, db, `DROP TABLE IF EXISTS gf_mcp_t_users`)
	execDDL(t, db, `CREATE TABLE gf_mcp_t_users (
		id BIGINT AUTO_INCREMENT PRIMARY KEY,
		name VARCHAR(64),
		email VARCHAR(128),
		status VARCHAR(16) DEFAULT 'active',
		created_at DATETIME
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`)
	execDDL(t, db, `CREATE INDEX idx_gf_mcp_users_status ON gf_mcp_t_users(status)`)
	execDDL(t, db, `CREATE TABLE gf_mcp_t_orders (
		id BIGINT AUTO_INCREMENT PRIMARY KEY,
		user_id BIGINT,
		amount DECIMAL(10,2),
		CONSTRAINT fk_gf_mcp_orders_user FOREIGN KEY (user_id) REFERENCES gf_mcp_t_users(id)
	)`)
	execDDL(t, db, `INSERT INTO gf_mcp_t_users (name, email, status) VALUES ('alice','alice@example.com','active'),('bob','bob@example.com','disabled')`)
	exerciseTools(t)
}

func TestIntegrationPG(t *testing.T) {
	cfg := envConfig(t, "GF_MCP_TEST_PG", "postgres")
	if err := dbconn.Init(context.Background(), cfg); err != nil {
		t.Fatalf("初始化 PG 失败: %v", err)
	}
	db := mustDB(t)
	execDDL(t, db, `DROP TABLE IF EXISTS gf_mcp_t_orders`)
	execDDL(t, db, `DROP TABLE IF EXISTS gf_mcp_t_users`)
	execDDL(t, db, `CREATE TABLE gf_mcp_t_users (
		id SERIAL PRIMARY KEY,
		name VARCHAR(64),
		email VARCHAR(128),
		status VARCHAR(16) DEFAULT 'active',
		created_at TIMESTAMP
	)`)
	execDDL(t, db, `CREATE INDEX idx_gf_mcp_users_status ON gf_mcp_t_users(status)`)
	execDDL(t, db, `CREATE TABLE gf_mcp_t_orders (
		id SERIAL PRIMARY KEY,
		user_id INTEGER REFERENCES gf_mcp_t_users(id),
		amount NUMERIC(10,2)
	)`)
	execDDL(t, db, `INSERT INTO gf_mcp_t_users (name, email, status) VALUES ('alice','alice@example.com','active'),('bob','bob@example.com','disabled')`)
	exerciseTools(t)
}
```

- [ ] **Step 2: 跑 SQLite 集成（无需任何环境）**

```bash
go test ./internal/mcp/tools/ -run TestIntegrationSQLite -v
```

Expected: PASS。若失败，按输出修 dialect/工具实现直至通过（这是方言实现正确性的主验证点）。

- [ ] **Step 3: 跑 MySQL/PG 集成（凭据经环境变量，不入仓库）**

```bash
$env:GF_MCP_TEST_MYSQL="localhost|3306|root|123456|test"
$env:GF_MCP_TEST_PG="192.168.0.214|5432|postgres|123456|postgres"
go test ./internal/mcp/tools/ -run "TestIntegrationMySQL|TestIntegrationPG" -v
```

Expected: 两个 PASS。注意 PG 侧在 `postgres` 库建临时表，测试结束未 DROP 属预期（重复运行先 DROP）。

- [ ] **Step 4: Commit**

```bash
git add internal/mcp/tools/integration_test.go
git commit -m "test: 7 工具跨库集成测试（SQLite 常跑，MySQL/PG 环境变量门控）"
```

---

### Task 15: README 更新

**Files:**
- Modify: `README.MD`

- [ ] **Step 1: 更新以下章节（其余保持不动）**

1. 开头简介下新增「支持的数据库」表：

```markdown
## 支持的数据库

| 数据库 | --type 取值 | 默认端口 | 驱动（全部纯 Go，零 CGO） |
|--------|-------------|----------|---------------------------|
| MySQL | `mysql` | 3306 | go-sql-driver/mysql |
| PostgreSQL | `postgres` / `postgresql` / `pgsql` | 5432 | lib/pq |
| SQLite | `sqlite` | -（文件路径） | glebarez/go-sqlite（modernc，无需 CGO） |
| SQL Server | `sqlserver` / `mssql` | 1433 | microsoft/go-mssqldb |
| Oracle | `oracle` | 1521 | sijms/go-ora（无需 Instant Client） |
| 达梦 DM8 | `dm` | 5236 | chunanyong/dm |
```

2. 命令行参数表：`--type` 说明改为上表取值；`--port`/`--host`/`--user`/`--password` 标注「省略时使用默认端口；SQLite 全部不需要」；新增一行：

```markdown
| --extra | 否 | 透传驱动的额外参数，格式 k1=v1&k2=v2（如 MySQL 的 loc=Local、SQLite 的 busy_timeout(5000)） | - |
```

3. Stdio 连接示例：保留 MySQL/PostgreSQL/SQLite 示例，追加 SQL Server / Oracle / DM 示例：

```json
{
  "mcpServers": {
    "my-sqlserver": {
      "command": "./bin/gf-mcp-db",
      "args": ["--type", "sqlserver", "--host", "localhost", "--user", "sa", "--password", "your_password", "--database", "master"]
    },
    "my-oracle": {
      "command": "./bin/gf-mcp-db",
      "args": ["--type", "oracle", "--host", "localhost", "--port", "1521", "--user", "scott", "--password", "tiger", "--database", "ORCL"]
    },
    "my-dm": {
      "command": "./bin/gf-mcp-db",
      "args": ["--type", "dm", "--host", "localhost", "--port", "5236", "--user", "SYSDBA", "--password", "SYSDBA", "--database", "DMSERVER"]
    }
  }
}
```

4. HTTP 模式：headers 表追加 `X-DB-Extra`、URL 参数追加 `extra`（说明同 --extra）。

5. 新增「已知限制」小节：

```markdown
## 已知限制

- `clear_cache` 已改为清除服务端元数据缓存（全库一致），不再执行 MySQL 专有的 `FLUSH TABLES`。
- Oracle：`execute_query` 的 INSERT 不返回自增 ID（Oracle 无 LastInsertId）；分页语法为 `FETCH FIRST n ROWS ONLY`（Oracle 12c+）；元数据按大写对象名匹配。
- SQL Server：分页使用 `SELECT TOP n`；gf 驱动要求 datetime2/datetimeoffset 才支持自动时间字段。
- DM（达梦8）：兼容 Oracle 目录视图；分页支持 LIMIT 语法。
- SQL Server / Oracle / DM 路径未经真实实例回归验证（编译与单元测试覆盖），欢迎反馈问题。
- MySQL：如需保留旧版时区行为，可追加 `--extra "loc=Local"`。
```

- [ ] **Step 2: Commit**

```bash
git add README.MD
git commit -m "docs: README 补充 6 库支持、extra 参数、各库示例与已知限制"
```

---

### Task 16: 全量验证与收尾

**Files:** 无新文件

- [ ] **Step 1: 格式与静态检查**

```bash
gofmt -l .
go vet ./...
```

Expected: gofmt 无输出；vet 无报错（若 Task 5/12 的占位 import 未删会在此暴露，删除后重跑）。

- [ ] **Step 2: 全量测试（SQLite 全跑；MySQL/PG 环境变量门控）**

```bash
$env:GF_MCP_TEST_MYSQL="localhost|3306|root|123456|test"
$env:GF_MCP_TEST_PG="192.168.0.214|5432|postgres|123456|postgres"
go test ./... -count=1 -v
```

Expected: 全部 PASS。

- [ ] **Step 3: 零 CGO 多平台编译验证**

```bash
$env:CGO_ENABLED="0"
$env:GOOS="linux"; $env:GOARCH="amd64"; go build -o ./bin/gf-mcp-db_linux .
$env:GOOS="windows"; $env:GOARCH="amd64"; go build -o ./bin/gf-mcp-db.exe .
$env:GOOS="darwin"; $env:GOARCH="arm64"; go build -o ./bin/gf-mcp-db_darwin .
Remove-Item Env:GOOS, Env:GOARCH
```

Expected: 三个平台均编译成功（证明纯 Go、无 CGO 依赖）。

- [ ] **Step 4: 手动 stdio 冒烟（真实 MCP 握手，SQLite）**

用任意 MCP 客户端（或 Task 13 Step 3 的方式）以 `--type sqlite --database <临时文件>` 连接，调用 `get_table_list`。Expected: 返回空表列表文本（新建库无表）。

- [ ] **Step 5: 清理与最终提交**

```bash
git status --porcelain
```

确认无意外文件入库（bin/、临时库文件不入库；若 .gitignore 未覆盖 bin/ 则不提交它）。如有格式化等零散修正：

```bash
git add -A -- . ':!bin' ':!CLAUDE.md' ':!gf-mcp-db.exe~'
git commit -m "chore: 多数据库支持收尾（格式化与验证修正）"
```

---

## 自查记录（writing-plans Self-Review）

1. **Spec 覆盖**：驱动引入（Task 1）、dbconn（Task 2，对应 spec §3）、dialect 六实现（Task 3-8，spec §3/§4 方言要点）、7 工具统一输出（Task 10-12，spec §4 表格逐行对应）、extra 透传（Task 2/13/15）、clear_cache 语义修正（Task 12）、去重（Task 13）、测试三层（Task 2/3/4-8/9 单测 + Task 14 集成 + Task 16 编译级）、文档（Task 15）。无缺口。
2. **占位符扫描**：无 TBD/TODO；Task 5/12 的 `var _ =` 占位 import 行已附带"未使用则删除"的明确处理指令。
3. **类型一致性**：`IDialect` 方法签名在 Task 3 定义、Task 4-8 实现一致（Name/QuoteIdent/Paginate/Indexes/ForeignKeys/TableStat）；`dbconn.Config` 字段与 Task 13 router/cmd 使用一致；`dialect.ColumnsFromTableFields`/`MatchPattern`/`Get` 与 Task 10/11 调用一致；`isQuerySQL` 定义（Task 9）与使用（Task 12）一致；统一输出键全文一致。
