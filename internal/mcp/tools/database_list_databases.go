/*
 * @desc:列出数据库工具
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

// ListDatabases 列出数据库工具结构
type ListDatabases struct{}

// ReturnTool 返回工具定义
func (t *ListDatabases) ReturnTool() mcp.Tool {
	return mcp.NewTool("list_databases",
		mcp.WithDescription(`# 🗄️ 列出数据库

## 🎯 工具功能
列出数据库服务器中的所有数据库。

## 💡 使用示例
列出所有数据库:
{}`),
	)
}

// Handler 工具处理函数
func (t *ListDatabases) Handler(r *Reg) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var result string
		err := g.Try(ctx, func(ctx context.Context) {
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

			// 执行查询 - SHOW DATABASES
			var queryResult gdb.Result
			var queryErr error
			queryResult, queryErr = db.Query(ctx, "SHOW DATABASES")
			liberr.ErrIsNil(ctx, queryErr)

			// 提取数据库名列表
			var dbNames []string
			for _, row := range queryResult {
				for _, value := range row {
					dbNames = append(dbNames, gconv.String(value))
				}
			}

			result = fmt.Sprintf("数据库服务器中共有 %d 个数据库，数据库列表：%s", len(dbNames), gconv.String(dbNames))
		})

		if err != nil {
			return r.returnRes(err)
		}

		return mcp.NewToolResultText(result), nil
	}
}

// RegisterListDatabases 注册列出数据库工具
func (r *Reg) RegisterListDatabases() {
	register.AddHandler(func(mcpServer *server.MCPServer) {
		tool := new(ListDatabases)
		mcpServer.AddTool(tool.ReturnTool(), tool.Handler(r))
	})
}
