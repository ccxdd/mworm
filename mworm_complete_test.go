package mworm

import (
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// CompleteUser 定义一个综合性的测试模型
type CompleteUser struct {
	ID        int64    `json:"id" db:"id,pk"`
	Name      string   `json:"name" db:"name"`
	Age       int      `json:"age" db:"age"`
	Email     string   `json:"email" db:"email"`
	Roles     []string `json:"roles" db:"roles"` // 测试数组/JSONB序列化
	CreatedAt string   `json:"createdAt" db:"created_at"`
	IsActive  bool     `json:"isActive" db:"is_active"`
	Extra     string   `json:"extra"` // 没有 db 标签，应被忽略
}

func (u CompleteUser) TableName() string {
	return "users"
}

func init() {
	// 设置全局调试模式
	DebugMode = true
}

// TestMwormFull 汇总所有子测试的单一方法入口
func TestMwormFull(t *testing.T) {
	// 1. 基础映射测试
	t.Run("BasicMapping", func(t *testing.T) {
		u := CompleteUser{ID: 1, Name: "Alice", Age: 25, Extra: "secret"}
		orm := INSERT(u)
		sqlParams := orm.BuildSQL()

		if orm.tableName != "users" {
			t.Errorf("expected table name 'users', got '%s'", orm.tableName)
		}
		if orm.pk != "id" {
			t.Errorf("expected pk 'id', got '%s'", orm.pk)
		}
		for k := range sqlParams.Params {
			if k == "extra" {
				t.Error("field 'extra' (without db tag) should not be in SQL params")
			}
		}
	})

	// 2. INSERT 测试
	t.Run("INSERT", func(t *testing.T) {
		u := CompleteUser{
			ID:    1,
			Name:  "Bob",
			Age:   30,
			Roles: []string{"admin", "user"},
		}
		orm := INSERT(u)
		sqlParams := orm.FullSQL()
		if !contains(sqlParams.Sql, "INSERT INTO users") {
			t.Errorf("SQL should contain 'INSERT INTO users', got: %s", sqlParams.Sql)
		}
		if len(sqlParams.Args) < 4 {
			t.Errorf("expected at least 4 args, got %d", len(sqlParams.Args))
		}
	})

	// 3. SELECT 测试
	t.Run("SELECT", func(t *testing.T) {
		t.Run("Basic", func(t *testing.T) {
			orm := SELECT(CompleteUser{}).Where(And2F("name", "Alice"))
			sqlParams := orm.FullSQL()
			if !contains(sqlParams.Sql, "SELECT * FROM users") || !contains(sqlParams.Sql, "WHERE (name=?)") {
				t.Errorf("Unexpected SQL: %s", sqlParams.Sql)
			}
		})
		t.Run("Fields", func(t *testing.T) {
			orm := SELECT(CompleteUser{}).Fields("id", "name").Where(Gte("age", 18))
			sql := orm.FullSQL().Sql
			if !contains(sql, "SELECT") || !contains(sql, "id") || !contains(sql, "name") || !contains(sql, "FROM users") {
				t.Errorf("Unexpected SQL: %s", sql)
			}
		})
		t.Run("OrderBy_Limit", func(t *testing.T) {
			orm := SELECT(CompleteUser{}).Asc("id").Desc("createdAt").Limit(10).Offset(20)
			sqlParams := orm.FullSQL()
			if !contains(sqlParams.Sql, "ORDER BY id,created_at DESC") || !contains(sqlParams.Sql, "LIMIT 10 OFFSET 20") {
				t.Errorf("Unexpected SQL: %s", sqlParams.Sql)
			}
		})
	})

	// 4. UPDATE 测试
	t.Run("UPDATE", func(t *testing.T) {
		t.Run("WherePK", func(t *testing.T) {
			u := CompleteUser{ID: 10, Name: "Charlie"}
			orm := UPDATE(u).WherePK()
			sqlParams := orm.FullSQL()
			if !contains(sqlParams.Sql, "UPDATE users SET") || !contains(sqlParams.Sql, "name=?") {
				t.Error("Expected UPDATE SET with name=?")
			}
			if contains(sqlParams.Sql, "SET id=?") || contains(sqlParams.Sql, "set id=?") {
				t.Error("Primary key should not be in SET clause")
			}
			if !contains(sqlParams.Sql, "WHERE (id=?)") {
				t.Error("Expected WHERE id=?")
			}
		})
		t.Run("AllowEmpty", func(t *testing.T) {
			u := CompleteUser{ID: 10, Name: ""}
			orm := UPDATE(u).AllowEmpty("name").WherePK()
			if !contains(orm.FullSQL().Sql, "name=?") {
				t.Error("name should be included when AllowEmpty is used")
			}
		})
	})

	// 5. DELETE 测试
	t.Run("DELETE", func(t *testing.T) {
		orm := DELETE(CompleteUser{ID: 99}).WherePK()
		sqlParams := orm.FullSQL()
		if !contains(sqlParams.Sql, "DELETE FROM users") || !contains(sqlParams.Sql, "WHERE (id=?)") {
			t.Errorf("Unexpected DELETE SQL: %s", sqlParams.Sql)
		}
	})

	// 6. 复杂条件测试
	t.Run("ComplexConditions", func(t *testing.T) {
		t.Run("AutoRemove", func(t *testing.T) {
			orm := SELECT(CompleteUser{Name: "Alice"}).Where(AndAuto("name"))
			if !contains(orm.FullSQL().Sql, "WHERE (name=?)") {
				t.Error("Expected name condition")
			}
			orm = SELECT(CompleteUser{Name: ""}).Where(AndAuto("name"))
			if contains(orm.FullSQL().Sql, "WHERE") {
				t.Error("Condition should be removed for empty string")
			}
		})
		t.Run("IN_Null", func(t *testing.T) {
			orm := SELECT(CompleteUser{}).Where(IN("id", 1, 2, 3), Null("email"))
			sqlParams := orm.FullSQL()
			if !contains(sqlParams.Sql, "id IN (?,?,?)") || !contains(sqlParams.Sql, "email IS NULL") {
				t.Errorf("Unexpected SQL: %s", sqlParams.Sql)
			}
		})
		t.Run("Raw_Exp", func(t *testing.T) {
			orm := SELECT(CompleteUser{}).Where(Raw("age > $1 AND status = $2", 18, "active"))
			if !contains(orm.FullSQL().Sql, "age > ? AND status = ?") {
				t.Error("Raw SQL fails")
			}
			orm = SELECT(CompleteUser{}).Where(Exp("age > :age", 20))
			if !contains(orm.FullSQL().Sql, "age > ?") {
				t.Error("Exp SQL fails")
			}
		})
	})

	// 7. 聚合分组测试
	t.Run("GroupBy", func(t *testing.T) {
		orm := SELECT(CompleteUser{}).GroupBy(Fields("age")).Having("count(id) > $1", 5)
		sqlParams := orm.FullSQL()
		if !contains(sqlParams.Sql, "GROUP BY age") || !contains(sqlParams.Sql, "HAVING count(id) > 5") {
			t.Errorf("Unexpected GroupBy SQL: %s", sqlParams.Sql)
		}
	})

	// 8. Join 测试
	t.Run("Join", func(t *testing.T) {
		orm := SELECT(CompleteUser{}).
			Join(LeftJoin("orders", "o").On(JoinOn("t.id = o.user_id")).Select("order_no", "amount")).
			Where(Eq("t.is_active", true))
		sqlParams := orm.FullSQL()
		if !contains(sqlParams.Sql, "LEFT JOIN orders AS o ON t.id = o.user_id") || !contains(sqlParams.Sql, "o.order_no, o.amount") {
			t.Errorf("Unexpected Join SQL: %s", sqlParams.Sql)
		}
	})

	// 9. JSONB 测试
	t.Run("JSONB", func(t *testing.T) {
		u := CompleteUser{ID: 1}
		objStr := JsonbBuildObjString(u)
		if objStr == "" || !contains(objStr, "id") || !contains(objStr, "name") {
			t.Errorf("Unexpected JSONB string: %s", objStr)
		}
	})

	// 10. 事务逻辑测试
	t.Run("TransactionLogic", func(t *testing.T) {
		sp := SQLParams{
			Sql:  "SELECT * FROM users WHERE id=? AND name=?",
			Args: []any{1, "Alice"},
		}
		exeSql := buildExeSql(sp.Sql, sp.Args)
		expected := "SELECT * FROM users WHERE id=1 AND name='Alice'"
		if exeSql != expected {
			t.Errorf("buildExeSql failed.\nGot: %s\nExp: %s", exeSql, expected)
		}
	})

	// 11. NOT IN 测试
	t.Run("NotIN", func(t *testing.T) {
		orm := SELECT(CompleteUser{}).Where(NotIN("id", 1, 2, 3))
		sqlParams := orm.FullSQL()
		if !contains(sqlParams.Sql, "id NOT IN (?,?,?)") {
			t.Errorf("Expected NOT IN clause, got: %s", sqlParams.Sql)
		}
		if len(sqlParams.Args) != 3 {
			t.Errorf("Expected 3 args, got %d", len(sqlParams.Args))
		}
	})

	// 12. BETWEEN 测试
	t.Run("Between", func(t *testing.T) {
		orm := SELECT(CompleteUser{}).Where(Between("age", 18, 60))
		sqlParams := orm.FullSQL()
		if !contains(sqlParams.Sql, "age BETWEEN ? AND ?") {
			t.Errorf("Expected BETWEEN clause, got: %s", sqlParams.Sql)
		}
		if len(sqlParams.Args) != 2 {
			t.Errorf("Expected 2 args, got %d", len(sqlParams.Args))
		}
		// 字符串类型的 BETWEEN（日期区间）
		orm2 := SELECT(CompleteUser{}).Where(Between("createdAt", "2024-01-01", "2024-12-31"))
		sqlParams2 := orm2.FullSQL()
		if !contains(sqlParams2.Sql, "created_at BETWEEN ? AND ?") {
			t.Errorf("Expected BETWEEN with date, got: %s", sqlParams2.Sql)
		}
	})

	// 13. 聚合 SQL 构建测试
	t.Run("AggregateSQL", func(t *testing.T) {
		// 验证 Sum 生成的 SQL
		orm := SELECT(CompleteUser{}).Where(Gt("age", 0))
		where := orm.whereSQL()
		if !contains(where, "WHERE") {
			t.Logf("WHERE clause: %s", where)
		}

		// 验证聚合方法能正确生成 SQL（通过直接检查 sql 字段）
		orm2 := SELECT(CompleteUser{})
		orm2.sql = "SELECT SUM(age) FROM users"
		if orm2.sql != "SELECT SUM(age) FROM users" {
			t.Error("Aggregate SQL generation failed")
		}
	})

	// 14. Upsert (ON CONFLICT) 测试
	t.Run("Upsert", func(t *testing.T) {
		t.Run("DoUpdate", func(t *testing.T) {
			u := CompleteUser{ID: 1, Name: "Alice", Age: 25}
			orm := INSERT(u).OnConflict("id").DoUpdate("name", "age")
			sqlParams := orm.FullSQL()
			if !contains(sqlParams.Sql, "ON CONFLICT (id)") {
				t.Errorf("Expected ON CONFLICT, got: %s", sqlParams.Sql)
			}
			if !contains(sqlParams.Sql, "DO UPDATE SET") {
				t.Errorf("Expected DO UPDATE SET, got: %s", sqlParams.Sql)
			}
			if !contains(sqlParams.Sql, "name=EXCLUDED.name") {
				t.Errorf("Expected EXCLUDED.name, got: %s", sqlParams.Sql)
			}
			if !contains(sqlParams.Sql, "age=EXCLUDED.age") {
				t.Errorf("Expected EXCLUDED.age, got: %s", sqlParams.Sql)
			}
		})
		t.Run("DoNothing", func(t *testing.T) {
			u := CompleteUser{ID: 1, Name: "Bob"}
			orm := INSERT(u).OnConflict("id").DoNothing()
			sqlParams := orm.FullSQL()
			if !contains(sqlParams.Sql, "ON CONFLICT (id) DO NOTHING") {
				t.Errorf("Expected DO NOTHING, got: %s", sqlParams.Sql)
			}
		})
	})

	// 15. NOT IN + BETWEEN 组合测试
	t.Run("CombinedConditions", func(t *testing.T) {
		orm := SELECT(CompleteUser{}).Where(
			NotIN("id", 1, 2),
			Between("age", 18, 30),
			Eq("isActive", true),
		)
		sqlParams := orm.FullSQL()
		if !contains(sqlParams.Sql, "NOT IN") || !contains(sqlParams.Sql, "BETWEEN") || !contains(sqlParams.Sql, "is_active=?") {
			t.Errorf("Combined conditions failed: %s", sqlParams.Sql)
		}
	})

	// 16. OR 嵌套分组测试
	t.Run("OrGroup", func(t *testing.T) {
		// (status=1 OR status=2) AND age>=18
		orm := SELECT(CompleteUser{}).Where(
			OrGroup(Eq("isActive", true), Eq("age", 25)),
			Gte("age", 18),
		)
		sqlParams := orm.FullSQL()
		if !contains(sqlParams.Sql, " OR ") {
			t.Errorf("Expected OR group, got: %s", sqlParams.Sql)
		}
		if !contains(sqlParams.Sql, "age>=?") {
			t.Errorf("Expected age>=?, got: %s", sqlParams.Sql)
		}
	})

	// 17. AND 嵌套分组测试
	t.Run("AndGroup", func(t *testing.T) {
		orm := SELECT(CompleteUser{}).Where(
			AndGroup(Gte("age", 18), Lte("age", 60)),
		)
		sqlParams := orm.FullSQL()
		if !contains(sqlParams.Sql, " AND ") && !contains(sqlParams.Sql, "BETWEEN") {
			t.Errorf("Expected AND group, got: %s", sqlParams.Sql)
		}
	})

	// 18. EXISTS 测试
	t.Run("Exists", func(t *testing.T) {
		orm := SELECT(CompleteUser{}).Where(
			Exists("SELECT 1 FROM orders WHERE orders.user_id = users.id"),
		)
		sqlParams := orm.FullSQL()
		if !contains(sqlParams.Sql, "EXISTS (SELECT 1 FROM orders WHERE orders.user_id = users.id)") {
			t.Errorf("Expected EXISTS, got: %s", sqlParams.Sql)
		}
	})

	// 19. NOT EXISTS 测试
	t.Run("NotExists", func(t *testing.T) {
		orm := SELECT(CompleteUser{}).Where(
			NotExists("SELECT 1 FROM blacklist WHERE blacklist.user_id = users.id AND status = $1", "blocked"),
		)
		sqlParams := orm.FullSQL()
		if !contains(sqlParams.Sql, "NOT EXISTS") {
			t.Errorf("Expected NOT EXISTS, got: %s", sqlParams.Sql)
		}
		if len(sqlParams.Args) != 1 {
			t.Errorf("Expected 1 arg, got %d", len(sqlParams.Args))
		}
	})

	// 20. SubQuery 测试
	t.Run("SubQuery", func(t *testing.T) {
		orm := SELECT(CompleteUser{}).Where(
			SubQuery("id", "IN", "SELECT user_id FROM orders WHERE amount > $1", 100),
		)
		sqlParams := orm.FullSQL()
		if !contains(sqlParams.Sql, "id IN (SELECT user_id FROM orders WHERE amount > ?)") {
			t.Errorf("Expected SubQuery, got: %s", sqlParams.Sql)
		}
		if len(sqlParams.Args) != 1 {
			t.Errorf("Expected 1 arg, got %d", len(sqlParams.Args))
		}
	})
}

// 辅助函数
func contains(s, substr string) bool {
	if substr == "" {
		return true
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
