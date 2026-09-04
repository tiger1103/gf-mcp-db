/*
 * @desc:PostgreSQL 方言
 */

package dialect

import (
	"context"

	"github.com/gogf/gf/v2/container/gvar"
	"github.com/gogf/gf/v2/database/gdb"
)

func init() {
	Register(NewPgsqlDialect())
}

// NewPgsqlDialect 构造 PostgreSQL 方言
func NewPgsqlDialect() IDialect {
	return &pgsqlDialect{BaseDialect: NewBaseDialect("pgsql", `"`, `"`)}
}

type pgsqlDialect struct {
	BaseDialect
}

// Indexes 索引列表（pg_index / pg_attribute）
func (d *pgsqlDialect) Indexes(ctx context.Context, db gdb.DB, table string) (gdb.Result, error) {
	res, err := db.Query(ctx, `
		SELECT i.relname AS index_name, a.attname AS column_name, ix.indisunique AS is_unique
		FROM pg_class t
		JOIN pg_index ix ON ix.indrelid = t.oid
		JOIN pg_class i ON i.oid = ix.indexrelid
		JOIN pg_attribute a ON a.attrelid = t.oid AND a.attnum = ANY(ix.indkey)
		JOIN pg_namespace n ON n.oid = t.relnamespace
		WHERE t.relname = ? AND n.nspname = current_schema() AND i.relname NOT LIKE 'pg\_%'
		ORDER BY i.relname, a.attnum`, table)
	if err != nil {
		return nil, err
	}
	out := make(gdb.Result, 0, len(res))
	for _, r := range res {
		out = append(out, gdb.Record{
			"index_name":  r["index_name"],
			"column_name": r["column_name"],
			"is_unique":   gvar.New(r["is_unique"].Bool()),
		})
	}
	return out, nil
}

// ForeignKeys 外键列表（information_schema）
func (d *pgsqlDialect) ForeignKeys(ctx context.Context, db gdb.DB, table string) (gdb.Result, error) {
	res, err := db.Query(ctx, `
		SELECT tc.constraint_name AS constraint_name, kcu.column_name AS column_name,
		       ccu.table_name AS referenced_table_name, ccu.column_name AS referenced_column_name
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu
		  ON tc.constraint_name = kcu.constraint_name AND tc.constraint_schema = kcu.constraint_schema
		JOIN information_schema.constraint_column_usage ccu
		  ON ccu.constraint_name = tc.constraint_name AND ccu.constraint_schema = tc.constraint_schema
		WHERE tc.constraint_type = 'FOREIGN KEY' AND tc.table_name = ?
		ORDER BY tc.constraint_name, kcu.ordinal_position`, table)
	if err != nil {
		return nil, err
	}
	return res, nil
}

// TableStat 行数估算与表注释（pg_class）；reltuples < 0 表示未 ANALYZE，省略该键
func (d *pgsqlDialect) TableStat(ctx context.Context, db gdb.DB, table string) (gdb.Record, error) {
	res, err := db.Query(ctx, `
		SELECT c.reltuples::bigint AS rows_estimate, obj_description(c.oid, 'pg_class') AS table_comment
		FROM pg_class c
		WHERE c.relname = ? AND c.relkind IN ('r','p')`, table)
	if err != nil {
		return nil, err
	}
	if len(res) == 0 {
		return nil, nil
	}
	r := res[0]
	stat := gdb.Record{"table_comment": r["table_comment"]}
	if rows := r["rows_estimate"].Int(); rows >= 0 {
		stat["rows_estimate"] = gvar.New(rows)
	}
	return stat, nil
}
