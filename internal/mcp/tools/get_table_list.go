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
