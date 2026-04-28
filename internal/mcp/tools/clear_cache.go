/*
 * @desc:清除 Schema 缓存工具
 * @company:云南奇讯科技有限公司
 * @Author: yixiaohu<yxh669@qq.com>
 * @Date:   2025/4/23 16:13
 */

package tools

import (
	"context"
	"errors"
	"fmt"
	"gf-mcp-db/internal/consts"
	"gf-mcp-db/internal/mcp/register"
	"gf-mcp-db/library/liberr"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// ClearCache 清除 Schema 缓存工具结构
type ClearCache struct{}

// ReturnTool 返回工具定义
func (t *ClearCache) ReturnTool() mcp.Tool {
	return mcp.NewTool("clear_cache",
		mcp.WithDescription(`# 🗑️ 清除 Schema 缓存

## 🎯 工具功能
清除数据库 Schema 缓存，当数据库结构发生变化时使用此工具刷新缓存。

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
			// 获取可选参数
			table, _ := request.GetArguments()["table"].(string)

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

			// 执行 FLUSH 操作
			var flushSql string
			if table == "" {
				// 清除所有表相关的缓存
				flushSql = "FLUSH TABLES"
			} else {
				// 清除指定表的缓存
				flushSql = fmt.Sprintf("FLUSH TABLES `%s`", table)
			}

			_, flushErr := db.Exec(ctx, flushSql)
			liberr.ErrIsNil(ctx, flushErr)

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
