/*
 * @desc:获取列的唯一值工具
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
	"github.com/gogf/gf/v2/util/gconv"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// GetEnumValues 获取列的唯一值工具结构
type GetEnumValues struct{}

// ReturnTool 返回工具定义
func (t *GetEnumValues) ReturnTool() mcp.Tool {
	return mcp.NewTool("get_enum_values",
		mcp.WithDescription(`# 🔢 获取列的唯一值

## 🎯 工具功能
获取指定列的所有唯一值，用于了解 status、type 等枚举类型字段的可能取值。

## 💡 使用示例
获取 users 表的 status 列唯一值:
{
  "table": "users",
  "column": "status"
}

带条件获取唯一值:
{
  "table": "orders",
  "column": "status",
  "where": "created_at > '2024-01-01'"
}`),
		mcp.WithString("table",
			mcp.Required(),
			mcp.Description("表名")),
		mcp.WithString("column",
			mcp.Required(),
			mcp.Description("列名")),
		mcp.WithString("where",
			mcp.Description("可选的 WHERE 条件，用于过滤数据")),
		mcp.WithNumber("limit",
			mcp.Description("结果限制条数，默认 1000")),
	)
}

// Handler 工具处理函数
func (t *GetEnumValues) Handler(r *Reg) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var result string
		err := g.Try(ctx, func(ctx context.Context) {
			// 获取参数
			table, ok := request.GetArguments()["table"].(string)
			if !ok || table == "" {
				liberr.ErrIsNilCode(ctx, errors.New("table 参数必须是非空字符串"), consts.CodeInfo)
			}

			column, ok := request.GetArguments()["column"].(string)
			if !ok || column == "" {
				liberr.ErrIsNilCode(ctx, errors.New("column 参数必须是非空字符串"), consts.CodeInfo)
			}

			// 获取可选参数
			where, _ := request.GetArguments()["where"].(string)
			limitVal, hasLimit := request.GetArguments()["limit"]
			limit := 1000
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

			// 构建查询语句
			sql := fmt.Sprintf("SELECT DISTINCT `%s` FROM `%s`", column, table)
			if where != "" {
				sql += " WHERE " + where
			}
			sql += fmt.Sprintf(" LIMIT %d", limit)

			// 执行查询
			queryResult, queryErr := db.Query(ctx, sql)
			liberr.ErrIsNil(ctx, queryErr)

			// 提取唯一值列表
			var uniqueValues []string
			for _, row := range queryResult {
				for _, value := range row {
					uniqueValues = append(uniqueValues, gconv.String(value))
				}
			}

			// 获取列类型信息
			typeSql := fmt.Sprintf("SHOW COLUMNS FROM `%s` WHERE Field = '%s'", table, column)
			typeResult, typeErr := db.Query(ctx, typeSql)
			columnType := ""
			if typeErr == nil && len(typeResult) > 0 {
				columnType = gconv.String(typeResult[0]["Type"])
			}

			result = fmt.Sprintf("列 %s.%s 的唯一值（类型：%s）：共 %d 个，值为：%s",
				table, column, columnType, len(uniqueValues), gconv.String(uniqueValues))
		})

		if err != nil {
			return r.returnRes(err)
		}

		return mcp.NewToolResultText(result), nil
	}
}

// RegisterGetEnumValues 注册获取列唯一值工具
func (r *Reg) RegisterGetEnumValues() {
	register.AddHandler(func(mcpServer *server.MCPServer) {
		tool := new(GetEnumValues)
		mcpServer.AddTool(tool.ReturnTool(), tool.Handler(r))
	})
}
