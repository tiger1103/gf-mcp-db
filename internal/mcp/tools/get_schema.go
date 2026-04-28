/*
 * @desc:获取数据库结构信息工具
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
			mcp.Description("表名匹配模式，支持通配符 %，例如 'user%' 匹配所有以 user 开头的表名")),
	)
}

// Handler 工具处理函数
func (t *GetSchema) Handler(r *Reg) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var result string
		err := g.Try(ctx, func(ctx context.Context) {
			// 获取可选参数
			pattern, _ := request.GetArguments()["pattern"].(string)

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

			// 获取所有表名
			var sql string
			if pattern == "" {
				sql = "SHOW TABLES"
			} else {
				sql = fmt.Sprintf("SHOW TABLES LIKE '%s'", pattern)
			}

			tablesResult, tablesErr := db.Query(ctx, sql)
			liberr.ErrIsNil(ctx, tablesErr)

			// 提取表名列表
			var tableNames []string
			for _, row := range tablesResult {
				for _, value := range row {
					tableNames = append(tableNames, gconv.String(value))
				}
			}

			// 获取每个表的结构信息
			var schemaInfo []map[string]interface{}
			for _, tableName := range tableNames {
				tableInfo := make(map[string]interface{})
				tableInfo["table_name"] = tableName

				// 获取列信息
				columnsSql := fmt.Sprintf("SHOW COLUMNS FROM `%s`", tableName)
				columnsResult, columnsErr := db.Query(ctx, columnsSql)
				if columnsErr == nil {
					var columns []map[string]string
					for _, row := range columnsResult {
						colInfo := make(map[string]string)
						for key, value := range row {
							colInfo[key] = gconv.String(value)
						}
						columns = append(columns, colInfo)
					}
					tableInfo["columns"] = columns
				}

				// 获取索引信息
				indexSql := fmt.Sprintf("SHOW INDEX FROM `%s`", tableName)
				indexResult, indexErr := db.Query(ctx, indexSql)
				if indexErr == nil {
					var indexes []map[string]string
					for _, row := range indexResult {
						idxInfo := make(map[string]string)
						for key, value := range row {
							idxInfo[key] = gconv.String(value)
						}
						indexes = append(indexes, idxInfo)
					}
					tableInfo["indexes"] = indexes
				}

				schemaInfo = append(schemaInfo, tableInfo)
			}

			result = fmt.Sprintf("数据库结构信息：共 %d 个表，详细信息：%s", len(tableNames), gconv.String(schemaInfo))
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
