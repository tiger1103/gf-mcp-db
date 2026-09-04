/*
 * @desc:Oracle 方言（与 DM 共享 Oracle 兼容目录视图实现）
 */

package dialect

import (
	"context"
	"fmt"
	"strings"

	"github.com/gogf/gf/v2/container/gvar"
	"github.com/gogf/gf/v2/database/gdb"
)

func init() {
	Register(NewOracleDialect())
}

// NewOracleDialect 构造 Oracle 方言
func NewOracleDialect() IDialect {
	return &oracleDialect{oracleLikeDialect{BaseDialect: NewBaseDialect("oracle", `"`, `"`)}}
}

type oracleDialect struct {
	oracleLikeDialect
}

// Paginate FETCH FIRST（Oracle 12c+；先去尾分号/空白）
func (d *oracleDialect) Paginate(selectSQL string, limit int) string {
	if limit <= 0 {
		limit = 100
	}
	sqlStr := strings.TrimRight(strings.TrimSpace(selectSQL), "; \t\n\r")
	return fmt.Sprintf("%s FETCH FIRST %d ROWS ONLY", sqlStr, limit)
}

// oracleLikeDialect Oracle 兼容目录视图共享实现（Oracle / DM）。
// 注意：两驱动均不回填 TableField.Key，主键需查约束表（见 PrimaryKeys 覆盖）。
type oracleLikeDialect struct {
	BaseDialect
}

// PrimaryKeys 主键列集合（ALL_CONSTRAINTS 约束类型 P）
func (d *oracleLikeDialect) PrimaryKeys(ctx context.Context, db gdb.DB, table string) (map[string]bool, error) {
	res, err := db.Query(ctx, `
		SELECT acc.COLUMN_NAME
		FROM ALL_CONSTRAINTS ac
		JOIN ALL_CONS_COLUMNS acc
		  ON acc.OWNER = ac.OWNER AND acc.CONSTRAINT_NAME = ac.CONSTRAINT_NAME
		WHERE ac.CONSTRAINT_TYPE = 'P' AND ac.TABLE_NAME = ? AND ac.OWNER = USER`, table)
	if err != nil {
		return nil, err
	}
	pks := make(map[string]bool)
	for _, r := range res {
		pks[r["COLUMN_NAME"].String()] = true
	}
	return pks, nil
}

// Indexes 索引列表（ALL_INDEXES + ALL_IND_COLUMNS，限当前用户）
func (d *oracleLikeDialect) Indexes(ctx context.Context, db gdb.DB, table string) (gdb.Result, error) {
	res, err := db.Query(ctx, `
		SELECT ic.INDEX_NAME, ic.COLUMN_NAME, ix.UNIQUENESS
		FROM ALL_INDEXES ix
		JOIN ALL_IND_COLUMNS ic
		  ON ic.INDEX_OWNER = ix.OWNER AND ic.INDEX_NAME = ix.INDEX_NAME
		WHERE ix.TABLE_NAME = ? AND ix.OWNER = USER
		ORDER BY ic.INDEX_NAME, ic.COLUMN_POSITION`, table)
	if err != nil {
		return nil, err
	}
	out := make(gdb.Result, 0, len(res))
	for _, r := range res {
		out = append(out, gdb.Record{
			"index_name":  r["INDEX_NAME"],
			"column_name": r["COLUMN_NAME"],
			"is_unique":   gvar.New(strings.EqualFold(r["UNIQUENESS"].String(), "UNIQUE")),
		})
	}
	return out, nil
}

// ForeignKeys 外键列表（ALL_CONSTRAINTS 约束类型 R，按位置配对引用列）
func (d *oracleLikeDialect) ForeignKeys(ctx context.Context, db gdb.DB, table string) (gdb.Result, error) {
	res, err := db.Query(ctx, `
		SELECT ac.CONSTRAINT_NAME, acc.COLUMN_NAME,
		       rc.TABLE_NAME AS REFERENCED_TABLE_NAME, rcc.COLUMN_NAME AS REFERENCED_COLUMN_NAME
		FROM ALL_CONSTRAINTS ac
		JOIN ALL_CONS_COLUMNS acc
		  ON acc.OWNER = ac.OWNER AND acc.CONSTRAINT_NAME = ac.CONSTRAINT_NAME
		JOIN ALL_CONSTRAINTS rc
		  ON rc.OWNER = ac.R_OWNER AND rc.CONSTRAINT_NAME = ac.R_CONSTRAINT_NAME
		JOIN ALL_CONS_COLUMNS rcc
		  ON rcc.OWNER = rc.OWNER AND rcc.CONSTRAINT_NAME = rc.CONSTRAINT_NAME AND rcc.POSITION = acc.POSITION
		WHERE ac.CONSTRAINT_TYPE = 'R' AND ac.TABLE_NAME = ? AND ac.OWNER = USER
		ORDER BY ac.CONSTRAINT_NAME, acc.POSITION`, table)
	if err != nil {
		return nil, err
	}
	out := make(gdb.Result, 0, len(res))
	for _, r := range res {
		out = append(out, gdb.Record{
			"constraint_name":        r["CONSTRAINT_NAME"],
			"column_name":            r["COLUMN_NAME"],
			"referenced_table_name":  r["REFERENCED_TABLE_NAME"],
			"referenced_column_name": r["REFERENCED_COLUMN_NAME"],
		})
	}
	return out, nil
}

// TableStat 行数估算与表注释（ALL_TABLES + ALL_TAB_COMMENTS）；NUM_ROWS < 0 省略行数
func (d *oracleLikeDialect) TableStat(ctx context.Context, db gdb.DB, table string) (gdb.Record, error) {
	res, err := db.Query(ctx, `
		SELECT t.NUM_ROWS AS ROWS_ESTIMATE, tc.COMMENTS AS TABLE_COMMENT
		FROM ALL_TABLES t
		LEFT JOIN ALL_TAB_COMMENTS tc
		  ON tc.OWNER = t.OWNER AND tc.TABLE_NAME = t.TABLE_NAME
		WHERE t.TABLE_NAME = ? AND t.OWNER = USER`, table)
	if err != nil {
		return nil, err
	}
	if len(res) == 0 {
		return nil, nil
	}
	r := res[0]
	stat := gdb.Record{"table_comment": r["TABLE_COMMENT"]}
	if rows := r["ROWS_ESTIMATE"].Int(); rows >= 0 {
		stat["rows_estimate"] = gvar.New(rows)
	}
	return stat, nil
}
