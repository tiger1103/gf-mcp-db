/*
 * @desc:获取数据库结构信息工具
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

// GetSchema 获取数据库结构信息工具结构
type GetSchema struct{}

// ReturnTool 返回工具定义
func (t *GetSchema) ReturnTool() mcp.Tool {
	return mcp.NewTool("get_schema",
		mcp.WithDescription(`# 📐 获取数据库结构信息

## 🎯 工具功能
获取数据库的完整结构信息，包括所有表名、列名、数据类型、主键、索引等元数据。

## 💡 使用示例
获取所有表结构:
{}

获取匹配 user 的表结构:
{
  "pattern": "user%"
}`),
		mcp.WithString("pattern",
			mcp.Description("表名匹配模式，支持通配符 %（任意串）与 _（单字符），不区分大小写，例如 'user%' 匹配所有以 user 开头的表名")),
	)
}

// Handler 工具处理函数
func (t *GetSchema) Handler(r *Reg) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var result string
		err := g.Try(ctx, func(ctx context.Context) {
			db := getDB(ctx)
			d := currentDialect(ctx, db)

			tables, tablesErr := db.Tables(ctx)
			liberr.ErrIsNil(ctx, tablesErr)

			pattern := argString(request.GetArguments(), "pattern")
			schemaInfo := make([]map[string]any, 0, len(tables))
			for _, tableName := range tables {
				if !dialect.MatchPattern(tableName, pattern) {
					continue
				}
				entry := map[string]any{"table_name": tableName}

				fields, fieldsErr := db.TableFields(ctx, tableName)
				if fieldsErr == nil {
					pks, pkErr := d.PrimaryKeys(ctx, db, tableName)
					if pkErr != nil {
						// 主键信息不可用时显式标注，避免 primary_key:false 被误读为「无主键」
						g.Log().Warning(ctx, "获取表主键失败:", tableName, pkErr)
						entry["primary_keys_error"] = pkErr.Error()
					}
					entry["columns"] = dialect.ColumnsFromTableFields(fields, pks)
				} else {
					entry["columns_error"] = fieldsErr.Error()
				}

				if indexes, indexesErr := d.Indexes(ctx, db, tableName); indexesErr == nil {
					entry["indexes"] = indexes
				} else {
					entry["indexes_error"] = indexesErr.Error()
				}

				schemaInfo = append(schemaInfo, entry)
			}

			result = fmt.Sprintf("数据库结构信息：共 %d 个表，详细信息：%s", len(schemaInfo), gconv.String(schemaInfo))
		})

		if err != nil {
			return r.returnRes(err)
		}

		return mcp.NewToolResultText(result), nil
	}
}

// RegisterGetSchema 注册获取数据库结构信息工具
func (r *Reg) RegisterGetSchema() {
	register.AddHandler(func(mcpServer *server.MCPServer) {
		tool := new(GetSchema)
		mcpServer.AddTool(tool.ReturnTool(), tool.Handler(r))
	})
}
