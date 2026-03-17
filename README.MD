# Universal Database MCP Server

一个通用的数据库操作 MCP（Model Context Protocol）服务，支持多种数据库类型的数据操作。

## 功能特性

| 工具名 | 用途 |
|--------|------|
| `execute_query` | 执行 SQL 查询或数据库命令（SELECT、INSERT、UPDATE、DELETE 等） |
| `get_table_list` | 列出当前数据库中的所有表或匹配特定模式的表 |
| `get_schema` | 获取数据库结构信息（所有表名、列名、数据类型、主键、索引等元数据） |
| `get_table_info` | 获取指定表的详细信息（列定义、索引、外键、预估行数等） |
| `clear_cache` | 清除 Schema 缓存（当数据库结构发生变化时使用） |
| `get_enum_values` | 获取指定列的所有唯一值（用于了解 status、type 等枚举类型的可能值） |
| `get_sample_data` | 获取表的示例数据（已自动脱敏，用于了解数据格式） |

这些工具旨在帮助开发者快速了解数据库结构、安全执行查询以及获取数据样本，提高数据库开发和调试效率。

## 支持的工具 (Tools)

连接数据库后，可以使用以下工具操作数据库：

### 1. execute_query
执行 SQL 查询或数据库命令，支持 SELECT、INSERT、UPDATE、DELETE 等操作。

**参数：**
- `sql` (必需): SQL 查询语句
- `limit`: 结果限制条数，默认 100，仅对 SELECT 查询有效

**使用示例：**
```json
{
  "name": "execute_query",
  "arguments": {
    "sql": "SELECT * FROM users LIMIT 10"
  }
}
```

### 2. get_table_list
列出当前数据库中的所有表或匹配特定模式的表。

**参数：**
- `pattern`: 表名匹配模式，支持通配符 %，例如 'user%' 匹配所有以 user 开头的表名

**使用示例：**
```json
{
  "name": "get_table_list",
  "arguments": {
    "pattern": "user%"
  }
}
```

### 3. get_schema
获取数据库的完整结构信息，包括所有表名、列名、数据类型、主键、索引等元数据。

**参数：**
- `pattern`: 表名匹配模式，支持通配符 %，例如 'user%' 匹配所有以 user 开头的表名

**使用示例：**
```json
{
  "name": "get_schema",
  "arguments": {}
}
```

### 4. get_table_info
获取指定表的详细信息，包括列定义、索引、外键、表注释、预估行数等。

**参数：**
- `table` (必需): 表名

**使用示例：**
```json
{
  "name": "get_table_info",
  "arguments": {
    "table": "users"
  }
}
```

### 5. clear_cache
清除数据库 Schema 缓存，当数据库结构发生变化时使用此工具刷新缓存。

**参数：**
- `table`: 可选，指定要清除缓存的表名，不传则清除所有缓存

**使用示例：**
```json
{
  "name": "clear_cache",
  "arguments": {}
}
```

### 6. get_enum_values
获取指定列的所有唯一值，用于了解 status、type 等枚举类型字段的可能取值。

**参数：**
- `table` (必需): 表名
- `column` (必需): 列名
- `where`: 可选的 WHERE 条件，用于过滤数据
- `limit`: 结果限制条数，默认 1000

**使用示例：**
```json
{
  "name": "get_enum_values",
  "arguments": {
    "table": "orders",
    "column": "status"
  }
}
```

### 7. get_sample_data
获取表的示例数据，已自动脱敏敏感信息（如密码、手机号、邮箱等），用于了解数据格式和结构。

**参数：**
- `table` (必需): 表名
- `where`: 可选的 WHERE 条件，用于过滤数据
- `order`: 可选的 ORDER BY 子句，例如 'id DESC'
- `limit`: 结果限制条数，默认 10

**使用示例：**
```json
{
  "name": "get_sample_data",
  "arguments": {
    "table": "users",
    "limit": 5
  }
}
```

> **注意：** 所有工具都使用 MCP 连接配置中指定的数据库，无需在工具调用时指定数据库名称。

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
