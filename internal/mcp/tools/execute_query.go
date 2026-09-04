/*
 * @desc:执行 SQL 查询工具
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
  "sql": "INSERT INTO users (name, email) VALUES ('John', 'john@example.com')"
}`),
		mcp.WithString("sql",
			mcp.Required(),
			mcp.Description("SQL 查询语句")),
		mcp.WithNumber("limit",
			mcp.Description("结果限制条数，默认 100，仅对查询类语句有效")),
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

				truncated := false
				if len(queryResult) > limit {
					queryResult = queryResult[:limit]
					truncated = true
				}

				result = fmt.Sprintf("查询成功，返回 %d 条记录，结果为：%s", len(queryResult), gconv.String(queryResult))
				if truncated {
					result += fmt.Sprintf("（结果超过 limit=%d，已截断）", limit)
				}
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

// RegisterExecuteQuery 注册执行 SQL 查询工具
func (r *Reg) RegisterExecuteQuery() {
	register.AddHandler(func(mcpServer *server.MCPServer) {
		tool := new(ExecuteQuery)
		mcpServer.AddTool(tool.ReturnTool(), tool.Handler(r))
	})
}
