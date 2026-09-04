/*
 * @desc:MySQL 方言
 */

package dialect

import (
	"context"
	"fmt"

	"github.com/gogf/gf/v2/container/gvar"
	"github.com/gogf/gf/v2/database/gdb"
)

func init() {
	Register(NewMysqlDialect())
}

// NewMysqlDialect 构造 MySQL 方言
func NewMysqlDialect() IDialect {
	return &mysqlDialect{BaseDialect: NewBaseDialect("mysql", "`", "`")}
}

type mysqlDialect struct {
	BaseDialect
}

// Indexes 索引列表（SHOW INDEX）
func (d *mysqlDialect) Indexes(ctx context.Context, db gdb.DB, table string) (gdb.Result, error) {
	res, err := db.Query(ctx, fmt.Sprintf("SHOW INDEX FROM %s", d.QuoteIdent(table)))
	if err != nil {
		return nil, err
	}
	out := make(gdb.Result, 0, len(res))
	for _, r := range res {
		out = append(out, gdb.Record{
			"index_name":  r["Key_name"],
			"column_name": r["Column_name"],
			"is_unique":   gvar.New(!r["Non_unique"].Bool()),
		})
	}
	return out, nil
}

// ForeignKeys 外键列表（information_schema，输出键统一小写）
func (d *mysqlDialect) ForeignKeys(ctx context.Context, db gdb.DB, table string) (gdb.Result, error) {
	res, err := db.Query(ctx, `
		SELECT CONSTRAINT_NAME AS constraint_name, COLUMN_NAME AS column_name,
		       REFERENCED_TABLE_NAME AS referenced_table_name, REFERENCED_COLUMN_NAME AS referenced_column_name
		FROM information_schema.KEY_COLUMN_USAGE
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? AND REFERENCED_TABLE_NAME IS NOT NULL
		ORDER BY CONSTRAINT_NAME, ORDINAL_POSITION`, table)
	if err != nil {
		return nil, err
	}
	return res, nil
}

// TableStat 表状态（information_schema.TABLES；MySQL 5.7 的 SHOW TABLE STATUS 不支持预编译，故不使用）
func (d *mysqlDialect) TableStat(ctx context.Context, db gdb.DB, table string) (gdb.Record, error) {
	res, err := db.Query(ctx, `
		SELECT ENGINE, TABLE_ROWS, TABLE_COMMENT, DATA_LENGTH, INDEX_LENGTH
		FROM information_schema.TABLES
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ?`, table)
	if err != nil {
		return nil, err
	}
	if len(res) == 0 {
		return nil, nil
	}
	r := res[0]
	return gdb.Record{
		"engine":        r["ENGINE"],
		"rows_estimate": r["TABLE_ROWS"],
		"table_comment": r["TABLE_COMMENT"],
		"data_length":   r["DATA_LENGTH"],
		"index_length":  r["INDEX_LENGTH"],
	}, nil
}
