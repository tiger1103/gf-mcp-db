/*
 * @desc:获取列的唯一值工具
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
			args := request.GetArguments()
			table := requireArgString(args, "table")
			column := requireArgString(args, "column")
			where := argString(args, "where")
			limit := argInt(args, "limit", 1000)

			db := getDB(ctx)
			d := currentDialect(ctx, db)

			querySQL := fmt.Sprintf("SELECT DISTINCT %s FROM %s", d.QuoteIdent(column), d.QuoteIdent(table))
			if where != "" {
				querySQL += " WHERE " + where
			}
			querySQL = d.Paginate(querySQL, limit)

			queryResult, queryErr := db.Query(ctx, querySQL)
			liberr.ErrIsNil(ctx, queryErr)

			uniqueValues := make([]string, 0, len(queryResult))
			for _, row := range queryResult {
				for _, value := range row {
					uniqueValues = append(uniqueValues, gconv.String(value))
				}
			}

			columnType := ""
			if fields, fieldsErr := db.TableFields(ctx, table); fieldsErr == nil {
				if f, ok := fields[column]; ok {
					columnType = f.Type
				}
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
