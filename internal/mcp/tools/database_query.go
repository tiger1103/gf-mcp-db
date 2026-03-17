/*
 * @desc:数据库查询工具
 * @company:云南奇讯科技有限公司
 * @Author: yixiaohu<yxh669@qq.com>
 * @Date:   2025/4/23 16:13
 */

package tools

import (
	"context"
	"errors"
	"fmt"
	"gf-mcp/internal/consts"
	"gf-mcp/internal/mcp/register"
	"gf-mcp/library/liberr"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/util/gconv"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// DatabaseQuery 数据库查询工具结构
type DatabaseQuery struct{}

// ReturnTool 返回工具定义
func (t *DatabaseQuery) ReturnTool() mcp.Tool {
	return mcp.NewTool("query_database",
		mcp.WithDescription(`# 🔍 数据库查询工具

## 🎯 工具功能
用于执行数据库查询操作，返回查询结果。

## 📋 支持的操作
- SELECT 查询
- SHOW 语句
- EXPLAIN 分析
- 其他只读查询操作

## ⚠️ 使用限制
- 仅用于查询操作，不能用于修改数据
- 如需修改数据请使用 execute_sql 工具

## 💡 使用示例
查询所有用户:
{
  "sql": "SELECT * FROM users LIMIT 10"
}`),
		mcp.WithString("sql",
			mcp.Required(),
			mcp.Description("SQL 查询语句")),
		mcp.WithNumber("limit",
			mcp.Description("结果限制条数，默认 100")),
	)
}

// Handler 工具处理函数
func (t *DatabaseQuery) Handler(r *Reg) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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

			// 执行查询
			var queryResult gdb.Result
			var queryErr error
			queryResult, queryErr = db.Query(ctx, sql)
			liberr.ErrIsNil(ctx, queryErr)

			// 限制结果数量
			if len(queryResult) > limit {
				queryResult = queryResult[:limit]
			}

			result = fmt.Sprintf("查询成功，返回 %d 条记录，结果为：%s", len(queryResult), gconv.String(queryResult))
		})

		if err != nil {
			return r.returnRes(err)
		}

		return mcp.NewToolResultText(result), nil
	}
}

// RegisterDatabaseQuery 注册数据库查询工具
func (r *Reg) RegisterDatabaseQuery() {
	register.AddHandler(func(mcpServer *server.MCPServer) {
		tool := new(DatabaseQuery)
		mcpServer.AddTool(tool.ReturnTool(), tool.Handler(r))
	})
}
