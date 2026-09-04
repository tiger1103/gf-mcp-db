/*
 * @desc:获取表详细信息工具
 */

package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/util/gconv"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/tiger1103/gf-mcp-db/internal/consts"
	"github.com/tiger1103/gf-mcp-db/internal/mcp/dialect"
	"github.com/tiger1103/gf-mcp-db/internal/mcp/register"
	"github.com/tiger1103/gf-mcp-db/library/liberr"
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
			table := requireArgString(request.GetArguments(), "table")
			db := getDB(ctx)
			d := currentDialect(ctx, db)

			fields, fieldsErr := db.TableFields(ctx, table)
			if fieldsErr != nil && isCaseInsensitiveCatalog(d.Name()) && table != strings.ToUpper(table) {
				// Oracle/DM 驱动内部已做大写归一；此处防御性重试覆盖异常路径
				fields, fieldsErr = db.TableFields(ctx, strings.ToUpper(table))
			}
			liberr.ErrIsNil(ctx, fieldsErr)
			if len(fields) == 0 {
				panic(liberr.NewCode(consts.CodeInfo, "表不存在或没有列信息："+table))
			}
			lookupTable := table
			if isCaseInsensitiveCatalog(d.Name()) {
				lookupTable = strings.ToUpper(table)
			}

			pks, pkErr := d.PrimaryKeys(ctx, db, lookupTable)
			if pkErr != nil {
				// 主键信息不可用时显式标注，避免 primary_key:false 被误读为「无主键」
				g.Log().Warning(ctx, "获取表主键失败:", table, pkErr)
			}

			tableInfo := map[string]any{
				"table_name": table,
				"columns":    dialect.ColumnsFromTableFields(fields, pks),
			}
			if pkErr != nil {
				tableInfo["primary_keys_error"] = pkErr.Error()
			}
			if indexes, err := d.Indexes(ctx, db, lookupTable); err == nil {
				tableInfo["indexes"] = indexes
			} else {
				tableInfo["indexes_error"] = err.Error()
			}
			if fks, err := d.ForeignKeys(ctx, db, lookupTable); err == nil && len(fks) > 0 {
				tableInfo["foreign_keys"] = fks
			} else if err != nil {
				tableInfo["foreign_keys_error"] = err.Error()
			}
			if stat, err := d.TableStat(ctx, db, lookupTable); err == nil && stat != nil {
				tableInfo["table_stat"] = stat
			} else if err != nil {
				tableInfo["table_stat_error"] = err.Error()
			}

			result = fmt.Sprintf("表 %s 的详细信息：%s", table, gconv.String(tableInfo))
		})

		if err != nil {
			return r.returnRes(err)
		}

		return mcp.NewToolResultText(result), nil
	}
}

// isCaseInsensitiveCatalog 目录视图按大写存储标识符的库（Oracle/DM）
func isCaseInsensitiveCatalog(dbType string) bool {
	return dbType == "oracle" || dbType == "dm"
}

// RegisterGetTableInfo 注册获取表详细信息工具
func (r *Reg) RegisterGetTableInfo() {
	register.AddHandler(func(mcpServer *server.MCPServer) {
		tool := new(GetTableInfo)
		mcpServer.AddTool(tool.ReturnTool(), tool.Handler(r))
	})
}
