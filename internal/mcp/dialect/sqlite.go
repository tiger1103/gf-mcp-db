/*
 * @desc:SQLite 方言（PRAGMA 不支持参数绑定，标识符经 QuoteIdent 白名单校验后拼接）
 */

package dialect

import (
	"context"
	"fmt"

	"github.com/gogf/gf/v2/container/gvar"
	"github.com/gogf/gf/v2/database/gdb"
)

func init() {
	Register(NewSqliteDialect())
}

// NewSqliteDialect 构造 SQLite 方言
func NewSqliteDialect() IDialect {
	return &sqliteDialect{BaseDialect: NewBaseDialect("sqlite", "`", "`")}
}

type sqliteDialect struct {
	BaseDialect
}

// Indexes 索引列表（PRAGMA index_list + index_info）
func (d *sqliteDialect) Indexes(ctx context.Context, db gdb.DB, table string) (gdb.Result, error) {
	res, err := db.Query(ctx, fmt.Sprintf("PRAGMA index_list(%s)", d.QuoteIdent(table)))
	if err != nil {
		return nil, err
	}
	out := gdb.Result{}
	for _, r := range res {
		indexName := r["name"].String()
		isUnique := r["unique"].Bool()
		colRes, colErr := db.Query(ctx, fmt.Sprintf("PRAGMA index_info(%s)", d.QuoteIdent(indexName)))
		if colErr != nil {
			return nil, colErr
		}
		for _, cr := range colRes {
			out = append(out, gdb.Record{
				"index_name":  gvar.New(indexName),
				"column_name": cr["name"],
				"is_unique":   gvar.New(isUnique),
			})
		}
	}
	return out, nil
}

// ForeignKeys 外键列表（PRAGMA foreign_key_list）
func (d *sqliteDialect) ForeignKeys(ctx context.Context, db gdb.DB, table string) (gdb.Result, error) {
	res, err := db.Query(ctx, fmt.Sprintf("PRAGMA foreign_key_list(%s)", d.QuoteIdent(table)))
	if err != nil {
		return nil, err
	}
	out := gdb.Result{}
	for _, r := range res {
		out = append(out, gdb.Record{
			"constraint_name":        gvar.New(fmt.Sprintf("fk_%s_%s", table, r["id"].String())),
			"column_name":            r["from"],
			"referenced_table_name":  r["table"],
			"referenced_column_name": r["to"],
		})
	}
	return out, nil
}

// TableStat SQLite 无行数估算能力
func (d *sqliteDialect) TableStat(ctx context.Context, db gdb.DB, table string) (gdb.Record, error) {
	return nil, nil
}
