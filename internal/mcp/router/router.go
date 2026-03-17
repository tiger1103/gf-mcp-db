/*
 * @desc:MCP 路由注册
 * @company:云南奇讯科技有限公司
 * @Author: yixiaohu<yxh669@qq.com>
 * @Date:   2025/4/23 09:38
 */

package router

import (
	"context"
	"gf-mcp/internal/consts"
	"gf-mcp/internal/mcp/register"
	"gf-mcp/internal/mcp/tools"
	"gf-mcp/library/liberr"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/os/glog"
	"github.com/mark3labs/mcp-go/server"
)

// DatabaseConfig 数据库连接配置
type DatabaseConfig struct {
	DBType   string
	Host     string
	Port     string
	Username string
	Password string
	Database string
	Charset  string
	Debug    bool
}

// parseDatabaseConfigFromQuery 从 URL 查询参数解析数据库配置
func parseDatabaseConfigFromQuery(r *ghttp.Request) *DatabaseConfig {
	charset := r.Get("charset", "utf8mb4").String()
	debug := r.Get("debug", "false").Bool()
	return &DatabaseConfig{
		DBType:   r.Get("type").String(),
		Host:     r.Get("host").String(),
		Port:     r.Get("port").String(),
		Username: r.Get("user").String(),
		Password: r.Get("password").String(),
		Database: r.Get("database").String(),
		Charset:  charset,
		Debug:    debug,
	}
}

// parseDatabaseConfigFromHeader 从请求头解析数据库配置
func parseDatabaseConfigFromHeader(r *ghttp.Request) *DatabaseConfig {
	charset := r.GetHeader("X-DB-Charset")
	if charset == "" {
		charset = "utf8mb4"
	}
	debug := r.GetHeader("X-DB-Debug") == "true"
	return &DatabaseConfig{
		DBType:   r.GetHeader("X-DB-Type"),
		Host:     r.GetHeader("X-DB-Host"),
		Port:     r.GetHeader("X-DB-Port"),
		Username: r.GetHeader("X-DB-User"),
		Password: r.GetHeader("X-DB-Password"),
		Database: r.GetHeader("X-DB-Database"),
		Charset:  charset,
		Debug:    debug,
	}
}

// buildDSN 构建数据库连接字符串
func buildDSN(config *DatabaseConfig) (string, error) {
	var dsn string
	switch config.DBType {
	case "mysql":
		dsn = "mysql:" + config.Username + ":" + config.Password + "@tcp(" + config.Host + ":" + config.Port + ")/" + config.Database + "?charset=" + config.Charset + "&loc=Local"
	case "postgres", "postgresql":
		dsn = "pgsql:" + config.Username + ":" + config.Password + "@" + config.Host + ":" + config.Port + "/" + config.Database + "?sslmode=disable"
	case "sqlite":
		dsn = "sqlite:" + config.Database
	default:
		return "", liberr.NewCode(consts.CodeInfo, "不支持的数据库类型："+config.DBType)
	}
	return dsn, nil
}

// initDatabaseConnection 初始化数据库连接
func initDatabaseConnection(ctx context.Context, r *ghttp.Request, sessionId string) error {
	// 优先从 URL 查询参数获取配置
	config := parseDatabaseConfigFromQuery(r)

	// 如果查询参数中没有 DBType，则尝试从请求头获取
	if config.DBType == "" {
		config = parseDatabaseConfigFromHeader(r)
	}

	// 如果没有数据库配置，则不初始化连接
	if config.DBType == "" {
		g.Log().Debug(ctx, "未提供数据库配置，跳过数据库连接初始化")
		return nil
	}

	// 验证必要参数
	if config.DBType != "sqlite" {
		if config.Host == "" || config.Port == "" || config.Username == "" || config.Database == "" {
			return liberr.NewCode(consts.CodeInfo, "数据库配置不完整，需要：type, host, port, user, database")
		}
	} else {
		if config.Database == "" {
			return liberr.NewCode(consts.CodeInfo, "SQLite 需要提供 database 参数（文件路径）")
		}
	}

	// 构建 DSN
	dsn, err := buildDSN(config)
	if err != nil {
		return err
	}

	// 配置数据库连接 - 使用 "default" 作为默认组名
	err = gdb.SetConfig(gdb.Config{
		"default": gdb.ConfigGroup{
			gdb.ConfigNode{
				Link:    dsn,
				Debug:   config.Debug,
				Charset: config.Charset,
			},
		},
	})
	if err != nil {
		return err
	}

	// 测试连接 - 使用 "default" 组名获取数据库连接
	db := g.DB("default")
	if db == nil {
		return liberr.NewCode(consts.CodeInfo, "数据库连接失败")
	}

	g.Log().Info(ctx, "数据库连接初始化成功，类型："+config.DBType+", 数据库："+config.Database)
	return nil
}

// handleDatabaseConnection 处理数据库连接初始化
func handleDatabaseConnection(ctx context.Context, r *ghttp.Request) {
	sessionId := r.GetSessionId()
	if err := initDatabaseConnection(ctx, r, sessionId); err != nil {
		glog.Error(ctx, "初始化数据库连接失败:", err)
	}
}

// Register 注册 MCP 服务
func Register(ctx context.Context, s *ghttp.Server) {
	// 创建一个新的 MCPServer 实例
	mcpServer := server.NewMCPServer("universal-db-mcp", "1.0.0")

	// 工具注册 - 数据库操作工具
	register.DoRegister(new(tools.Reg))

	// 添加到 mcp 服务
	register.DoHandler(mcpServer)

	// 创建一个新的 SSEServer 实例，并设置端点路径为 /mcp
	sseServer := server.NewSSEServer(mcpServer,
		server.WithSSEEndpoint("/mcp"),
	)

	// 获取 SSE 和消息端点路径
	ssePath := sseServer.CompleteSsePath()
	messagePath := sseServer.CompleteMessagePath()

	// 将 SSEServer 的 SSE 端点和处理函数集成到 GoFrame 路由中（支持数据库连接配置）
	s.BindHandler(ssePath, func(r *ghttp.Request) {
		// 初始化数据库连接
		handleDatabaseConnection(ctx, r)
		sseServer.ServeHTTP(r.Response.Writer, r.Request)
	})

	// 将 SSEServer 的消息端点和处理函数集成到 GoFrame 路由中
	s.BindHandler(messagePath, func(r *ghttp.Request) {
		// 初始化数据库连接（从请求头获取配置）
		handleDatabaseConnection(ctx, r)
		sseServer.ServeHTTP(r.Response.Writer, r.Request)
	})
}
