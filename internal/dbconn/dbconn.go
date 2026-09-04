/*
 * @desc:统一数据库连接配置解析、校验与初始化（stdio / HTTP 共用）
 */

package dbconn

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/gogf/gf/v2/database/gdb"

	"github.com/tiger1103/gf-mcp-db/internal/consts"
	"github.com/tiger1103/gf-mcp-db/library/liberr"
)

// Config 数据库连接配置（三种入口共用：CLI 参数 / URL query / HTTP header）
type Config struct {
	DBType   string // mysql | postgres/pgsql | sqlite | sqlserver/mssql | oracle | dm
	Host     string
	Port     string
	Username string
	Password string
	Database string // 数据库名；sqlite 时为文件路径
	Charset  string
	Extra    string // 透传驱动的额外参数，格式 k1=v1&k2=v2（注意：gf 会先用 Extra 覆盖 ConfigNode 同名字段，勿用 charset/type 等保留键）
	Debug    bool
}

// typeAliases 类型别名归一表
var typeAliases = map[string]string{
	"mysql":      "mysql",
	"pgsql":      "pgsql",
	"postgres":   "pgsql",
	"postgresql": "pgsql",
	"pg":         "pgsql",
	"sqlite":     "sqlite",
	"sqlite3":    "sqlite",
	"mssql":      "mssql",
	"sqlserver":  "mssql",
	"oracle":     "oracle",
	"dm":         "dm",
}

// defaultPorts 各数据库默认端口
var defaultPorts = map[string]string{
	"mysql":  "3306",
	"pgsql":  "5432",
	"mssql":  "1433",
	"oracle": "1521",
	"dm":     "5236",
}

// NormalizeType 归一数据库类型别名
func NormalizeType(dbType string) (string, bool) {
	t, ok := typeAliases[strings.ToLower(strings.TrimSpace(dbType))]
	return t, ok
}

// BuildConfigNode 归一/校验配置并构建 gdb.ConfigNode。
// 不使用 Link 字符串：直接填充字段，与各 contrib 驱动 Open() 的消费方式一致。
func BuildConfigNode(cfg *Config) (*gdb.ConfigNode, error) {
	dbType, ok := NormalizeType(cfg.DBType)
	if !ok {
		return nil, liberr.NewCode(consts.CodeInfo, fmt.Sprintf(
			"不支持的数据库类型：%q（支持：mysql、postgres/pgsql、sqlite、sqlserver/mssql、oracle、dm）", cfg.DBType))
	}
	if cfg.Extra != "" {
		// 自行校验：gstr.Parse 对无 "=" 的片段会静默跳过而非报错，
		// 故要求每个 "&" 分隔的片段都必须包含 "="（k1=v1&k2=v2）。
		for _, part := range strings.Split(cfg.Extra, "&") {
			if !strings.Contains(part, "=") {
				return nil, liberr.NewCode(consts.CodeInfo, fmt.Sprintf(
					"extra 参数格式非法，应为 k1=v1&k2=v2：%q", part))
			}
		}
	}
	node := &gdb.ConfigNode{
		Type: dbType,
		// 密码与文件路径可能合法包含空格，故仅 TrimSpace Host/Port
		Host:    strings.TrimSpace(cfg.Host),
		Port:    strings.TrimSpace(cfg.Port),
		User:    cfg.Username,
		Pass:    cfg.Password,
		Name:    cfg.Database,
		Charset: cfg.Charset,
		Extra:   cfg.Extra,
		Debug:   cfg.Debug,
	}
	if node.Port == "" {
		node.Port = defaultPorts[dbType]
	}
	if node.Charset == "" {
		switch dbType {
		case "mysql":
			node.Charset = "utf8mb4"
		case "dm":
			node.Charset = "UTF-8"
		}
	}
	if dbType == "mysql" && !strings.Contains(cfg.Extra, "loc=") {
		// 与旧版 DSN 行为一致：默认使用本地时区解析时间（可用 extra "loc=..." 覆盖）
		node.Timezone = "Local"
	}
	if dbType == "sqlite" {
		if node.Name == "" {
			return nil, liberr.NewCode(consts.CodeInfo, "SQLite 需要提供 database 参数（文件路径）")
		}
	} else {
		if node.Host == "" || node.User == "" || node.Name == "" {
			return nil, liberr.NewCode(consts.CodeInfo,
				"数据库配置不完整，需要：type, host, user, database（port 可省略，使用各库默认端口）")
		}
	}
	return node, nil
}

// initMu 串行化 Init 对全局连接配置的写入
var initMu sync.Mutex

// lastInitNode 记录最近一次成功初始化的连接配置，相同配置跳过重复 SetConfig+Ping（HTTP 模式按请求调用 Init）
var lastInitNode *gdb.ConfigNode

// restoreLastInit 初始化失败时恢复上一个成功配置，避免全局配置停留在失败配置上；
// 无可恢复配置时置空 lastInitNode，强制下次全量重初始化。
func restoreLastInit() {
	if lastInitNode == nil {
		return
	}
	if err := gdb.SetConfig(gdb.Config{
		gdb.DefaultGroupName: gdb.ConfigGroup{*lastInitNode},
	}); err != nil {
		lastInitNode = nil
	}
}

// Init 构建配置、写入全局默认连接组并 Ping 验证连通性。
// 相同配置的连续调用会跳过重复的 SetConfig 与 Ping（HTTP 模式按请求调用，避免连接池反复重建）。
func Init(ctx context.Context, cfg *Config) error {
	node, err := BuildConfigNode(cfg)
	if err != nil {
		return err
	}
	// 串行化全局连接配置写入，避免并发 Init 相互覆盖（HTTP 模式按请求初始化）
	initMu.Lock()
	defer initMu.Unlock()

	if lastInitNode != nil && *node == *lastInitNode {
		return nil
	}

	if err = gdb.SetConfig(gdb.Config{
		gdb.DefaultGroupName: gdb.ConfigGroup{*node},
	}); err != nil {
		return err
	}
	db, err := gdb.Instance(gdb.DefaultGroupName)
	if err != nil {
		restoreLastInit()
		return err
	}
	pingErr := make(chan error, 1)
	go func() {
		pingErr <- db.PingMaster()
	}()
	select {
	case err = <-pingErr:
		if err != nil {
			restoreLastInit()
			return err
		}
	case <-ctx.Done():
		restoreLastInit()
		return ctx.Err()
	}
	// 仅在成功后记录，失败的下次调用仍会重试
	lastInitNode = node
	return nil
}
