# 多数据库支持设计（SQLite / PostgreSQL / SQL Server / Oracle / DM）

- 日期：2026-09-04
- 状态：已与用户逐节确认
- 范围：gf-mcp-db 从「仅 MySQL 实际可用」扩展为 6 库统一支持，全部驱动零 CGO

## 1. 背景与目标

当前项目 7 个 MCP 工具全部硬编码 MySQL 专有语法（`SHOW TABLES`、`SHOW COLUMNS`、`SHOW INDEX`、`SHOW TABLE STATUS`、`FLUSH TABLES`、反引号标识符、`information_schema.KEY_COLUMN_USAGE + DATABASE()`），且 `internal/cmd/cmd.go` 与 `internal/mcp/router/router.go` 存在成对重复的连接配置/初始化代码。`go.mod` 虽引入 pgsql 驱动，但工具层 SQL 在非 MySQL 库上均会失败。

**目标**：支持 SQLite（不使用 CGO）、PostgreSQL、SQL Server、Oracle、DM（达梦）；7 个工具在全部 6 库上可用，输出采用统一跨库格式。

**用户决策记录**：

1. 输出格式：统一跨库格式（不严格兼容 MySQL 现有原始字段）。
2. cmd.go / router.go 重复代码：合并去重。
3. 测试环境：SQLite、MySQL（localhost:3306）、PG（192.168.0.214:5432）可真实连接；MSSQL/Oracle/DM 以编译验证 + 单测 + 基于官方文档的 SQL 审查兜底。
4. 实现方案：方案 A（gdb 元数据 API 打底 + 轻量方言层）。

## 2. 驱动选型（关键调研结论）

| 数据库 | gf contrib 驱动 | 底层依赖 | CGO |
|--------|-----------------|----------|-----|
| MySQL | `contrib/drivers/mysql/v2`（已引入） | go-sql-driver/mysql | 无 |
| PostgreSQL | `contrib/drivers/pgsql/v2`（已引入） | lib/pq | 无 |
| SQLite | `contrib/drivers/sqlite/v2`（新增） | glebarez/go-sqlite（modernc.org/sqlite） | 无 |
| SQL Server | `contrib/drivers/mssql/v2`（新增） | microsoft/go-mssqldb | 无 |
| Oracle | `contrib/drivers/oracle/v2`（新增） | sijms/go-ora（纯 Go OCI） | 无 |
| DM | `contrib/drivers/dm/v2`（新增） | gitee.com/chunanyong/dm | 无 |

- contrib 驱动与 gf 主库同版本 `v2.9.0`，保持 go.mod 一致。
- 全部纯 Go，`CGO_ENABLED=0` 交叉编译（hack/config.yaml 多平台打包）不受影响。
- 6 个官方驱动均实现 `gdb.DB.Tables(ctx)` 与 `gdb.DB.TableFields(ctx, table)`，以及 `GetChars()`（mysql/sqlite 反引号，其余双引号）。
- 各驱动 `Open()` 只消费 `ConfigNode` 字段（Host/Port/User/Pass/Name/Extra/Charset），不走 Link 字符串（gf 的 Link 解析正则 `type:user:pass@protocol(host)/db?...` 对 sqlite 文件路径等场景不适用）。
- 官方已知限制（写入文档即可，不需代码规避）：Oracle 无 `LastInsertId`；MSSQL 自动时间戳仅支持 datetime2/datetimeoffset；DM 的 InsertIgnore 需主键/唯一索引。

## 3. 架构

```
main.go ── 导入 6 个驱动（全部纯 Go）
   │
   ├─ stdio 模式 ──────────────┐
   │                          ▼
   │               internal/dbconn（新）
   │     · NormalizeType("postgres"/"postgresql"→"pgsql"、"sqlserver"→"mssql" 等)
   │     · Validate（sqlite 只需 database=文件路径；其余需 host/port/user/database）
   │     · BuildConfigNode → gdb.ConfigNode{Type,Host,Port,User,Pass,Name,Charset,Extra,Debug}
   │     · Init(ctx, cfg)：SetConfig + Ping 验证
   │     端口默认值：mysql 3306 / pgsql 5432 / mssql 1433 / oracle 1521 / dm 5236
   │
   └─ HTTP SSE 模式：router.go 解析 query/header 后调用 dbconn.Init
   │
   ▼
internal/mcp/tools（7 个工具，逻辑重构）
   │
   ▼
internal/mcp/dialect（新）
     · Get(dbType) → IDialect（注册表模式）
     · IDialect：QuoteIdent(name)、QuoteString(s)、Paginate(selectSQL, limit)、
                Indexes(ctx, db, table)、ForeignKeys(ctx, db, table)、TableStat(ctx, db, table)
     · mysql / pgsql / sqlite / mssql / oracle / dm 六实现
```

**关键决策：**

1. **不拼 Link 字符串**：直接填充 `gdb.ConfigNode` 字段，绕开 gf Link 正则限制；与 mssql/oracle/dm 驱动的 `Open()` 消费方式一致。
2. **标识符引用**统一经 `dialect.QuoteIdent`（底层 `db.GetChars()`），工具层禁止硬编码反引号。
3. **驱动导入**集中在 `main.go`（含 `mssql`、`oracle`、`dm`、`sqlite` 四个新 blank import）。
4. **`extra` 透传**：新增可选参数（CLI `--extra`、HTTP query `extra` / header `X-DB-Extra`，格式 `k1=v1&k2=v2`），写入 `ConfigNode.Extra`，由各驱动 `Open()` 自行解析（如 Oracle 连接超时、MSSQL 加密开关）。
5. **`clear_cache` 语义修正**：废除 `FLUSH TABLES`（MySQL 专有），统一清除 gdb 内部元数据缓存（`db.GetCore().GetInnerMemCache()`，覆盖 Tables/TableFields 缓存）；`table` 参数保留，作为提示信息展示（内部缓存无表级粒度时全清）。

## 4. 工具行为与统一输出

**统一列结构**（get_schema / get_table_info 共用，源自 `TableField`）：

```json
{
  "column_name": "id", "data_type": "bigint", "nullable": false,
  "primary_key": true, "default": null, "extra": "auto_increment", "comment": "主键"
}
```

| 工具 | 新行为 |
|------|--------|
| `get_table_list` | `db.Tables(ctx)`；`pattern` 在 Go 侧过滤（`%` 通配，语义与 SQL LIKE 一致） |
| `get_schema` | 表清单 + 每表 `TableFields` + `dialect.Indexes`；无 `SHOW COLUMNS/INDEX` |
| `get_table_info` | 列 + `Indexes` + `ForeignKeys` + `TableStat`（行数估算/引擎/注释按库可得性返回，拿不到则省略键） |
| `get_enum_values` | `SELECT DISTINCT {QuoteIdent(col)} FROM {QuoteIdent(table)} [WHERE …] {Paginate(limit)}`；列类型信息改由 `TableFields` 提供 |
| `get_sample_data` | 列清单改由 `TableFields`（脱敏规则不变），LIMIT 用 `Paginate`；`where`/`order` 为调用方提供的 SQL 片段，按现状原样拼接（不做二次解析，行为与 MySQL 时代一致） |
| `execute_query` | 透传用户 SQL 不改写；查询判定扩展为 `SELECT/WITH/SHOW/EXPLAIN/DESCRIBE/DESC/PRAGMA` 前缀；SELECT 结果仍在 Go 侧截断 limit；Oracle `LastInsertId` 恒为 0 照实返回 |
| `clear_cache` | 清 gdb 内部元数据缓存（见第 3 节决策 5） |

**方言差异要点**：

- `Paginate`：mysql/pg/sqlite/dm → `LIMIT n`；mssql → `SELECT TOP n …`（在 SELECT 后插入 TOP）；oracle → `FETCH FIRST n ROWS ONLY`（12c+）。
- `Indexes`：mysql `SHOW INDEX FROM`；pg `pg_indexes`；sqlite `PRAGMA index_list` + `PRAGMA index_info`；mssql `sys.indexes/sys.index_columns`；oracle/dm `all_indexes + all_ind_columns`。
- `ForeignKeys`：mysql `information_schema.KEY_COLUMN_USAGE`（`DATABASE()` 改为参数绑定）；pg `pg_constraint`；sqlite `PRAGMA foreign_key_list`；mssql `sys.foreign_keys + 外键列视图`；oracle/dm `all_constraints + all_cons_columns`。
- `TableStat`：mysql `SHOW TABLE STATUS`；pg `pg_class.reltuples`；mssql `sys.dm_db_partition_stats`；oracle/dm `all_tables.num_rows`；sqlite 无行数估算（省略）。

**错误处理**：沿用现有 `g.Try` + `liberr` panic 流与 `returnRes` 模式；未配置连接时错误文案不变。新增两类校验错误：不支持的 `type` 值（提示 6 种合法写法与别名）、`extra` 格式非法（需 `k=v&k=v`）。

## 5. 测试与验收

| 层 | 方式 | 覆盖 |
|----|------|------|
| 单元测试 | 纯 Go，不依赖外部库 | dbconn：类型归一、按库必填校验、ConfigNode 字段、extra 解析；dialect：Paginate/QuoteIdent 六实现、pattern 过滤 |
| 集成测试 | Go 测试 + stdio 冒烟 | SQLite（临时文件库）、MySQL（localhost:3306）、PG（192.168.0.214:5432，独立测试库建临时表） |
| 编译级验证 | `go build`（amd64 linux/windows）+ `go vet` | mssql/oracle/dm 全路径可编译、参数正确 |

- 集成测试用环境变量开关（如 `GF_MCP_TEST_PG_DSN`），未设置则 `t.Skip`；凭据不写入仓库。
- 验收标准：SQLite/MySQL/PG 上 7 工具全部返回统一格式结果；编译与 vet 通过。
- MSSQL/Oracle/DM：以 gf 官方驱动单测背书元数据正确性，方言 SQL 依据官方文档审查；README 标注"未经真实实例验证"。

## 6. 文档

README 更新：支持库列表与 6 种连接示例（Oracle 服务名、DM 默认端口说明）、`extra` 参数、`clear_cache` 行为变化、各库已知限制（Oracle 无 LastInsertId、MSSQL datetime2 注意事项等）。

## 7. 出范围事项（明确不做）

- 不引入 PG 多 schema（namespace）切换。
- 不做读写分离 / 连接池调优。
- 不升级 GoFrame 版本（保持 v2.9.0）。
- 不改 MCP 传输层（stdio / SSE 行为不变）。
