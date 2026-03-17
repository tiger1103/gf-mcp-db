/*
 * @desc:列出数据库表工具
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

// ListTables 列出数据库表工具结构
type ListTables struct{}

// ReturnTool 返回工具定义
func (t *ListTables) ReturnTool() mcp.Tool {
	return mcp.NewTool("list_tables",
		mcp.WithDescription(`# 📋 列出数据库表

## 🎯 工具功能
列出指定数据库中的所有表或匹配特定模式的表。

## 💡 使用示例
列出所有表:
{
  "database": "mydb"
}

列出匹配 user 的表:
{
  "database": "mydb",
  "pattern": "user%"
}`),
		mcp.WithString("database",
			mcp.Required(),
			mcp.Description("数据库名称")),
		mcp.WithString("pattern",
			mcp.Description("表名匹配模式，支持通配符 %，例如 'user%' 匹配所有以 user 开头的表名")),
	)
}

// Handler 工具处理函数
func (t *ListTables) Handler(r *Reg) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var result string
		err := g.Try(ctx, func(ctx context.Context) {
			// 获取参数
			database, ok := request.GetArguments()["database"].(string)
			if !ok || database == "" {
				liberr.ErrIsNilCode(ctx, errors.New("database 参数必须是非空字符串"), consts.CodeInfo)
			}

			// 获取可选参数
			pattern, _ := request.GetArguments()["pattern"].(string)

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

			// 构建查询语句
			var sql string
			if pattern == "" {
				sql = "SHOW TABLES FROM " + database
			} else {
				sql = fmt.Sprintf("SHOW TABLES FROM %s LIKE '%s'", database, pattern)
			}

			// 执行查询
			var queryResult gdb.Result
			var queryErr error
			queryResult, queryErr = db.Query(ctx, sql)
			liberr.ErrIsNil(ctx, queryErr)

			// 提取表名列表
			var tableNames []string
			for _, row := range queryResult {
				for _, value := range row {
					tableNames = append(tableNames, gconv.String(value))
				}
			}

			result = fmt.Sprintf("数据库 %s 中共有 %d 个表，表名列表：%s", database, len(tableNames), gconv.String(tableNames))
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
