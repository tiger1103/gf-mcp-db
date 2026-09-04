/*
 * @desc:gdb 驱动转发层：统一 gdb.Instance 与 g.DB 两条实例注册表
 *
 * 背景：GoFrame v2.9 存在两套互不相通的 gdb.DB 实例注册表：
 *  1. gdb.instances：由 gdb.Instance(group) 创建并缓存，gdb.SetConfig/SetConfigGroup 时整体清空重建；
 *  2. frame/gins 的 instance 表：由 g.DB(group)（即 gins.Database）创建并缓存，进程内永久保留，
 *     且每次工厂调用都会经 gdb.NewByGroup 新建一个全新 Core。
 * 两套注册表各自经 driver.New 创建独立的 Core 与连接池，导致：
 *  - 同一配置出现两个连接池（Windows 下 SQLite 文件句柄无法随单侧 Close 释放，t.TempDir 清理失败）；
 *  - g.DB 的缓存实例在配置切换后（HTTP 模式按请求 dbconn.Init、测试多库轮跑）永久指向旧库，
 *    工具会静默操作错误的数据库。
 *
 * 方案：以转发驱动包装全部 contrib 驱动。所有 driver.New 只返回同一个 forwardDB 代理实例，
 * 代理将 gdb.DB 的全部方法原子转发到「最近一次初始化配置」对应的真实连接：
 *  - 相同配置的重复创建（另一套注册表的工厂调用）直接复用现有连接池，不产生新连接；
 *  - 配置变化时创建新连接池、关闭并替换旧连接池（避免旧句柄/连接泄漏）。
 * 效果：g.DB("default") 与 gdb.Instance("default") 返回同一对象，且始终对应当前 dbconn.Init 的配置；
 * 任一注册表侧的 Close 都会释放当前配置的真实连接池。
 */

package dbconn

import (
	"context"
	"database/sql"
	"sync/atomic"
	"time"

	"github.com/gogf/gf/contrib/drivers/dm/v2"
	"github.com/gogf/gf/contrib/drivers/mssql/v2"
	"github.com/gogf/gf/contrib/drivers/mysql/v2"
	"github.com/gogf/gf/contrib/drivers/oracle/v2"
	"github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/contrib/drivers/sqlite/v2"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/os/gcache"
	"github.com/gogf/gf/v2/os/glog"
)

// dbEntry 当前生效的真实连接及其配置节点
type dbEntry struct {
	db   gdb.DB
	node gdb.ConfigNode
}

var (
	// forwardInstance 全局唯一代理实例：g.DB("default") 与 gdb.Instance("default") 缓存的均为它
	forwardInstance = &forwardDB{}
	// currentEntry 当前真实连接（原子读写；仅在 driver.New 内发生替换）
	currentEntry atomic.Pointer[dbEntry]
)

// forwardDB gdb.DB 转发代理：全部方法转发到当前真实连接
type forwardDB struct{}

// current 返回当前真实连接（未初始化时 panic，与 g.DB 无配置时的 panic 语义一致）
func (p *forwardDB) current() gdb.DB {
	if e := currentEntry.Load(); e != nil {
		return e.db
	}
	panic("dbconn: 数据库尚未初始化，请先调用 dbconn.Init")
}

// forwardDriver 转发驱动：包装 contrib 驱动，接管 gdb 两套注册表的实例创建
type forwardDriver struct {
	base gdb.Driver
}

// New 实现 gdb.Driver：相同配置复用现有连接池；配置变化则替换并关闭旧连接池；
// 始终返回全局唯一的 forwardInstance 代理。core 为本次调用新建的 Core，
// 在复用分支中被直接丢弃（其连接懒加载，未使用即无句柄泄漏）。
func (d *forwardDriver) New(core *gdb.Core, node *gdb.ConfigNode) (gdb.DB, error) {
	if e := currentEntry.Load(); e != nil && e.node == *node {
		return forwardInstance, nil
	}
	real, err := d.base.New(core, node)
	if err != nil {
		return nil, err
	}
	old := currentEntry.Swap(&dbEntry{db: real, node: *node})
	if old != nil {
		_ = old.db.Close(context.Background()) // 配置已切换，释放旧连接池（Close 幂等）
	}
	return forwardInstance, nil
}

func init() {
	// 覆盖 contrib 驱动的自身注册（本包 init 晚于所依赖的 contrib 包 init）
	for name, base := range map[string]gdb.Driver{
		"mysql":  mysql.New(),
		"pgsql":  pgsql.New(),
		"sqlite": sqlite.New(),
		"mssql":  mssql.New(),
		"oracle": oracle.New(),
		"dm":     dm.New(),
	} {
		if err := gdb.Register(name, &forwardDriver{base: base}); err != nil {
			panic("dbconn: 注册转发驱动失败 " + name + ": " + err.Error())
		}
	}
}

// 编译期确认 forwardDB 实现了完整 gdb.DB 接口
var _ gdb.DB = (*forwardDB)(nil)

// —— 以下为 gdb.DB 全量转发实现 ——

func (p *forwardDB) Model(tableNameOrStruct ...interface{}) *gdb.Model {
	return p.current().Model(tableNameOrStruct...)
}

func (p *forwardDB) Raw(rawSql string, args ...interface{}) *gdb.Model {
	return p.current().Raw(rawSql, args...)
}

func (p *forwardDB) Schema(schema string) *gdb.Schema {
	return p.current().Schema(schema)
}

func (p *forwardDB) With(objects ...interface{}) *gdb.Model {
	return p.current().With(objects...)
}

func (p *forwardDB) Open(config *gdb.ConfigNode) (*sql.DB, error) {
	return p.current().Open(config)
}

// Ctx 返回当前真实连接绑定上下文后的副本（短生命周期对象，不经过代理）
func (p *forwardDB) Ctx(ctx context.Context) gdb.DB {
	return p.current().Ctx(ctx)
}

func (p *forwardDB) Close(ctx context.Context) error {
	return p.current().Close(ctx)
}

func (p *forwardDB) Query(ctx context.Context, sql string, args ...interface{}) (gdb.Result, error) {
	return p.current().Query(ctx, sql, args...)
}

func (p *forwardDB) Exec(ctx context.Context, sql string, args ...interface{}) (sql.Result, error) {
	return p.current().Exec(ctx, sql, args...)
}

func (p *forwardDB) Prepare(ctx context.Context, sql string, execOnMaster ...bool) (*gdb.Stmt, error) {
	return p.current().Prepare(ctx, sql, execOnMaster...)
}

func (p *forwardDB) Insert(ctx context.Context, table string, data interface{}, batch ...int) (sql.Result, error) {
	return p.current().Insert(ctx, table, data, batch...)
}

func (p *forwardDB) InsertIgnore(ctx context.Context, table string, data interface{}, batch ...int) (sql.Result, error) {
	return p.current().InsertIgnore(ctx, table, data, batch...)
}

func (p *forwardDB) InsertAndGetId(ctx context.Context, table string, data interface{}, batch ...int) (int64, error) {
	return p.current().InsertAndGetId(ctx, table, data, batch...)
}

func (p *forwardDB) Replace(ctx context.Context, table string, data interface{}, batch ...int) (sql.Result, error) {
	return p.current().Replace(ctx, table, data, batch...)
}

func (p *forwardDB) Save(ctx context.Context, table string, data interface{}, batch ...int) (sql.Result, error) {
	return p.current().Save(ctx, table, data, batch...)
}

func (p *forwardDB) Update(ctx context.Context, table string, data interface{}, condition interface{}, args ...interface{}) (sql.Result, error) {
	return p.current().Update(ctx, table, data, condition, args...)
}

func (p *forwardDB) Delete(ctx context.Context, table string, condition interface{}, args ...interface{}) (sql.Result, error) {
	return p.current().Delete(ctx, table, condition, args...)
}

func (p *forwardDB) DoSelect(ctx context.Context, link gdb.Link, sql string, args ...interface{}) (result gdb.Result, err error) {
	return p.current().DoSelect(ctx, link, sql, args...)
}

func (p *forwardDB) DoInsert(ctx context.Context, link gdb.Link, table string, data gdb.List, option gdb.DoInsertOption) (result sql.Result, err error) {
	return p.current().DoInsert(ctx, link, table, data, option)
}

func (p *forwardDB) DoUpdate(ctx context.Context, link gdb.Link, table string, data interface{}, condition string, args ...interface{}) (result sql.Result, err error) {
	return p.current().DoUpdate(ctx, link, table, data, condition, args...)
}

func (p *forwardDB) DoDelete(ctx context.Context, link gdb.Link, table string, condition string, args ...interface{}) (result sql.Result, err error) {
	return p.current().DoDelete(ctx, link, table, condition, args...)
}

func (p *forwardDB) DoQuery(ctx context.Context, link gdb.Link, sql string, args ...interface{}) (result gdb.Result, err error) {
	return p.current().DoQuery(ctx, link, sql, args...)
}

func (p *forwardDB) DoExec(ctx context.Context, link gdb.Link, sql string, args ...interface{}) (result sql.Result, err error) {
	return p.current().DoExec(ctx, link, sql, args...)
}

func (p *forwardDB) DoFilter(ctx context.Context, link gdb.Link, sql string, args []interface{}) (newSql string, newArgs []interface{}, err error) {
	return p.current().DoFilter(ctx, link, sql, args)
}

func (p *forwardDB) DoCommit(ctx context.Context, in gdb.DoCommitInput) (out gdb.DoCommitOutput, err error) {
	return p.current().DoCommit(ctx, in)
}

func (p *forwardDB) DoPrepare(ctx context.Context, link gdb.Link, sql string) (*gdb.Stmt, error) {
	return p.current().DoPrepare(ctx, link, sql)
}

func (p *forwardDB) GetAll(ctx context.Context, sql string, args ...interface{}) (gdb.Result, error) {
	return p.current().GetAll(ctx, sql, args...)
}

func (p *forwardDB) GetOne(ctx context.Context, sql string, args ...interface{}) (gdb.Record, error) {
	return p.current().GetOne(ctx, sql, args...)
}

func (p *forwardDB) GetValue(ctx context.Context, sql string, args ...interface{}) (gdb.Value, error) {
	return p.current().GetValue(ctx, sql, args...)
}

func (p *forwardDB) GetArray(ctx context.Context, sql string, args ...interface{}) ([]gdb.Value, error) {
	return p.current().GetArray(ctx, sql, args...)
}

func (p *forwardDB) GetCount(ctx context.Context, sql string, args ...interface{}) (int, error) {
	return p.current().GetCount(ctx, sql, args...)
}

func (p *forwardDB) GetScan(ctx context.Context, objPointer interface{}, sql string, args ...interface{}) error {
	return p.current().GetScan(ctx, objPointer, sql, args...)
}

func (p *forwardDB) Union(unions ...*gdb.Model) *gdb.Model {
	return p.current().Union(unions...)
}

func (p *forwardDB) UnionAll(unions ...*gdb.Model) *gdb.Model {
	return p.current().UnionAll(unions...)
}

func (p *forwardDB) Master(schema ...string) (*sql.DB, error) {
	return p.current().Master(schema...)
}

func (p *forwardDB) Slave(schema ...string) (*sql.DB, error) {
	return p.current().Slave(schema...)
}

func (p *forwardDB) PingMaster() error {
	return p.current().PingMaster()
}

func (p *forwardDB) PingSlave() error {
	return p.current().PingSlave()
}

func (p *forwardDB) Begin(ctx context.Context) (gdb.TX, error) {
	return p.current().Begin(ctx)
}

func (p *forwardDB) BeginWithOptions(ctx context.Context, opts gdb.TxOptions) (gdb.TX, error) {
	return p.current().BeginWithOptions(ctx, opts)
}

func (p *forwardDB) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) error {
	return p.current().Transaction(ctx, f)
}

func (p *forwardDB) TransactionWithOptions(ctx context.Context, opts gdb.TxOptions, f func(ctx context.Context, tx gdb.TX) error) error {
	return p.current().TransactionWithOptions(ctx, opts, f)
}

func (p *forwardDB) GetCache() *gcache.Cache {
	return p.current().GetCache()
}

func (p *forwardDB) SetDebug(debug bool) {
	p.current().SetDebug(debug)
}

func (p *forwardDB) GetDebug() bool {
	return p.current().GetDebug()
}

func (p *forwardDB) GetSchema() string {
	return p.current().GetSchema()
}

func (p *forwardDB) GetPrefix() string {
	return p.current().GetPrefix()
}

func (p *forwardDB) GetGroup() string {
	return p.current().GetGroup()
}

func (p *forwardDB) SetDryRun(enabled bool) {
	p.current().SetDryRun(enabled)
}

func (p *forwardDB) GetDryRun() bool {
	return p.current().GetDryRun()
}

func (p *forwardDB) SetLogger(logger glog.ILogger) {
	p.current().SetLogger(logger)
}

func (p *forwardDB) GetLogger() glog.ILogger {
	return p.current().GetLogger()
}

func (p *forwardDB) GetConfig() *gdb.ConfigNode {
	return p.current().GetConfig()
}

func (p *forwardDB) SetMaxIdleConnCount(n int) {
	p.current().SetMaxIdleConnCount(n)
}

func (p *forwardDB) SetMaxOpenConnCount(n int) {
	p.current().SetMaxOpenConnCount(n)
}

func (p *forwardDB) SetMaxConnLifeTime(d time.Duration) {
	p.current().SetMaxConnLifeTime(d)
}

func (p *forwardDB) Stats(ctx context.Context) []gdb.StatsItem {
	return p.current().Stats(ctx)
}

func (p *forwardDB) GetCtx() context.Context {
	return p.current().GetCtx()
}

func (p *forwardDB) GetCore() *gdb.Core {
	return p.current().GetCore()
}

func (p *forwardDB) GetChars() (charLeft string, charRight string) {
	return p.current().GetChars()
}

func (p *forwardDB) Tables(ctx context.Context, schema ...string) (tables []string, err error) {
	return p.current().Tables(ctx, schema...)
}

func (p *forwardDB) TableFields(ctx context.Context, table string, schema ...string) (map[string]*gdb.TableField, error) {
	return p.current().TableFields(ctx, table, schema...)
}

func (p *forwardDB) ConvertValueForField(ctx context.Context, fieldType string, fieldValue interface{}) (interface{}, error) {
	return p.current().ConvertValueForField(ctx, fieldType, fieldValue)
}

func (p *forwardDB) ConvertValueForLocal(ctx context.Context, fieldType string, fieldValue interface{}) (interface{}, error) {
	return p.current().ConvertValueForLocal(ctx, fieldType, fieldValue)
}

func (p *forwardDB) CheckLocalTypeForField(ctx context.Context, fieldType string, fieldValue interface{}) (gdb.LocalType, error) {
	return p.current().CheckLocalTypeForField(ctx, fieldType, fieldValue)
}

func (p *forwardDB) FormatUpsert(columns []string, list gdb.List, option gdb.DoInsertOption) (string, error) {
	return p.current().FormatUpsert(columns, list, option)
}

func (p *forwardDB) OrderRandomFunction() string {
	return p.current().OrderRandomFunction()
}
