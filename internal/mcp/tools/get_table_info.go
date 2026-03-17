/*
 * @desc:获取表详细信息工具
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

// GetTableInfo 获取表详细信息工具结构
type GetTableInfo struct{}

// ReturnTool 返回工具定义
func (t *GetTableInfo) ReturnTool() mcp.Tool {
	return mcp.NewTool("get_table_info",
		mcp.WithDescription(`# 📊 获取表详细信息

## 🎯 工具功能
获取指定表的详细信息，包括列定义、索引、外键、表注释、预估行数等。

## 💡 使用示例
获取 users 表信息:
{
  "table": "users"
}`),
		mcp.WithString("table",
			mcp.Required(),
			mcp.Description("表名")),
	)
}

// Handler 工具处理函数
func (t *GetTableInfo) Handler(r *Reg) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var result string
		err := g.Try(ctx, func(ctx context.Context) {
			// 获取参数
			table, ok := request.GetArguments()["table"].(string)
			if !ok || table == "" {
				liberr.ErrIsNilCode(ctx, errors.New("table 参数必须是非空字符串"), consts.CodeInfo)
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

			// 构建表信息
			tableInfo := make(map[string]interface{})
			tableInfo["table_name"] = table

			// 获取列信息
			columnsSql := fmt.Sprintf("SHOW COLUMNS FROM `%s`", table)
			columnsResult, columnsErr := db.Query(ctx, columnsSql)
			liberr.ErrIsNil(ctx, columnsErr)

			var columns []map[string]string
			for _, row := range columnsResult {
				colInfo := make(map[string]string)
				for key, value := range row {
					colInfo[key] = gconv.String(value)
				}
				columns = append(columns, colInfo)
			}
			tableInfo["columns"] = columns

			// 获取索引信息
			indexSql := fmt.Sprintf("SHOW INDEX FROM `%s`", table)
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

			// 获取表统计信息（行数、引擎等）
			statsSql := fmt.Sprintf("SHOW TABLE STATUS WHERE Name = '%s'", table)
			statsResult, statsErr := db.Query(ctx, statsSql)
			if statsErr == nil && len(statsResult) > 0 {
				tableInfo["engine"] = gconv.String(statsResult[0]["Engine"])
				tableInfo["rows_estimate"] = gconv.String(statsResult[0]["Rows"])
				tableInfo["table_comment"] = gconv.String(statsResult[0]["Comment"])
				tableInfo["data_length"] = gconv.String(statsResult[0]["Data_length"])
				tableInfo["index_length"] = gconv.String(statsResult[0]["Index_length"])
			}

			// 获取外键信息
			fkSql := fmt.Sprintf(`
				SELECT 
					CONSTRAINT_NAME,
					COLUMN_NAME,
					REFERENCED_TABLE_NAME,
					REFERENCED_COLUMN_NAME
				FROM information_schema.KEY_COLUMN_USAGE
				WHERE TABLE_SCHEMA = DATABASE()
				AND TABLE_NAME = '%s'
				AND REFERENCED_TABLE_NAME IS NOT NULL`, table)
			fkResult, fkErr := db.Query(ctx, fkSql)
			if fkErr == nil && len(fkResult) > 0 {
				var foreignKeys []map[string]string
				for _, row := range fkResult {
					fkInfo := make(map[string]string)
					for key, value := range row {
						fkInfo[key] = gconv.String(value)
					}
					foreignKeys = append(foreignKeys, fkInfo)
				}
				tableInfo["foreign_keys"] = foreignKeys
			}

			result = fmt.Sprintf("表 %s 的详细信息：%s", table, gconv.String(tableInfo))
		})

		if err != nil {
			return r.returnRes(err)
		}

		return mcp.NewToolResultText(result), nil
	}
}

// RegisterGetTableInfo 注册获取表详细信息工具
func (r *Reg) RegisterGetTableInfo() {
	register.AddHandler(func(mcpServer *server.MCPServer) {
		tool := new(GetTableInfo)
		mcpServer.AddTool(tool.ReturnTool(), tool.Handler(r))
	})
}
