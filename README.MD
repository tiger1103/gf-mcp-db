# Universal Database MCP Server

一个通用的数据库操作 MCP（Model Context Protocol）服务，支持多种数据库类型的数据操作。

## 功能特性

- 🔗 **数据库连接** - 支持 MySQL、PostgreSQL、SQLite 等多种数据库
- 📊 **数据查询** - 执行 SQL 查询并返回结果
- ✏️ **数据操作** - 执行 DDL/DML 语句
- 📋 **表管理** - 列出数据库表、查看表结构
- 🗄️ **数据库管理** - 列出所有数据库

## 支持的工具 (Tools)

连接数据库后，可以使用以下工具操作数据库：

### 1. query_database
数据库查询工具，用于执行 SELECT 查询操作。

**参数：**
- `sql` (必需): SQL 查询语句
- `limit`: 结果限制条数，默认 100

### 2. execute_sql
SQL 语句执行器，用于执行 DDL/DML 语句。

**参数：**
- `sql` (必需): SQL 语句内容

### 3. list_tables
列出数据库表工具。

**参数：**
- `database` (必需): 数据库名称
- `pattern`: 表名匹配模式，支持通配符 %

### 4. describe_table
查看表结构工具。

**参数：**
- `database` (必需): 数据库名称
- `table` (必需): 表名

### 5. list_databases
列出数据库服务器中的所有数据库。

## 快速开始

### 环境要求
- Go 1.23+
- GoFrame CLI (`gf`)
- MCP 客户端

### 安装依赖

```bash
go mod tidy
```

### 构建项目

使用 GoFrame CLI 构建项目：

```bash
gf build
```

构建完成后，二进制文件位于 `./bin/gf-mcp-db` 目录下。

**注意：** 二进制文件名由 `hack/config.yaml` 中的 `gfcli.build.name` 配置决定，默认为 `gf-mcp-db`。

## 连接方式

### 方式一：HTTP SSE 模式

#### 运行服务

```bash
# 使用 go run
go run main.go

# 或使用编译后的二进制文件
./bin/gf-mcp-db
```

服务默认运行在 `http://localhost:8000`

#### 客户端配置

```json
{
  "mcpServers": {
    "universal-db-mcp": {
      "url": "http://localhost:8000/mcp",
      "headers": {
        "X-DB-Type": "mysql",
        "X-DB-Host": "localhost",
        "X-DB-Port": "3306",
        "X-DB-User": "root",
        "X-DB-Password": "123456",
        "X-DB-Database": "test"
      }
    }
  }
}
```

或使用 URL 参数方式：

```json
{
  "mcpServers": {
    "universal-db-mcp": {
      "url": "http://localhost:8000/mcp?type=mysql&host=localhost&port=3306&user=root&password=123456&database=test"
    }
  }
}
```

### 方式二：Stdio 模式（推荐）

#### 运行服务

```bash
# 使用 go run
go run main.go --type mysql --host localhost --port 3306 --user root --password 123456 --database test

# 或使用编译后的二进制文件
./bin/gf-mcp-db --type mysql --host localhost --port 3306 --user root --password 123456 --database test
```

#### 客户端配置

使用编译后的二进制文件：

**注意不需要运行二进制文件，直接在mcp客户端配置会自动调用二进制文件，需要将二进制文件放到系统环境能调用的位置，或command中使用二进制文件绝对路径**

```json
{
  "mcpServers": {
    "my-database": {
      "command": "gf-mcp-db",
      "args": [
        "--type", "mysql",
        "--host", "localhost",
        "--port", "3306",
        "--user", "root",
        "--password", "your_password",
        "--database", "your_database"
      ]
    }
  }
}
```

Windows 系统：

```json
{
  "mcpServers": {
    "my-database": {
      "command": "gf-mcp-db.exe",
      "args": [
        "--type", "mysql",
        "--host", "localhost",
        "--port", "3306",
        "--user", "root",
        "--password", "your_password",
        "--database", "your_database"
      ]
    }
  }
}
```

**命令行参数说明：**
| 参数 | 必需 | 说明 | 默认值 |
|------|------|------|--------|
| --type | 是 | 数据库类型 (mysql, postgres, sqlite) | - |
| --host | 是* | 数据库主机地址 (SQLite 不需要) | - |
| --port | 是* | 数据库端口 (SQLite 不需要) | - |
| --user | 是* | 数据库用户名 (SQLite 不需要) | - |
| --password | 是* | 数据库密码 (SQLite 不需要) | - |
| --database | 是 | 数据库名称或 SQLite 文件路径 | - |
| --charset | 否 | 字符集 | utf8mb4 |
| --debug | 否 | 是否开启调试模式 | false |

*注：SQLite 类型只需要 --database 参数（文件路径）

#### Stdio 连接示例

**MySQL:**
```json
{
  "mcpServers": {
    "my-mysql": {
      "command": "./bin/gf-mcp-db",
      "args": [
        "--type", "mysql",
        "--host", "localhost",
        "--port", "3306",
        "--user", "root",
        "--password", "123456",
        "--database", "testdb"
      ]
    }
  }
}
```

**PostgreSQL:**
```json
{
  "mcpServers": {
    "my-postgres": {
      "command": "./bin/gf-mcp-db",
      "args": [
        "--type", "postgres",
        "--host", "localhost",
        "--port", "5432",
        "--user", "postgres",
        "--password", "123456",
        "--database", "testdb"
      ]
    }
  }
}
```

**SQLite:**
```json
{
  "mcpServers": {
    "my-sqlite": {
      "command": "./bin/gf-mcp-db",
      "args": [
        "--type", "sqlite",
        "--database", "/path/to/database.db"
      ]
    }
  }
}
```

## 项目结构

```
gf-mcp-db/
├── main.go                     # 程序入口
├── hack/
│   └── config.yaml             # GoFrame CLI 配置（打包配置）
├── internal/
│   ├── cmd/                    # 命令行处理（支持 stdio 和 HTTP 模式）
│   ├── mcp/
│   │   ├── router/             # 路由注册（HTTP 模式）
│   │   ├── register/           # MCP 注册器
│   │   ├── tools/              # MCP 工具实现
│   │   └── model/              # 数据模型
│   └── consts/                 # 常量定义
├── library/                    # 公共库
├── manifest/
│   └── config/                 # 配置文件
└── resource/                   # 资源文件
```

## 开发说明

### 添加新的 MCP 工具

1. 在 `internal/mcp/tools/` 目录下创建新的工具文件
2. 实现工具结构体，包含 `ReturnTool()` 和 `Handler()` 方法
3. 实现 `RegisterXxx()` 方法注册工具

### 工具开发模板

```go
package tools

import (
    "context"
    "github.com/mark3labs/mcp-go/mcp"
    "github.com/mark3labs/mcp-go/server"
)

type MyTool struct{}

func (t *MyTool) ReturnTool() mcp.Tool {
    return mcp.NewTool("my_tool",
        mcp.WithDescription("工具描述"),
        mcp.WithString("param1", mcp.Required(), mcp.Description("参数描述")),
    )
}

func (t *MyTool) Handler(r *Reg) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
    return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
        // 实现工具逻辑
        return mcp.NewToolResultText("操作成功"), nil
    }
}

func (r *Reg) RegisterMyTool() {
    register.AddHandler(func(mcpServer *server.MCPServer) {
        tool := new(MyTool)
        mcpServer.AddTool(tool.ReturnTool(), tool.Handler(r))
    })
}
```

### 打包说明

使用 GoFrame CLI 进行打包：

```bash
# 打包所有平台
gf build

# 打包指定平台
gf build -a amd64 -s linux
```

打包配置在 `hack/config.yaml` 中：

```yaml
gfcli:
  build:
    name: "gf-mcp-db"        # 二进制文件名
    arch: "amd64,arm64,arm"  # 支持的架构
    system: "linux,windows,darwin"  # 支持的操作系统
    path: "./bin"             # 输出目录
```

## 许可证

MIT License

## 参考

- [MCP Go](https://github.com/mark3labs/mcp-go)
- [GoFrame](https://goframe.org/)
