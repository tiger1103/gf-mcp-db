/*
 * @desc:执行 SQL 语句工具
 * @company:云南奇讯科技有限公司
 * @Author: yixiaohu<yxh669@qq.com>
 * @Date:   2025/4/23 16:13
 */

package tools

import (
	"context"
	"errors"
	"gf-mcp/internal/consts"
	"gf-mcp/internal/mcp/register"
	"gf-mcp/library/liberr"
	"strconv"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// ExecuteSQL 执行 SQL 语句工具结构
type ExecuteSQL struct{}

// ReturnTool 返回工具定义
func (t *ExecuteSQL) ReturnTool() mcp.Tool {
	return mcp.NewTool("execute_sql",
		mcp.WithDescription(`# 🗃️ SQL 语句执行器

## 🎯 工具功能
专用于在数据库中执行**数据定义语言 (DDL)**和**数据操作语言 (DML)**语句。

## 📋 支持的操作类型
### ✅ 优先使用本工具的场景：
- **DDL（数据定义语言）**：
  - CREATE TABLE - 创建表
  - ALTER TABLE - 修改表结构
  - DROP TABLE - 删除表
  - TRUNCATE TABLE - 清空表

- **DML（数据操作语言）**：
  - INSERT - 插入数据
  - UPDATE - 更新数据
  - DELETE - 删除数据

## ⚠️ 重要使用限制
- **单次单条**：每次只能执行**一条**SQL 语句
- **禁止批量**：不支持多条语句同时执行
- **无结果集**：执行后返回操作状态，不返回数据结果

## 🔄 工具选择指南
| 操作类型 | 使用工具 | 返回结果 |
|---------|---------|---------|
| **DDL/DML 操作** | ✅ 本工具 | 执行状态、影响行数 |
| **数据查询** | 🔍 query_database 工具 | 数据集、查询结果 |

## 💡 使用示例
创建表:
{
  "sql": "CREATE TABLE users (id INT PRIMARY KEY, name VARCHAR(100))"
}

插入数据:
{
  "sql": "INSERT INTO users (id, name) VALUES (1, 'John')"
}`),
		mcp.WithString("sql",
			mcp.Required(),
			mcp.Description("SQL 语句内容")),
	)
}

// Handler 工具处理函数
func (t *ExecuteSQL) Handler(r *Reg) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var result string
		err := g.Try(ctx, func(ctx context.Context) {
			// 获取参数
			sql, ok := request.GetArguments()["sql"].(string)
			if !ok || sql == "" {
				liberr.ErrIsNilCode(ctx, errors.New("sql 参数必须是非空字符串"), consts.CodeInfo)
			}

			// 获取数据库连接 - 使用 "default" 组名
			var db gdb.DB
			g.TryCatch(ctx, func(ctx context.Context) {
				db = g.DB("default")
			}, func(ctx context.Context, exception error) {
				g.Log().Error(ctx, exception.Error())
				liberr.ErrIsNilCode(ctx, errors.New("请先连接数据库，在建立 MCP 连接时提供数据库配置参数"), consts.CodeInfo)
			})

			if db == nil {
				liberr.ErrIsNilCode(ctx, errors.New("请先连接数据库，在建立 MCP 连接时提供数据库配置参数"), consts.CodeInfo)
			}

			// 执行 SQL
			execResult, execErr := db.Exec(ctx, sql)
			liberr.ErrIsNil(ctx, execErr)

			// 获取影响行数
			rowsAffected, _ := execResult.RowsAffected()
			result = "SQL 执行成功，影响行数：" + strconv.FormatInt(rowsAffected, 10)
		})

		if err != nil {
			return r.returnRes(err)
		}

		return mcp.NewToolResultText(result), nil
	}
}

// RegisterExecuteSQL 注册执行 SQL 工具
func (r *Reg) RegisterExecuteSQL() {
	register.AddHandler(func(mcpServer *server.MCPServer) {
		tool := new(ExecuteSQL)
		mcpServer.AddTool(tool.ReturnTool(), tool.Handler(r))
	})
}
