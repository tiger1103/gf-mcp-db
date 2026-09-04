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

	_ "github.com/gogf/gf/contrib/drivers/mysql/v2"
	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	_ "github.com/gogf/gf/contrib/drivers/sqlite/v2"
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

func closeDefaultDB(t *testing.T) {
	t.Helper()
	if instance, err := gdb.Instance(gdb.DefaultGroupName); err == nil && instance != nil {
		_ = instance.Close(context.Background()) // 释放文件句柄，避免 Windows 下 TempDir 清理失败
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

	// 2. get_table_info（users：列+主键+索引；orders：外键）
	info := callTool(t, (&tools.GetTableInfo{}).Handler(reg), map[string]any{"table": "gf_mcp_t_users"})
	for _, want := range []string{"column_name", "email", "indexes"} {
		if !strings.Contains(info, want) {
			t.Fatalf("get_table_info 缺少 %q: %s", want, info)
		}
	}
	infoOrders := callTool(t, (&tools.GetTableInfo{}).Handler(reg), map[string]any{"table": "gf_mcp_t_orders"})
	if !strings.Contains(infoOrders, "foreign_keys") {
		t.Fatalf("get_table_info(orders) 缺少外键信息: %s", infoOrders)
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

	// 7. clear_cache（清缓存后再次查询应仍成功，验证缓存重建）
	cc := callTool(t, (&tools.ClearCache{}).Handler(reg), map[string]any{})
	if !strings.Contains(cc, "缓存已清除") {
		t.Fatalf("clear_cache 异常: %s", cc)
	}
	q2 := callTool(t, (&tools.ExecuteQuery{}).Handler(reg),
		map[string]any{"sql": "SELECT COUNT(*) AS cnt FROM gf_mcp_t_users"})
	if !strings.Contains(q2, "查询成功") {
		t.Fatalf("clear_cache 后查询失败: %s", q2)
	}
}

func TestIntegrationSQLite(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "gf_mcp_test.db")
	if err := dbconn.Init(context.Background(), &dbconn.Config{DBType: "sqlite", Database: dbPath}); err != nil {
		t.Fatalf("初始化 SQLite 失败: %v", err)
	}
	defer closeDefaultDB(t)
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
	defer closeDefaultDB(t)
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
	defer closeDefaultDB(t)
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
