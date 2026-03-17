/*
 * @desc:查看表结构工具
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

// DescribeTable 查看表结构工具结构
type DescribeTable struct{}

// ReturnTool 返回工具定义
func (t *DescribeTable) ReturnTool() mcp.Tool {
	return mcp.NewTool("describe_table",
		mcp.WithDescription(`# 📐 查看表结构

## 🎯 工具功能
查看指定表的详细结构信息，包括字段名、数据类型、约束等。

## 💡 使用示例
查看 users 表结构:
{
  "database": "mydb",
  "table": "users"
}`),
		mcp.WithString("database",
			mcp.Required(),
			mcp.Description("数据库名称")),
		mcp.WithString("table",
			mcp.Required(),
			mcp.Description("表名")),
	)
}

// Handler 工具处理函数
func (t *DescribeTable) Handler(r *Reg) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var result string
		err := g.Try(ctx, func(ctx context.Context) {
			// 获取参数
			database, ok := request.GetArguments()["database"].(string)
			if !ok || database == "" {
				liberr.ErrIsNilCode(ctx, errors.New("database 参数必须是非空字符串"), consts.CodeInfo)
			}

			table, ok := request.GetArguments()["table"].(string)
			if !ok || table == "" {
				liberr.ErrIsNilCode(ctx, errors.New("table 参数必须是非空字符串"), consts.CodeInfo)
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

			// 构建查询语句 - 使用 SHOW COLUMNS
			sql := fmt.Sprintf("SHOW COLUMNS FROM %s.%s", database, table)

			// 执行查询
			var queryResult gdb.Result
			var queryErr error
			queryResult, queryErr = db.Query(ctx, sql)
			liberr.ErrIsNil(ctx, queryErr)

			// 格式化结果
			var columns []map[string]string
			for _, row := range queryResult {
				colInfo := make(map[string]string)
				for key, value := range row {
					colInfo[key] = gconv.String(value)
				}
				columns = append(columns, colInfo)
			}

			result = fmt.Sprintf("表 %s.%s 的结构信息：%s", database, table, gconv.String(columns))
		})

		if err != nil {
			return r.returnRes(err)
		}

		return mcp.NewToolResultText(result), nil
	}
}

// RegisterDescribeTable 注册查看表结构工具
func (r *Reg) RegisterDescribeTable() {
	register.AddHandler(func(mcpServer *server.MCPServer) {
		tool := new(DescribeTable)
		mcpServer.AddTool(tool.ReturnTool(), tool.Handler(r))
	})
}
