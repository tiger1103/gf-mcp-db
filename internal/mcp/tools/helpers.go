/*
 * @desc:工具层公共助手：连接获取、方言获取、参数读取、SQL 类型判定
 */

package tools

import (
	"context"
	"errors"
	"strings"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/util/gconv"

	"github.com/tiger1103/gf-mcp-db/internal/consts"
	"github.com/tiger1103/gf-mcp-db/internal/mcp/dialect"
	"github.com/tiger1103/gf-mcp-db/library/liberr"
)

// errNoDatabase 未连接数据库的统一错误文案
var errNoDatabase = errors.New("请先连接数据库，在建立 MCP 连接时提供数据库配置参数")

// getDB 获取默认数据库连接（未配置时经 liberr 抛出可读错误）
func getDB(ctx context.Context) gdb.DB {
	var db gdb.DB
	g.TryCatch(ctx, func(ctx context.Context) {
		db = g.DB("default")
	}, func(ctx context.Context, exception error) {
		g.Log().Error(ctx, exception.Error())
		liberr.ErrIsNilCode(ctx, errNoDatabase, consts.CodeInfo)
	})
	if db == nil {
		liberr.ErrIsNilCode(ctx, errNoDatabase, consts.CodeInfo)
	}
	return db
}

// currentDialect 依据当前连接的数据库类型获取方言实现
func currentDialect(ctx context.Context, db gdb.DB) dialect.IDialect {
	d, err := dialect.Get(db.GetConfig().Type)
	liberr.ErrIsNilCode(ctx, err, consts.CodeInfo)
	return d
}

// argString 读取字符串参数（缺省为空串）
func argString(args map[string]any, key string) string {
	v, _ := args[key].(string)
	return v
}

// requireArgString 读取必填字符串参数（空值 panic，由 g.Try 统一恢复）
func requireArgString(args map[string]any, key string) string {
	v := argString(args, key)
	if v == "" {
		panic(liberr.NewCode(consts.CodeInfo, key+" 参数必须是非空字符串"))
	}
	return v
}

// argInt 读取正整数参数（MCP JSON 数值可能为 float64，统一经 gconv 转换；非法回落默认值）
func argInt(args map[string]any, key string, def int) int {
	v, ok := args[key]
	if !ok || v == nil {
		return def
	}
	if n := gconv.Int(v); n > 0 {
		return n
	}
	return def
}

// isQuerySQL 判断 SQL 是否为返回结果集的语句（前缀判定，供 execute_query 使用）
func isQuerySQL(sql string) bool {
	upper := strings.TrimSpace(strings.ToUpper(sql))
	for _, p := range []string{"SELECT", "WITH", "SHOW", "EXPLAIN", "DESCRIBE", "DESC", "PRAGMA"} {
		if strings.HasPrefix(upper, p) {
			return true
		}
	}
	return false
}
