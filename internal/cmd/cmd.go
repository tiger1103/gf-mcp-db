package cmd

import (
	"context"
	"gf-mcp/internal/consts"
	"gf-mcp/internal/mcp/register"
	"gf-mcp/internal/mcp/tools"
	"gf-mcp/library/liberr"
	"os"
	"strings"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/os/glog"
	"github.com/mark3labs/mcp-go/server"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/os/gcmd"
	"github.com/gogf/gf/v2/os/gctx"
)

var (
	Main = gcmd.Command{
		Name:  "main",
		Usage: "main",
		Brief: "start universal database mcp server",
		Func: func(ctx context.Context, parser *gcmd.Parser) (err error) {
			// 检查是否应该使用 stdio 模式
			if isStdioMode() {
				return runStdioMode(ctx)
			}

			// HTTP SSE 模式
			return runHttpMode(ctx, parser)
		},
	}
)

// DbConfig 数据库配置
type DbConfig struct {
	DBType   string
	Host     string
	Port     string
	Username string
	Password string
	Database string
	Charset  string
	Debug    bool
}

// runStdioMode 运行 stdio 模式
func runStdioMode(ctx context.Context) error {
	g.Log().SetFlags(glog.F_ASYNC | glog.F_TIME_DATE | glog.F_TIME_TIME | glog.F_FILE_LONG)
	g.Log().Info(ctx, "Universal Database MCP server starting with stdio mode")

	// 从命令行参数解析数据库配置
	args := os.Args[1:] // 跳过程序名
	dbConfig := parseDbConfigFromArgs(args)

	// 如果提供了数据库配置，则初始化连接
	if dbConfig != nil && dbConfig.DBType != "" {
		g.Log().Info(ctx, "初始化数据库连接，类型："+dbConfig.DBType+", 数据库："+dbConfig.Database)
		if err := initDatabaseConnection(ctx, dbConfig); err != nil {
			g.Log().Error(ctx, "初始化数据库连接失败:", err)
			return err
		}
	} else {
		g.Log().Warning(ctx, "未提供数据库配置，将在工具调用时处理")
	}

	// 创建 MCPServer
	mcpServer := server.NewMCPServer("universal-db-mcp", "1.0.0")

	// 工具注册
	register.DoRegister(&tools.Reg{})
	register.DoHandler(mcpServer)

	// 创建 StdioServer
	stdioServer := server.NewStdioServer(mcpServer)

	// 启动 stdio 服务
	return stdioServer.Listen(ctx, os.Stdin, os.Stdout)
}

// initDatabaseConnection 初始化数据库连接
func initDatabaseConnection(ctx context.Context, config *DbConfig) error {
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

	// 测试连接
	db := g.DB("default")
	if db == nil {
		return liberr.NewCode(consts.CodeInfo, "数据库连接失败")
	}

	g.Log().Info(ctx, "数据库连接初始化成功，类型："+config.DBType+", 数据库："+config.Database)
	return nil
}

// buildDSN 构建数据库连接字符串
func buildDSN(config *DbConfig) (string, error) {
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

// runHttpMode 运行 HTTP SSE 模式
func runHttpMode(ctx context.Context, parser *gcmd.Parser) error {
	g.Log().SetFlags(glog.F_ASYNC | glog.F_TIME_DATE | glog.F_TIME_TIME | glog.F_FILE_LONG)
	g.Log().Info(ctx, "Universal Database MCP server for sse starting")
	s := g.Server()
	s.Group("/", func(group *ghttp.RouterGroup) {
		group.Middleware(ghttp.MiddlewareHandlerResponse)
		// 导入 router 包并注册路由
		importRouter(ctx, s)
	})
	s.Run()
	return nil
}

// importRouter 导入 router 包并注册路由（避免循环导入）
func importRouter(ctx context.Context, s *ghttp.Server) {
	// 这里直接复制 router.Register 的逻辑
	importRouterFunc(ctx, s)
}

// importRouterFunc 路由注册函数
func importRouterFunc(ctx context.Context, s *ghttp.Server) {
	// 创建一个新的 MCPServer 实例
	mcpServer := server.NewMCPServer("universal-db-mcp", "1.0.0")

	// 工具注册 - 数据库操作工具
	register.DoRegister(&tools.Reg{})

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

// handleDatabaseConnection 处理数据库连接初始化（HTTP 模式）
func handleDatabaseConnection(ctx context.Context, r *ghttp.Request) {
	sessionId := r.GetSessionId()
	config := parseDatabaseConfigFromQuery(r)

	// 如果查询参数中没有 DBType，则尝试从请求头获取
	if config.DBType == "" {
		config = parseDatabaseConfigFromHeader(r)
	}

	// 如果没有数据库配置，则不初始化连接
	if config.DBType == "" {
		g.Log().Debug(ctx, "未提供数据库配置，跳过数据库连接初始化")
		return
	}

	// 验证必要参数
	if config.DBType != "sqlite" {
		if config.Host == "" || config.Port == "" || config.Username == "" || config.Database == "" {
			g.Log().Error(ctx, "数据库配置不完整，需要：type, host, port, user, database")
			return
		}
	} else {
		if config.Database == "" {
			g.Log().Error(ctx, "SQLite 需要提供 database 参数（文件路径）")
			return
		}
	}

	// 构建 DSN
	dsn, err := buildDSN(&DbConfig{
		DBType:   config.DBType,
		Host:     config.Host,
		Port:     config.Port,
		Username: config.Username,
		Password: config.Password,
		Database: config.Database,
		Charset:  config.Charset,
		Debug:    config.Debug,
	})
	if err != nil {
		g.Log().Error(ctx, "构建 DSN 失败:", err)
		return
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
		g.Log().Error(ctx, "配置数据库连接失败:", err)
		return
	}

	// 测试连接 - 使用 "default" 组名获取数据库连接
	db := g.DB("default")
	if db == nil {
		g.Log().Error(ctx, "数据库连接失败")
		return
	}

	g.Log().Info(ctx, "数据库连接初始化成功，类型："+config.DBType+", 数据库："+config.Database+", SessionID:"+sessionId)
}

// parseDatabaseConfigFromQuery 从 URL 查询参数解析数据库配置
func parseDatabaseConfigFromQuery(r *ghttp.Request) *DbConfig {
	charset := r.Get("charset", "utf8mb4").String()
	debug := r.Get("debug", "false").Bool()
	return &DbConfig{
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
func parseDatabaseConfigFromHeader(r *ghttp.Request) *DbConfig {
	charset := r.GetHeader("X-DB-Charset")
	if charset == "" {
		charset = "utf8mb4"
	}
	debug := r.GetHeader("X-DB-Debug") == "true"
	return &DbConfig{
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

// isStdioMode 检查是否应该使用 stdio 模式
func isStdioMode() bool {
	// 检查是否有 --stdio 标志
	for _, arg := range os.Args {
		if arg == "--stdio" || arg == "-s" {
			return true
		}
	}
	// 检查是否有数据库连接参数（stdio 模式特征）
	for _, arg := range os.Args {
		if strings.HasPrefix(arg, "--type=") || strings.HasPrefix(arg, "--host=") ||
			strings.HasPrefix(arg, "--database=") || arg == "--type" || arg == "--host" || arg == "--database" {
			return true
		}
	}
	return false
}

// parseDbConfigFromArgs 从命令行参数解析数据库配置
func parseDbConfigFromArgs(args []string) *DbConfig {
	config := &DbConfig{
		Charset: "utf8mb4",
	}

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--type":
			if i+1 < len(args) {
				config.DBType = args[i+1]
				i++
			}
		case "--host":
			if i+1 < len(args) {
				config.Host = args[i+1]
				i++
			}
		case "--port":
			if i+1 < len(args) {
				config.Port = args[i+1]
				i++
			}
		case "--user":
			if i+1 < len(args) {
				config.Username = args[i+1]
				i++
			}
		case "--password":
			if i+1 < len(args) {
				config.Password = args[i+1]
				i++
			}
		case "--database":
			if i+1 < len(args) {
				config.Database = args[i+1]
				i++
			}
		case "--charset":
			if i+1 < len(args) {
				config.Charset = args[i+1]
				i++
			}
		case "--debug":
			config.Debug = true
		}
	}

	return config
}

// 确保在包初始化时设置上下文
func init() {
	_ = gctx.GetInitCtx()
}
