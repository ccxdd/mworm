# mworm
go postgresql orm

# mworm 方法示例

以下是 mworm 项目常用方法的使用示例：

## 1. 分页查询
```go
// 定义实体结构体
 type User struct {
     ID   int    `json:"id" db:"id"`
     Name string `json:"name" db:"name"`
 }

// 分页查询，排除 json tag 字段
result, err := mworm.PAGE(User{}, 1, 10, []string{"password"}, mworm.And("name"))
if err != nil {
    // 错误处理
}
fmt.Println(result.List)
```

## 2. 条件构造
```go
// 构造 AND 条件
cond := mworm.And("name", "age")
// 构造 OR 条件
cond := mworm.Or("status")
// 构造 IN 条件
cond := mworm.IN("id", 1, 2, 3)
// 构造 LIKE 条件
cond := mworm.Like("name")
// 构造 NULL 条件
cond := mworm.IsNull("deleted_at")
// 构造比较条件
cond := mworm.Gt("age", 18)  // 大于
cond := mworm.Gte("age", 18) // 大于等于
cond := mworm.Lt("age", 30)  // 小于
cond := mworm.Lte("age", 30) // 小于等于
cond := mworm.Eq("status", 1) // 等于
```

## 3. 单条/多条查询
```go
// 单条查询
orm := mworm.SELECT(User{})
var user User
err := orm.Where(mworm.And("id")).One(&user)

// 多条查询
var users []User
err := orm.Where(mworm.And("status")).Many(&users)

// 组合多个条件查询
err := orm.Where(
    mworm.And("status"),
    mworm.Gt("age", 18),
    mworm.Like("name"),
).Many(&users)
```

## 4. 插入/更新/删除
```go
// 插入
orm := mworm.INSERT(User{ID: 1, Name: "Tom"})
err := orm.Exec()

// 更新
orm := mworm.UPDATE(User{ID: 1, Name: "Jerry"})
err := orm.Where(mworm.And("id")).Exec()

// 更新指定字段
orm := mworm.UPDATE(User{})
err := orm.SetField("name", "Tom").
    SetField("age", 20).
    Where(mworm.And("id")).
    Exec()

// 删除
orm := mworm.DELETE(User{})
err := orm.Where(mworm.And("id")).Exec()
```

## 5. 排序与分页
```go
// 排序
orm := mworm.SELECT(User{}).
    Asc("name").     // 按 name 升序
    Desc("id")       // 按 id 降序

// 分页
orm := mworm.SELECT(User{}).
    Limit(10).       // 限制返回 10 条
    Offset(0)        // 从第 0 条开始

// 组合使用
var users []User
err := orm.SELECT(User{}).
    Where(mworm.And("status")).
    Asc("name").
    Limit(10).
    Offset(0).
    Many(&users)
```

## 6. 原生 SQL 查询
```go
// 直接执行 SQL
orm := mworm.RawSQL("SELECT * FROM users WHERE status = 'active'")
var users []User
err := orm.Many(&users)

// 带参数的原生 SQL
params := map[string]interface{}{
    "status": "active",
    "age": 18,
}
orm := mworm.RawNamedSQL("SELECT * FROM users WHERE status = :status AND age > :age", params)
err := orm.Many(&users)
```

## 7. 关联查询
```go
// WITH 查询
orm := mworm.SELECT(User{}).
    With("orders").                    // 关联 orders 表
    WithAsc("created_at")             // orders 表按 created_at 升序

// JsonbMap 查询
var result map[string]User
err := orm.JsonbMap(&result, "id", "name")

// JsonbList 查询
var users []User
err := orm.JsonbList(&users)
```

## 8. 高级功能
```go
// 批量操作
err := mworm.Batch(
    mworm.INSERT(User{Name: "Tom"}),
    mworm.UPDATE(User{}).Where(mworm.And("id")),
    mworm.DELETE(User{}).Where(mworm.And("status")),
)

// 事务操作
err := mworm.BatchFunc(func(tx *sqlx.Tx) {
    // 在事务中执行操作
})

// 调试 SQL
orm := mworm.SELECT(User{}).Log(true)  // 打印 SQL 语句
```

## 9. 字段过滤
```go
// 指定查询字段
orm := mworm.SELECT(User{}).
    Fields("id", "name")    // 只查询 id 和 name 字段

// 排除字段
orm := mworm.SELECT(User{}).
    ExcludeFields("password", "salt")   // 排除 password 和 salt 字段
```

## 初始化配置
```go
// 连接数据库
db, err := sqlx.Connect("postgres", "postgres://user:password@localhost:5432/dbname?sslmode=disable")
if err != nil {
    log.Fatal(err)
}
err = mworm.BindDB(db)
if err != nil {
    log.Fatal(err)
}
```

更多用法请参考源码注释和接口定义。
