# mworm

`mworm` 是一个基于 `sqlx` 封装的 Go 语言 ORM 库，专为 PostgreSQL 和 MySQL 设计。它提供了一套流畅的 API 来构建 SQL 查询、处理复杂的条件逻辑、以及方便的结果集映射。

特别针对 PostgreSQL 的 JSONB、RETURNING 等特性进行了优化支持。

## 安装

```bash
go get github.com/ccxdd/mworm
```

## 快速开始

### 1. 初始化连接

在使用 `mworm` 之前，需要先绑定 `sqlx.DB` 对象。

```go
import (
    "github.com/ccxdd/mworm"
    "github.com/jmoiron/sqlx"
    _ "github.com/jackc/pgx/v5/stdlib"
    "log"
)

func initDB() {
    // 连接数据库
    db, err := sqlx.Connect("pgx", "postgres://user:password@localhost:5432/dbname?sslmode=disable")
    if err != nil {
        log.Fatal(err)
    }
    
    // 绑定到 mworm
    err = mworm.BindDB(db)
    if err != nil {
        log.Fatal(err)
    }
}
```

### 2. 定义模型

使用 `db` tag 映射数据库字段，`json` tag 用于 JSON 序列化及部分 mworm 内部逻辑（如分页结果）。

```go
type User struct {
    ID        int64     `json:"id" db:"id,pk"`          // pk 标识主键
    Name      string    `json:"name" db:"name"`
    Age       int       `json:"age" db:"age"`
    Status    int       `json:"status" db:"status"`
    CreatedAt time.Time `json:"createdAt" db:"created_at"`
}

// 实现 TableName 接口（可选，默认使用结构体名，但建议显式指定）
func (u User) TableName() string {
    return "users"
}
```

## 常用功能

### 1. CRUD 操作

#### 插入 (Insert)

```go
user := User{Name: "Tom", Age: 18, Status: 1}
// 基础插入
err := mworm.INSERT(user).Exec()

// PostgreSQL 支持 RETURNING
var id int64
err := mworm.INSERT(user).RETURNING(&id, nil, "id")
```

#### 查询 (Select)

```go
// 单条查询
var user User
err := mworm.SELECT(User{}).
    Where(mworm.Eq("id", 1)).
    One(&user)

// 多条查询
var users []User
err := mworm.SELECT(User{}).
    Where(mworm.Gt("age", 18)).
    Asc("age").
    Limit(10).
    Many(&users)
```

#### 更新 (Update)

```go
// 更新整个结构体（非空字段）
user.Name = "Jerry"
err := mworm.UPDATE(user).
    Where(mworm.Eq("id", 1)).
    Exec()

// 更新指定字段
err := mworm.UPDATE(User{}).
    SetField("name", "Jerry").
    SetField("status", 2).
    Where(mworm.Eq("id", 1)).
    Exec()

// 使用自定义表达式更新 (SetExpression)
// 适用于原子操作或复杂的数据库函数调用
err := mworm.UPDATE(User{}).
    SetExpression("age", "age + 1").
    SetExpression("data", "jsonb_set(data, '{last_active}', '\"2026-04-01\"')").
    Where(mworm.Eq("id", 1)).
    Exec()
```

#### 删除 (Delete)

```go
err := mworm.DELETE(User{}).
    Where(mworm.Eq("id", 1)).
    Exec()
```

### 2. 条件构造

`mworm` 提供了丰富的条件构造器，支持链式调用。

```go
// 基础条件
mworm.And("name", "age")      // name = ? AND age = ? (值从结构体取)
mworm.Eq("status", 1)         // status = 1
mworm.Gt("age", 18)           // age > 18
mworm.Lt("age", 60)           // age < 60
mworm.IN("status", 1, 2, 3)   // status IN (1, 2, 3)
mworm.NotIN("status", 4, 5)   // status NOT IN (4, 5)
mworm.Between("age", 18, 60)  // age BETWEEN 18 AND 60
mworm.Like("name")            // name LIKE '%value%'
mworm.ILike("name")           // name ILIKE '%value%' (PostgreSQL 忽略大小写)
mworm.NEqILike("name")        // name NOT ILIKE '%value%' (PostgreSQL 忽略大小写)

// 自动忽略空值 (非常适合搜索表单)
// 如果 name 或 age 为空值/零值，则该条件自动被忽略
mworm.AndAuto("name", "age")

// 组合条件
orm := mworm.SELECT(User{}).Where(
    mworm.AndAuto("name"),
    mworm.Gte("age", 18),
    mworm.NotIN("status", 0, -1),
    mworm.Between("createdAt", "2024-01-01", "2024-12-31"),
)
```

#### OR/AND 嵌套分组

用于构造复杂的组合条件，如 `(A OR B) AND (C AND D)`。

```go
// (status=1 OR status=2) AND age>=18
mworm.SELECT(User{}).Where(
    mworm.OrGroup(mworm.Eq("status", 1), mworm.Eq("status", 2)),
    mworm.Gte("age", 18),
)
// → WHERE (status=? OR status=?) AND age>=?

// (age>=18 AND age<=60)
mworm.SELECT(User{}).Where(
    mworm.AndGroup(mworm.Gte("age", 18), mworm.Lte("age", 60)),
)
// → WHERE (age>=? AND age<=?)
```

#### EXISTS / NOT EXISTS

```go
// EXISTS 子查询
mworm.SELECT(User{}).Where(
    mworm.Exists("SELECT 1 FROM orders WHERE orders.user_id = users.id"),
)

// NOT EXISTS 带参数
mworm.SELECT(User{}).Where(
    mworm.NotExists("SELECT 1 FROM blacklist WHERE user_id = users.id AND status = $1", "blocked"),
)
```

#### 子查询 (SubQuery)

```go
// WHERE id IN (子查询)
mworm.SELECT(User{}).Where(
    mworm.SubQuery("id", "IN", "SELECT user_id FROM orders WHERE amount > $1", 100),
)
// → WHERE id IN (SELECT user_id FROM orders WHERE amount > ?)
```

### 3. 聚合查询

```go
// 计数
count, err := mworm.SELECT(User{}).Where(mworm.Gt("age", 18)).Count("*")

// 求和
sum, err := mworm.SELECT(User{}).Where(mworm.Gt("age", 0)).Sum("score")

// 平均值
avg, err := mworm.SELECT(User{}).Avg("age")

// 最小值
min, err := mworm.SELECT(User{}).Min("score")

// 最大值
max, err := mworm.SELECT(User{}).Max("score")
```

### 4. 分页查询

`mworm.PAGE` 提供高效的分页查询。

```go
// page: 当前页码, pageSize: 每页数量
// excludeTags: 不需要返回的字段 json tag
result, err := mworm.PAGE(User{}, 1, 10, []string{"password"}, 
    mworm.AndAuto("name"), // 搜索条件
    mworm.Desc("createdAt"), // 排序
)

if err != nil {
    log.Fatal(err)
}
fmt.Printf("Total: %d, List: %v\n", result.Total, result.List)
```

### 5. Upsert (ON CONFLICT)

PostgreSQL 的 `INSERT ON CONFLICT` 支持。

```go
// 冲突时更新指定字段
err := mworm.INSERT(user).OnConflict("id").DoUpdate("name", "age").Exec()
// → INSERT INTO users (...) VALUES (...) ON CONFLICT (id) DO UPDATE SET name=EXCLUDED.name, age=EXCLUDED.age

// 极致简化的 Upsert (自动推断需要更新的字段)
// 它会自动将除 "id" 外的所有结构体有效字段生成为 DO UPDATE SET 语句
err := mworm.INSERT(user).Upsert("id").Exec()

// 冲突时忽略
err := mworm.INSERT(user).OnConflict("id").DoNothing().Exec()
// → INSERT INTO users (...) VALUES (...) ON CONFLICT (id) DO NOTHING
```

### 6. JOIN 查询

```go
mworm.SELECT(User{}).
    Join(mworm.LeftJoin("orders", "o").
        On(mworm.JoinOn("t.id = o.user_id")).
        Select("order_no", "amount")).
    Where(mworm.Eq("t.is_active", true)).
    Many(&results)
```

### 7. 高级特性

#### 批量操作与事务

```go
// 批量执行多个操作
err := mworm.Batch(
    mworm.INSERT(User{Name: "A"}),
    mworm.INSERT(User{Name: "B"}),
)

// 事务支持
err := mworm.BatchFunc(func(tx *sqlx.Tx) error {
    // 在此处使用 tx 执行原生 sqlx 操作
    return nil
})

```

#### 原生 SQL

```go
// 执行原生 SQL（PostgreSQL 使用 $1, $2 占位符）
mworm.ExecRawSQL("UPDATE users SET status = $1 WHERE id = $2", 1, 100)

// MySQL 使用 ? 占位符
// mworm.ExecRawSQL("UPDATE users SET status = ? WHERE id = ?", 1, 100)

// 原生 SQL 查询映射
var users []User
mworm.RawSQL("SELECT * FROM users WHERE age > 18").Many(&users)
```

#### JSONB 支持 (PostgreSQL)

```go
// 将查询结果聚合为 JSONB List
jsonStr, err := mworm.SELECT(User{}).JsonbListString()

// 将查询结果聚合为 JSONB Map
jsonMap, err := mworm.SELECT(User{}).JsonbMapString("id", "name")

// 复杂 JSONB & 数组查询条件
mworm.SELECT(User{}).Where(
    mworm.JsonbContains("data", `{"role": "admin"}`), // data @> ?
    mworm.JsonbHasKey("data", "last_login"),          // data ? ?
    mworm.ArrayAny("tags", "golang"),                 // tags = ANY(?)
    mworm.ArrayOverlap("tags", []string{"a", "b"}),    // tags && ARRAY[...]
)

// 通用 PostgreSQL 运算符
mworm.PgOp("tags", "@>", `{"go"}`)
```
```

#### CTE (Common Table Expressions)

```go
// 使用 WITH 子句构建 CTE
orm := mworm.SELECT(User{}).
    Where(mworm.Eq("status", 1)).
    With("active_users") // 定义 CTE 表名

// 生成: WITH active_users AS (SELECT * FROM users WHERE status=1) SELECT * FROM active_users
result := orm.FullSQL()
```

#### GROUP BY 分组

```go
var result []struct {
    Name  string `json:"name" db:"name"`
    Count int    `json:"count" db:"count"`
}
mworm.SELECT(User{}).
    GroupBy(mworm.Fields("name"), mworm.Raw(`count(*)`)).
    Having("count(name) > $1", 1).
    Many(&result)
```

## 调试

开启调试模式，打印生成的 SQL 语句：

```go
mworm.SELECT(User{}).Log(true).Many(&users)
// 或者全局开启
mworm.DebugMode = true
```

## 字段常量生成器

使用字段常量替代字符串，提供编译期检查和 IDE 自动补全。

### 使用示例

假设你的项目结构如下：

```
myproject/
├── go.mod           # require github.com/ccxdd/mworm
├── models/
│   ├── user.go      # 包含 User 结构体
│   └── order.go     # 包含 Order 结构体
└── main.go
```

运行生成器：

```bash
cd /path/to/myproject

# 方式1：直接运行（推荐，无需安装）
go run github.com/ccxdd/mworm/cmd/fieldgen -dir=models/ -r

# 方式2：全局安装后使用
go install github.com/ccxdd/mworm/cmd/fieldgen@latest
fieldgen -dir=models/ -r
```

或在模型文件中添加 go generate 指令：

```go
//go:generate go run github.com/ccxdd/mworm/cmd/fieldgen -src=$GOFILE

type User struct {
    // ...
}
```

然后运行 `go generate ./...` 即可自动生成。

生成结果：

```
models/
├── user.go
├── user_fields.go      ← 自动生成
├── order.go
└── order_fields.go     ← 自动生成
```

### 生成代码示例

```go
// user_fields.go（自动生成）
var UserF = struct {
    ID        string
    Name      string
    CreatedAt string
}{
    ID:        "id",
    Name:      "name",
    CreatedAt: "createdAt",
}
```

### 使用效果

```go
import "myproject/models"

// 之前（容易拼错）
mworm.SELECT(user).Where(mworm.And("createdAt", "name"))

// 之后（类型安全，IDE 自动补全）
mworm.SELECT(user).Where(mworm.And(models.UserF.CreatedAt, models.UserF.Name))
```

> **注意**：只为实现了 `TableName()` 方法的结构体生成字段常量。

## API 速查表

| 功能 | API | 示例 |
|------|-----|------|
| 插入 | `INSERT(entity).Exec()` | `INSERT(user).Exec()` |
| 查询单条 | `SELECT(entity).Where(...).One(&dest)` | `SELECT(User{}).Where(Eq("id",1)).One(&u)` |
| 查询多条 | `SELECT(entity).Where(...).Many(&dest)` | `SELECT(User{}).Where(Gt("age",18)).Many(&users)` |
| 更新 | `UPDATE(entity).Where(...).Exec()` | `UPDATE(user).WherePK().Exec()` |
| 删除 | `DELETE(entity).Where(...).Exec()` | `DELETE(User{}).WherePK().Exec()` |
| 等于 | `Eq(tag, value)` | `Eq("status", 1)` |
| 不等于 | `NEq(tag, value)` | `NEq("status", 0)` |
| 大于/大于等于 | `Gt(tag, value)` / `Gte(tag, value)` | `Gt("age", 18)` |
| 小于/小于等于 | `Lt(tag, value)` / `Lte(tag, value)` | `Lt("age", 60)` |
| IN | `IN(tag, values...)` | `IN("id", 1, 2, 3)` |
| NOT IN | `NotIN(tag, values...)` | `NotIN("status", 4, 5)` |
| BETWEEN | `Between(tag, min, max)` | `Between("age", 18, 60)` |
| LIKE | `Like(tag...)` | `Like("name")` |
| ILIKE | `ILike(tag...)` | `ILike("name")` |
| IS NULL | `Null(tag...)` | `Null("email")` |
| IS NOT NULL | `NEqNull(tag...)` | `NEqNull("email")` |
| OR 分组 | `OrGroup(cgs...)` | `OrGroup(Eq("a",1), Eq("b",2))` |
| AND 分组 | `AndGroup(cgs...)` | `AndGroup(Gte("a",1), Lte("b",10))` |
| EXISTS | `Exists(sql, args...)` | `Exists("SELECT 1 FROM ...")` |
| NOT EXISTS | `NotExists(sql, args...)` | `NotExists("SELECT 1 FROM ...")` |
| 子查询 | `SubQuery(tag, symbol, sql, args...)` | `SubQuery("id", "IN", "SELECT ...")` |
| JSONB 包含 | `JsonbContains(tag, value)` | `JsonbContains("data", "{\"a\":1}")` |
| 数组匹配 | `ArrayAny(tag, value)` | `ArrayAny("tags", "go")` |
| 聚合-计数 | `Count(column)` | `Count("*")` |
| 聚合-求和 | `Sum(column)` | `Sum("score")` |
| 聚合-平均 | `Avg(column)` | `Avg("age")` |
| 聚合-最小 | `Min(column)` | `Min("score")` |
| 聚合-最大 | `Max(column)` | `Max("score")` |
| Upsert (全自动) | `Upsert(conflictTags...)` | `INSERT(u).Upsert("id")` |
| Upsert (手动) | `OnConflict(tags...).DoUpdate(tags...)` | `INSERT(u).OnConflict("id").DoUpdate("name")` |
| 更新表达式 | `SetExpression(tag, expr)` | `SetExpression("age", "age + 1")` |
| 分页 | `PAGE(entity, page, size, excludes, cgs...)` | `PAGE(User{}, 1, 10, nil)` |
| 排序 | `Asc(tag...)` / `Desc(tag...)` | `Asc("id").Desc("createdAt")` |
| 原生 SQL | `Raw(express, args...)` | `Raw("age > $1", 18)` |
