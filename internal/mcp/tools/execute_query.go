/*
 * @desc:执行 SQL 查询工具
 * @company:云南奇讯科技有限公司
 * @Author: yixiaohu<yxh669@qq.com>
 * @Date:   2025/4/23 16:13
 */

package tools

import (
	"context"
	"errors"
	"fmt"
	"github.com/tiger1103/gf-mcp-db/internal/consts"
	"github.com/tiger1103/gf-mcp-db/internal/mcp/register"
	"github.com/tiger1103/gf-mcp-db/library/liberr"
	"strings"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/util/gconv"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// ExecuteQuery 执行 SQL 查询工具结构
type ExecuteQuery struct{}

// ReturnTool 返回工具定义
func (t *ExecuteQuery) ReturnTool() mcp.Tool {
	return mcp.NewTool("execute_query",
		mcp.WithDescription(`# 🗃️ 执行 SQL 查询

## 🎯 工具功能
执行 SQL 查询或数据库命令，支持 SELECT、INSERT、UPDATE、DELETE 等操作。

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
			mcp.Description("结果限制条数，默认 100，仅对 SELECT 查询有效")),
	)
}

// Handler 工具处理函数
func (t *ExecuteQuery) Handler(r *Reg) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var result string
		err := g.Try(ctx, func(ctx context.Context) {
			// 获取参数
			sql, ok := request.GetArguments()["sql"].(string)
			if !ok || sql == "" {
				liberr.ErrIsNilCode(ctx, errors.New("sql 参数必须是非空字符串"), consts.CodeInfo)
			}

			// 获取可选参数
			limitVal, hasLimit := request.GetArguments()["limit"]
			limit := 100
			if hasLimit && limitVal != nil {
				if l, ok := limitVal.(int); ok {
					limit = l
				}
			}

			// 获取数据库连接
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

			// 判断 SQL 类型并执行相应操作
			sqlUpper := strings.TrimSpace(strings.ToUpper(sql))
			if strings.HasPrefix(sqlUpper, "SELECT") || strings.HasPrefix(sqlUpper, "SHOW") || strings.HasPrefix(sqlUpper, "EXPLAIN") {
				// 查询操作
				queryResult, queryErr := db.Query(ctx, sql)
				liberr.ErrIsNil(ctx, queryErr)

				// 限制结果数量
				if len(queryResult) > limit {
					queryResult = queryResult[:limit]
				}

				result = fmt.Sprintf("查询成功，返回 %d 条记录，结果为：%s", len(queryResult), gconv.String(queryResult))
			} else {
				// 非查询操作（INSERT、UPDATE、DELETE、DDL 等）
				execResult, execErr := db.Exec(ctx, sql)
				liberr.ErrIsNil(ctx, execErr)

				// 获取影响行数
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
