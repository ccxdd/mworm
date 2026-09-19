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

### 2. 定义模型与 Tag 规范

在 `mworm` 中，模型定义必须同时提供 `db` 和 `json` tag。

#### 核心规范：调用时使用 json tag 还是 db tag？
* **`db` tag**：仅用于映射数据库底层的物理列名（以及标识主键 `pk`、排除更新 `eu` 等）。
* **`json` tag**：除了用于 JSON 序列化，**在调用 `mworm` 的所有链式查询、条件构造器（如 `mworm.Eq`、`mworm.And`、`mworm.Or` 等）、排序（`Asc`、`Desc`）、字段设置（`SetField`、`SetExpression`）时，传入的字段名参数必须使用 `json` tag，绝对不能使用 `db` tag。**

`mworm` 底层会自动解析并将传入的 `json` tag 转换为对应的 `db` tag 以生成正确的 SQL 语句。

**单主键模型：**
```go
type User struct {
    ID        int64     `json:"id" db:"id,pk"`          // pk 标识主键
    Name      string    `json:"name" db:"name"`
    Age       int       `json:"age" db:"age"`
}
```

**复合主键模型：**
`mworm` 完美支持多字段复合主键，在 `WherePK`、`Upsert` 等操作中会自动识别所有标记为 `pk` 的字段。
```go
type Favorite struct {
    UserId   int64  `json:"userId" db:"user_id,pk"`     // 查询条件传参需用 json tag: "userId"
    Code     string `json:"code" db:"code,pk"`          // 多个 pk 标记构成的复合主键
    Name     string `json:"name" db:"name"`
}

func (f Favorite) TableName() string {
    return "favorite"
}
```

## 常用功能

### 1. CRUD 操作

#### 插入 (Insert)

```go
user := User{Name: "Tom", Age: 18}
// 基础插入
err := mworm.INSERT(user).Exec()

// PostgreSQL 支持 RETURNING
var id int64
err := mworm.INSERT(user).RETURNING(&id, nil, "id")
```

#### 查询 (Select)

```go
// 单条查询 (注意: 传入的字段名使用 json tag "id")
var user User
err := mworm.SELECT(User{}).
    Where(mworm.Eq("id", 1)).
    One(&user)

// 多条查询 (注意: 传入的字段名使用 json tag "age")
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

// 使用 WherePK() 自动根据主键更新 (支持单/复合主键)
// 1. 自动根据结构体中的 pk 字段生成 WHERE 条件
// 2. 自动在 SET 子句中排除主键字段，防止主键被修改
err := mworm.UPDATE(fav).WherePK().Exec()

// 更新指定字段 (注意: 传入的字段名使用 json tag "name", "status")
err := mworm.UPDATE(User{}).
    SetField("name", "Jerry").
    SetField("status", 2).
    Where(mworm.Eq("id", 1)).
    Exec()

// 使用自定义表达式更新 (SetExpression) (注意: 传入的字段名使用 json tag "age", "data")
err := mworm.UPDATE(User{}).
    SetExpression("age", "age + 1").
    SetExpression("data", "jsonb_set(data, '{last_active}', '\"2026-04-01\"')").
    Where(mworm.Eq("id", 1)).
    Exec()

// 更新空值或零值 (AllowEmpty) (默认情况下，结构体空值字段在 UPDATE 时会被忽略)
// 传入需要允许更新空值的字段的 json tag
err := mworm.UPDATE(User{Name: ""}).
    AllowEmpty("name").
    Where(mworm.Eq("id", 1)).
    Exec()
```

#### 删除 (Delete)

```go
// 使用指定条件删除 (注意: 传入的字段名使用 json tag "id")
err := mworm.DELETE(User{}).
    Where(mworm.Eq("id", 1)).
    Exec()

// 使用主键删除
err := mworm.DELETE(fav).WherePK().Exec()
```

### 2. 条件构造

`mworm` 提供了丰富的条件构造器，支持链式调用。**所有条件构造器的第一个字段名参数均需使用 json tag。**

```go
// 基础条件
mworm.And("name", "age")      // name = ? AND age = ? (使用 json tag "name", "age")
mworm.Eq("status", 1)         // status = 1 (使用 json tag "status")
mworm.Gt("age", 18)           // age > 18 (使用 json tag "age")
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
    mworm.Between("createdAt", "2024-01-01", "2024-12-31"), // 使用 json tag "createdAt"
)
```

#### OR/AND 嵌套分组

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
// WHERE id IN (子查询) (注意: 传入的字段名使用 json tag "id")
mworm.SELECT(User{}).Where(
    mworm.SubQuery("id", "IN", "SELECT user_id FROM orders WHERE amount > $1", 100),
)
// → WHERE id IN (SELECT user_id FROM orders WHERE amount > ?)
```

### 3. 聚合查询

```go
// 计数
count, err := mworm.SELECT(User{}).Where(mworm.Gt("age", 18)).Count("*")

// 求和 (使用 json tag "score")
sum, err := mworm.SELECT(User{}).Where(mworm.Gt("age", 0)).Sum("score")

// 平均值 (使用 json tag "age")
avg, err := mworm.SELECT(User{}).Avg("age")

// 最小值 (使用 json tag "score")
min, err := mworm.SELECT(User{}).Min("score")

// 最大值 (使用 json tag "score")
max, err := mworm.SELECT(User{}).Max("score")
```

### 4. 分页查询

`mworm.PAGE` 提供高效的分页查询。

```go
// page: 当前页码, pageSize: 每页数量
// excludeTags: 不需要返回的字段 json tag 数组
result, err := mworm.PAGE(User{}, 1, 10, []string{"password"}, 
    mworm.AndAuto("name"), // 搜索条件
    mworm.Desc("createdAt"), // 排序 (使用 json tag "createdAt")
)

if err != nil {
    log.Fatal(err)
}
fmt.Printf("Total: %d, List: %v\n", result.Total, result.List)
```

### 5. Upsert (ON CONFLICT)

PostgreSQL 的 `INSERT ON CONFLICT` 支持。

```go
// 冲突时更新指定字段 (使用 json tag "name", "age")
err := mworm.INSERT(user).OnConflict("id").DoUpdate("name", "age").Exec()
// → INSERT INTO users (...) VALUES (...) ON CONFLICT (id) DO UPDATE SET name=EXCLUDED.name, age=EXCLUDED.age

// 极致简化的 Upsert (自动推断需要更新的字段，冲突字段使用 json tag "id")
err := mworm.INSERT(user).Upsert("id").Exec()

// 冲突时忽略
err := mworm.INSERT(user).OnConflict("id").DoNothing().Exec()
// → INSERT INTO users (...) VALUES (...) ON CONFLICT (id) DO NOTHING
```

### 6. 批量插入（BulkInsert）

`BulkInsert` 将同类型 slice 的 **N 行数据合并为一条 SQL** 发送给数据库，相比在循环中逐条调用 `INSERT`，数据库往返从 N 次降为 **1 次**，写入耗时从秒级降至毫秒级。

#### 基础用法

```go
// 准备 slice，元素类型须实现 ORMInterface（即有 TableName() 方法）
rows := []MoneyFlowMin{
    {TradeDate: "20260814", TradeMin: "0930", ThemeSymbol: "A", MainNetAmount: 100},
    {TradeDate: "20260814", TradeMin: "0930", ThemeSymbol: "B", MainNetAmount: -50},
    // ... 数百行
}

// 纯 INSERT（有冲突会报错）
err := mworm.BulkInsert(rows).Exec()

// Upsert：冲突时自动更新非冲突列（推荐）
// 冲突字段使用 json tag
err := mworm.BulkInsert(rows).
    Upsert("tradedate", "trademin", "theme_symbol").
    Exec()

// 冲突时忽略（不报错，不更新）
err := mworm.BulkInsert(rows).
    DoNothing("tradedate", "trademin", "theme_symbol").
    Exec()

// 手动指定冲突列和需要更新的列
err := mworm.BulkInsert(rows).
    OnConflict("tradedate", "trademin", "theme_symbol").
    DoUpdate("main_net_amount", "main_buy_amount").
    Exec()
```

#### 排除某些列

```go
// 排除 updated_at 列不写入（使用 json tag）
err := mworm.BulkInsert(rows).
    ExcludeFields("updatedAt").
    Upsert("id").
    Exec()
```

#### 与 Batch / BatchArray 的区别

| | `Batch` / `BatchArray` | `BulkInsert` |
|---|---|---|
| SQL 语句数量 | N 条（每行一条 INSERT） | **1 条** |
| 事务 | 1 个事务 | 1 个事务（内部自动管理） |
| 数据库往返 | N 次 | **1 次** |
| 适用场景 | 多种不同 ORM 操作混合（INSERT + UPDATE 等） | 同类型 slice 批量写入 |
| 错误粒度 | 可逐行处理 | 全批成功或全批回滚 |

#### 注意事项

> **零值字段行为**：`BulkInsert` 对所有行**全列写入**（含零值），与单行 `INSERT` 会跳过零值字段的行为不同。  
> 若某列可能为零且不希望写入，请用 `ExcludeFields("jsonTag")` 显式排除。

生成的 SQL 示例（3 行，9 列）：

```sql
INSERT INTO moneyflow_min
  (tradedate, trademin, theme_symbol, theme_name, level, p_symbol,
   main_net_amount, main_buy_amount, main_sell_amount)
VALUES
  ($1, $2, $3, $4, $5, $6, $7, $8, $9),
  ($10, $11, $12, $13, $14, $15, $16, $17, $18),
  ($19, $20, $21, $22, $23, $24, $25, $26, $27)
ON CONFLICT (tradedate, trademin, theme_symbol) DO UPDATE SET
  theme_name = EXCLUDED.theme_name,
  main_net_amount = EXCLUDED.main_net_amount,
  ...
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

// 复杂 JSONB & 数组查询条件 (使用 json tag "data", "tags")
mworm.SELECT(User{}).Where(
    mworm.JsonbContains("data", `{"role": "admin"}`), // data @> ?
    mworm.JsonbHasKey("data", "last_login"),          // data ? ?
    mworm.ArrayAny("tags", "golang"),                 // tags = ANY(?)
    mworm.ArrayOverlap("tags", []string{"a", "b"}),    // tags && ARRAY[...]
)

// 通用 PostgreSQL 运算符
mworm.PgOp("tags", "@>", `{"go"}`)
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

#### 动态表名 (Table)

当模型没有定义 `TableName()` 方法，或者表名需要动态变化时（如分表场景）：

```go
// 显式指定表名进行查询
var users []User
err := mworm.Table("users_2026").
    Select(User{}).
    Where(mworm.Gt("age", 18)).
    Many(&users)
```

#### 条件分支 (If)

用于在链式调用中根据业务逻辑条件动态拼接查询条件，避免打断链式调用：

```go
// 根据 limitVar 的值动态决定是否添加 Limit 限制
err := mworm.SELECT(User{}).
    Where(mworm.Eq("status", 1)).
    If(func(o *mworm.OrmModel) {
        if limitVar > 0 {
            o.Limit(limitVar)
        }
    }).
    Many(&users)
```

#### JSONB 直接映射反序列化 (JsonbMap / JsonbList)

直接将 JSONB 聚合查询结果反序列化映射为 Go 结构体/切片，底层集成高性能 `sonic` 库：

```go
// JsonbMap: 将聚合结果直接映射为 map
var idMap = make(map[string]string)
err := mworm.SELECT(User{}).JsonbMap(&idMap, "nickname", "phone")

// JsonbList: 将列表直接反序列化为切片
var userList []User
err := mworm.SELECT(User{}).JsonbList(&userList)
```


## 调试

开启调试模式，打印生成的 SQL 语句：

```go
mworm.SELECT(User{}).Log(true).Many(&users)
// 或者全局开启
mworm.DebugMode = true
```

## 字段常量生成器

使用字段常量替代字符串，提供编译期检查和 IDE 自动补全。生成的常量映射的即是字段的 **json tag**。

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

// 方式1：直接运行（推荐，无需安装）
go run github.com/ccxdd/mworm/cmd/fieldgen -dir=models/ -r

// 方式2：全局安装后使用
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
    ID:        "id",            // 映射 json tag "id"
    Name:      "name",          // 映射 json tag "name"
    CreatedAt: "createdAt",     // 映射 json tag "createdAt"
}
```

### 使用效果

```go
import "myproject/models"

// 之前（容易拼错）
mworm.SELECT(user).Where(mworm.And("createdAt", "name"))

// 之后（类型安全，IDE 自动补全，且保证使用的是 json tag）
mworm.SELECT(user).Where(mworm.And(models.UserF.CreatedAt, models.UserF.Name))
```

> **注意**：只为实现了 `TableName()` 方法的结构体生成字段常量。

## API 速查表

| 功能 | API | 示例 |
|------|-----|------|
| 插入 | `INSERT(entity).Exec()` | `INSERT(user).Exec()` |
| **批量插入** | **`BulkInsert(slice).Exec()`** | **`BulkInsert(rows).Upsert("id").Exec()`** |
| 查询单条 | `SELECT(entity).Where(...).One(&dest)` | `SELECT(User{}).Where(Eq("id",1)).One(&u)` |
| 查询多条 | `SELECT(entity).Where(...).Many(&dest)` | `SELECT(User{}).Where(Gt("age",18)).Many(&users)` |
| 更新 | `UPDATE(entity).Where(...).Exec()` | `UPDATE(user).WherePK().Exec()` |
| 删除 | `DELETE(entity).Where(...).Exec()` | `DELETE(user).WherePK().Exec()` |
| 主键条件 | `WherePK()` | 自动构建单/复合主键 WHERE 条件，且在 UPDATE 时自动排除主键字段 |
| 等于 | `Eq(tag, value)` | `Eq("status", 1)` **(必须使用 json tag)** |
| 不等于 | `NEq(tag, value)` | `NEq("status", 0)` **(必须使用 json tag)** |
| 大于/大于等于 | `Gt(tag, value)` / `Gte(tag, value)` | `Gt("age", 18)` **(必须使用 json tag)** |
| 小于/小于等于 | `Lt(tag, value)` / `Lte(tag, value)` | `Lt("age", 60)` **(必须使用 json tag)** |
| IN | `IN(tag, values...)` | `IN("id", 1, 2, 3)` **(必须使用 json tag)** |
| NOT IN | `NotIN(tag, values...)` | `NotIN("status", 4, 5)` **(必须使用 json tag)** |
| BETWEEN | `Between(tag, min, max)` | `Between("age", 18, 60)` **(必须使用 json tag)** |
| LIKE | `Like(tag...)` | `Like("name")` **(必须使用 json tag)** |
| ILIKE | `ILike(tag...)` | `ILike("name")` **(必须使用 json tag)** |
| IS NULL | `Null(tag...)` | `Null("email")` **(必须使用 json tag)** |
| IS NOT NULL | `NEqNull(tag...)` | `NEqNull("email")` **(必须使用 json tag)** |
| OR 分组 | `OrGroup(cgs...)` | `OrGroup(Eq("a",1), Eq("b",2))` |
| AND 分组 | `AndGroup(cgs...)` | `AndGroup(Gte("a",1), Lte("b",10))` |
| EXISTS | `Exists(sql, args...)` | `Exists("SELECT 1 FROM ...")` |
| NOT EXISTS | `NotExists(sql, args...)` | `NotExists("SELECT 1 FROM ...")` |
| 子查询 | `SubQuery(tag, symbol, sql, args...)` | `SubQuery("id", "IN", "SELECT ...")` **(必须使用 json tag)** |
| JSONB 包含 | `JsonbContains(tag, value)` | `JsonbContains("data", "{\"a\":1}")` **(必须使用 json tag)** |
| 数组匹配 | `ArrayAny(tag, value)` | `ArrayAny("tags", "go")` **(必须使用 json tag)** |
| 聚合-计数 | `Count(column)` | `Count("*")` |
| 聚合-求和 | `Sum(column)` | `Sum("score")` **(必须使用 json tag)** |
| 聚合-平均 | `Avg(column)` | `Avg("age")` **(必须使用 json tag)** |
| 聚合-最小 | `Min(column)` | `Min("score")` **(必须使用 json tag)** |
| 聚合-最大 | `Max(column)` | `Max("score")` **(必须使用 json tag)** |
| Upsert (全自动) | `Upsert(conflictTags...)` | `INSERT(u).Upsert("id")` **(必须使用 json tag)** |
| Upsert (手动) | `OnConflict(tags...).DoUpdate(tags...)` | `INSERT(u).OnConflict("id").DoUpdate("name")` **(必须使用 json tag)** |
| 更新表达式 | `SetExpression(tag, expr)` | `SetExpression("age", "age + 1")` **(必须使用 json tag)** |
| 更新空值 | `AllowEmpty(tags...)` | `AllowEmpty("name")` **(必须使用 json tag)** |
| 动态表名 | `Table(tableName)` | `Table("users_2026")` |
| 条件分支 | `If(func(o *OrmModel))` | `.If(func(o *mworm.OrmModel){ o.Limit(10) })` |
| JSONB 映射(Map) | `JsonbMap(&dest, keys...)` | `SELECT(User{}).JsonbMap(&m, "id", "name")` |
| JSONB 映射(List) | `JsonbList(&dest)` | `SELECT(User{}).JsonbList(&list)` |
| 分页 | `PAGE(entity, page, size, excludes, cgs...)` | `PAGE(User{}, 1, 10, nil)` |
| **链式分页** | **`Paginate(orm, page, size, &dest)`** | **`Paginate(SELECT(User{}).Where(...).Desc("id"), 1, 10, &list)`** |
| 排序 | `Asc(tag...)` / `Desc(tag...)` | `Asc("id").Desc("createdAt")` **(必须使用 json tag)** |
| **上下文** | **`WithContext(ctx)`** | **`SELECT(User{}).WithContext(ctx).One(&u)`** |
| **事务绑定** | **`Tx(tx)`** | **`INSERT(order).Tx(tx).Exec()`** |
| **托管事务** | **`Transaction(ctx, fn)`** | **`mworm.Transaction(ctx, func(tx *sqlx.Tx) error { ... })`** |
| **独立客户端** | **`New(db)`** | **`client := mworm.New(slaveDB); client.SELECT(...)`** |
| **忽略 0 行** | **`IgnoreZeroRows()`** | **`UPDATE(user).WherePK().IgnoreZeroRows().Exec()`** |
| 原生 SQL | `Raw(express, args...)` | `Raw("age > $1", 18)` |

---

## 进阶高级特性

### 1. 上下文与超时熔断 (WithContext)

支持传入 `context.Context`，在请求超时或客户端取消连接时，立即中断数据库执行，防止慢查询占满连接池：

```go
ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
defer cancel()

var user User
err := mworm.SELECT(User{}).
    WithContext(ctx).
    Where(mworm.Eq("id", 123)).
    One(&user)
```

### 2. 链式事务与托管闭包 (Transactions)

支持两种事务使用方式：

#### 方式 A：托管式事务闭包（推荐）
自动管理 `Begin`、`Commit` 并在发生 `error` 或 `panic` 时自动 `Rollback`：

```go
err := mworm.Transaction(ctx, func(tx *sqlx.Tx) error {
    // 事务内直接链式调用 .Tx(tx)
    if err := mworm.INSERT(order).Tx(tx).Exec(); err != nil {
        return err
    }
    return mworm.UPDATE(account).Tx(tx).Where(mworm.Eq("id", 1)).Exec()
})
```

#### 方式 B：手动绑定外部事务
```go
tx, _ := mworm.SqlxDB.Beginx()
defer tx.Rollback()

err := mworm.BulkInsert(items).Tx(tx).Exec()
if err != nil {
    return err
}
tx.Commit()
```

### 3. 多数据库实例与读写分离 (Client)

除全局单例 `mworm.SqlxDB` 外，支持独立客户端模式：

```go
// 创建独立客户端实例
slaveClient := mworm.New(slaveDB)

// 针对从库查询
var users []User
err := slaveClient.SELECT(User{}).Where(mworm.Eq("status", 1)).Many(&users)
```

### 4. 优雅链式分页 (Paginate)

原生链式分页能够天然继承 `Where`、`Desc`、`Asc`、`WithContext`、`Tx` 等所有设置：

```go
var userList []User
res, err := mworm.Paginate(
    mworm.SELECT(User{}).
        Where(mworm.Eq("status", 1)).
        Desc("createdAt").
        WithContext(ctx),
    1, 20, &userList,
)
if err != nil {
    return err
}

fmt.Printf("总数: %d, 总页数: %d, 是否有下一页: %v\n", res.Total, res.TotalPage, res.HasNext())
```

### 5. 影响行数控制与哨兵错误

对于 UPDATE/DELETE 操作，如果影响行数为 0，`mworm` 默认返回 `mworm.ErrNoRowsAffected`。可通过 `IgnoreZeroRows()` 豁免：

```go
// 捕获特定错误
if errors.Is(err, mworm.ErrNoRowsAffected) {
    // 没有任何行被修改
}

// 或主动忽略 0 行影响
err := mworm.UPDATE(u).WherePK().IgnoreZeroRows().Exec()
```

### 6. 慢查询自动监控与告警

配置全局慢查询阈值，当查询耗时超过阈值时将自动打印 Warn 告警日志：

```go
// 设置慢查询告警阈值为 200 毫秒 (设为 0 则不启用)
mworm.SlowQueryThreshold = 200 * time.Millisecond
```

