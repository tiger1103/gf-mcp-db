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
