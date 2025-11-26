package mworm

import (
	"fmt"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
)

type User struct {
	ID   int64  `json:"id" db:"id,pk"`
	Name string `json:"name" db:"name"`
}

func (u User) TableName() string {
	return "users"
}

func TestJoinSQL(t *testing.T) {
	// Mock DB to avoid panic in Table() -> SqlxDB.DriverName()
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()
	sqlxDB := sqlx.NewDb(db, "sqlmock")
	BindDB(sqlxDB)

	// 模拟场景：查询用户及其订单信息
	// 主表：users (别名 t)
	// 关联表：orders (别名 o)

	// 1. 基础 Left Join
	// SQL 预期: SELECT t.*, o.order_no, o.amount FROM users t LEFT JOIN orders AS o ON t.id = o.user_id WHERE (t.status=1)
	orm := SELECT(User{}).
		Join(
			LeftJoin("orders", "o").
				On(JoinOn("t.id = o.user_id")).
				Select("order_no", "amount"),
		).
		Where(Eq("t.status", 1))

	sqlParams := orm.BuildSQL()
	fmt.Println("Test 1 SQL:", sqlParams.Sql)

	expectedSQL1 := "SELECT t.*, o.order_no, o.amount FROM users t LEFT JOIN orders AS o ON t.id = o.user_id WHERE t.status=1"
	if sqlParams.Sql != expectedSQL1 {
		t.Errorf("Test 1 Failed.\nExpected: %s\nGot:      %s", expectedSQL1, sqlParams.Sql)
	}

	// 2. 多表 Join + 复杂条件
	// SQL 预期: SELECT t.*, o.order_no, p.product_name FROM users t
	//           INNER JOIN orders AS o ON t.id = o.user_id AND o.status = 1
	//           LEFT JOIN products AS p ON o.product_id = p.id
	//           WHERE (t.age > 18)
	orm2 := SELECT(User{}).
		Join(
			InnerJoin("orders", "o").
				On(JoinOn("t.id = o.user_id"), Eq("o.status", 1)).
				Select("order_no"),
		).
		Join(
			LeftJoin("products", "p").
				On(JoinOn("o.product_id = p.id")).
				Select("product_name"),
		).
		Where(Gt("t.age", 18))

	sqlParams2 := orm2.BuildSQL()
	fmt.Println("Test 2 SQL:", sqlParams2.Sql)

	// 注意：Map 遍历顺序不确定，Where 条件顺序可能变化，这里主要验证 Join 部分
	// 简单验证关键片段
	if !strings.Contains(sqlParams2.Sql, "INNER JOIN orders AS o ON t.id = o.user_id AND o.status = 1") {
		t.Errorf("Test 2 Failed: Inner Join clause missing or incorrect")
	}
	if !strings.Contains(sqlParams2.Sql, "LEFT JOIN products AS p ON o.product_id = p.id") {
		t.Errorf("Test 2 Failed: Left Join clause missing or incorrect")
	}
}
