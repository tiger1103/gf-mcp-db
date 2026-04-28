/*
 * @desc:获取表示例数据工具
 * @company:云南奇讯科技有限公司
 * @Author: yixiaohu<yxh669@qq.com>
 * @Date:   2025/4/23 16:13
 */

package tools

import (
	"context"
	"errors"
	"fmt"
	"github.com/tiger1103/gf-mcp-db/internal/consts"
	"github.com/tiger1103/gf-mcp-db/internal/mcp/register"
	"github.com/tiger1103/gf-mcp-db/library/liberr"
	"regexp"
	"strings"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/util/gconv"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// GetSampleData 获取表示例数据工具结构
type GetSampleData struct{}

// ReturnTool 返回工具定义
func (t *GetSampleData) ReturnTool() mcp.Tool {
	return mcp.NewTool("get_sample_data",
		mcp.WithDescription(`# 📊 获取表示例数据

## 🎯 工具功能
获取表的示例数据，已自动脱敏敏感信息（如密码、手机号、邮箱等），用于了解数据格式和结构。

## 💡 使用示例
获取 users 表示例数据:
{
  "table": "users"
}

带条件获取示例数据:
{
  "table": "orders",
  "where": "status = 'completed'",
  "limit": 5
}`),
		mcp.WithString("table",
			mcp.Required(),
			mcp.Description("表名")),
		mcp.WithString("where",
			mcp.Description("可选的 WHERE 条件，用于过滤数据")),
		mcp.WithString("order",
			mcp.Description("可选的 ORDER BY 子句，例如 'id DESC'")),
		mcp.WithNumber("limit",
			mcp.Description("结果限制条数，默认 10")),
	)
}

// Handler 工具处理函数
func (t *GetSampleData) Handler(r *Reg) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var result string
		err := g.Try(ctx, func(ctx context.Context) {
			// 获取参数
			table, ok := request.GetArguments()["table"].(string)
			if !ok || table == "" {
				liberr.ErrIsNilCode(ctx, errors.New("table 参数必须是非空字符串"), consts.CodeInfo)
			}

			// 获取可选参数
			where, _ := request.GetArguments()["where"].(string)
			order, _ := request.GetArguments()["order"].(string)
			limitVal, hasLimit := request.GetArguments()["limit"]
			limit := 10
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

			// 获取列信息用于脱敏
			columnsSql := fmt.Sprintf("SHOW COLUMNS FROM `%s`", table)
			columnsResult, columnsErr := db.Query(ctx, columnsSql)
			liberr.ErrIsNil(ctx, columnsErr)

			// 识别需要脱敏的列
			sensitiveColumns := make(map[string]bool)
			sensitivePatterns := []string{
				"password", "passwd", "pwd", "secret", "token", "key",
				"phone", "mobile", "tel", "email", "wechat", "qq",
				"id_card", "idcard", "identity", "card_no", "cardno",
				"address", "bank_card", "bankcard", "credit_card",
			}

			for _, row := range columnsResult {
				colName := strings.ToLower(gconv.String(row["Field"]))
				colType := strings.ToLower(gconv.String(row["Type"]))

				// 检查列名是否匹配敏感模式
				for _, pattern := range sensitivePatterns {
					if strings.Contains(colName, pattern) {
						sensitiveColumns[colName] = true
						break
					}
				}

				// 检查列类型是否为字符串类型且长度较大（可能是敏感数据）
				if strings.Contains(colType, "varchar") || strings.Contains(colType, "text") {
					for _, pattern := range sensitivePatterns {
						if strings.Contains(colName, pattern) {
							sensitiveColumns[colName] = true
							break
						}
					}
				}
			}

			// 构建查询语句
			sql := fmt.Sprintf("SELECT * FROM `%s`", table)
			if where != "" {
				sql += " WHERE " + where
			}
			if order != "" {
				sql += " ORDER BY " + order
			}
			sql += fmt.Sprintf(" LIMIT %d", limit)

			// 执行查询
			queryResult, queryErr := db.Query(ctx, sql)
			liberr.ErrIsNil(ctx, queryErr)

			// 对敏感数据进行脱敏
			for i := range queryResult {
				for colName := range queryResult[i] {
					colNameLower := strings.ToLower(colName)
					if sensitiveColumns[colNameLower] {
						originalValue := gconv.String(queryResult[i][colName])
						queryResult[i][colName] = g.NewVar(maskSensitiveData(originalValue))
					}
				}
			}

			result = fmt.Sprintf("表 %s 的示例数据（已脱敏）：共 %d 条记录，结果为：%s",
				table, len(queryResult), gconv.String(queryResult))

			// 添加脱敏说明
			if len(sensitiveColumns) > 0 {
				var maskedCols []string
				for col := range sensitiveColumns {
					maskedCols = append(maskedCols, col)
				}
				result += fmt.Sprintf("\n\n已脱敏的敏感字段：%s", strings.Join(maskedCols, ", "))
			}
		})

		if err != nil {
			return r.returnRes(err)
		}

		return mcp.NewToolResultText(result), nil
	}
}

// maskSensitiveData 脱敏敏感数据
func maskSensitiveData(data string) string {
	if data == "" {
		return "***"
	}

	length := len(data)

	// 手机号脱敏（11 位）
	if matched, _ := regexp.MatchString(`^1[3-9]\d{9}$`, data); matched {
		return data[:3] + "****" + data[7:]
	}

	// 邮箱脱敏
	if matched, _ := regexp.MatchString(`^[^\s@]+@[^\s@]+\.[^\s@]+$`, data); matched {
		atIndex := strings.Index(data, "@")
		if atIndex > 2 {
			return data[:2] + "***" + data[atIndex:]
		}
		return "***" + data[atIndex:]
	}

	// 身份证号脱敏（18 位）
	if length == 18 {
		if matched, _ := regexp.MatchString(`^\d{17}[\dXx]$`, data); matched {
			return data[:6] + "********" + data[14:]
		}
	}

	// 银行卡号脱敏
	if length >= 16 && length <= 19 {
		if matched, _ := regexp.MatchString(`^\d+$`, data); matched {
			return data[:6] + "******" + data[len(data)-4:]
		}
	}

	// 密码等敏感字段直接隐藏
	if length <= 20 {
		return strings.Repeat("*", min(length, 10))
	}

	// 其他长文本截断显示
	if length > 50 {
		return data[:20] + "..." + "***"
	}

	return "***"
}

// min 返回两个整数中的较小值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// RegisterGetSampleData 注册获取表示例数据工具
func (r *Reg) RegisterGetSampleData() {
	register.AddHandler(func(mcpServer *server.MCPServer) {
		tool := new(GetSampleData)
		mcpServer.AddTool(tool.ReturnTool(), tool.Handler(r))
	})
}
