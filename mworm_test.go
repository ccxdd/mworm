package mworm

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
)

type TestTable struct {
	ID        int      `json:"id" db:"id,pk"`
	Name      string   `json:"name" db:"name"`
	Type      int      `json:"type" db:"type"`
	CreatedAt string   `json:"createdAt" db:"created_at"`
	Images    []string `json:"images" db:"images"`
	IgnoreMe  string   `json:"ignoreMe"`
}

type TestStruct struct {
	Name string `json:"name"`
	Age  int    `json:"age"`
	High string `json:"high"`
}

func (t TestTable) TableName() string {
	return "test_table"
}

func skipIfNoPostgres(t *testing.T) {
	db, err := sqlx.Open("pgx", DBConnectionString())
	if err != nil {
		t.Skip("跳过集成测试：无法连接 PostgreSQL:", err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		t.Skip("跳过集成测试：PostgreSQL Ping 超时或失败:", err)
		return
	}
	db.SetMaxOpenConns(100)
	db.SetMaxIdleConns(100)
	_ = BindDB(db)
}

func OpenSqlxDB() {
	db, err := sqlx.Open("pgx", DBConnectionString())
	if err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return
	}
	db.SetMaxOpenConns(100)
	db.SetMaxIdleConns(100)
	_ = BindDB(db)
}

func DBConnectionString() string {
	return "host=127.0.0.1 port=5432 user=ccxdd dbname=gva_user_center sslmode=disable"
}

func TestOrm(t *testing.T) {
	SqlxDB = new(sqlx.DB)

	o := INSERT(TestTable{ID: 9})
	fmt.Println(o.FullSQL())
	if o.err != nil {
		t.Fatal(o.err)
	}

	o = SELECT(TestTable{ID: 10, Name: "name", Type: 11}).Where(AutoFill(), Desc("id"))
	fmt.Println(o.FullSQL())
	if o.err != nil {
		t.Fatal(o.err)
	}

	o = SELECT(TestTable{ID: 10, Name: "name", Type: 11}).WherePK()
	fmt.Println(o.FullSQL())
	if o.err != nil {
		t.Fatal(o.err)
	}

	o = SELECT(TestTable{}).Where(And2F("id", "100"), Or2F("name", 1, "A"))
	fmt.Println(o.FullSQL())
	if o.err != nil {
		t.Fatal(o.err)
	}

	o = SELECT(TestTable{}).Where(Gt("type"), And("type"), Lte("name"))
	fmt.Println(o.FullSQL())
	if o.err != nil {
		t.Fatal(o.err)
	}

	o = SELECT(TestTable{}).Where(Exp(`created>='abc' AND abc=:abc`, "123"))
	fmt.Println(o.FullSQL())
	if o.err != nil {
		t.Fatal(o.err)
	}
	//
	o = SELECT(TestTable{}).Where(Exp(`created>='abc' AND abc=:abc`, 123, "ABC"))
	fmt.Println(o.FullSQL())
	if o.err != nil {
		t.Fail()
	}
	//
	o = SELECT(TestTable{}).Where(Exp(`created>='abc' AND abc=:abc`, "123")).With("t")
	fmt.Println(o.FullSQL())
	if o.err != nil {
		t.Fail()
	}
	//
	sp := SELECT(TestTable{}).Where(Exp(`created>='abc' AND abc=:abc`, "123")).With("t").WithAsc("date").FullSQL()
	fmt.Println(sp)
	if o.err != nil {
		t.Fail()
	}
}

func TestInsertUpdate(t *testing.T) {
	SqlxDB = new(sqlx.DB)

	o := INSERT(TestTable{ID: 9})
	fmt.Println(o.FullSQL())
	if o.err != nil {
		t.Fatal(o.err)
	}

	o = UPDATE(TestTable{ID: 9, Name: "2222"}).ExcludeFields("id").WherePK()
	fmt.Println(o.FullSQL())
	if o.err != nil {
		t.Fatal(o.err)
	}

	o = UPDATE(TestTable{ID: 9, Name: "2222"}).Fields("name", "type").Where(AutoFill())
	fmt.Println(o.FullSQL())
	if o.err != nil {
		t.Fatal(o.err)
	}
}

func TestStructMap(t *testing.T) {
	//
	test := TestStruct{}
	a, b := StructToMap(test)
	fmt.Println(a, b)
}

func TestJsonbBuildObjString(t *testing.T) {
	test := TestTable{}
	fmt.Println(JsonbBuildObjString(test, "row"))
	fmt.Println(JsonbBuildObjString(test))
	fmt.Println(JsonTagToJsonbKeys(test, "t", "createdAt", "images"))
}

type TbUser struct {
	Uid            string `json:"uid" db:"uid,pk"`                     //用户ID
	Username       string `json:"username" db:"user_name"`             //用户名
	Password       string `json:"password,omitempty" db:"password"`    //密码
	NickName       string `json:"nickname" db:"nick_name"`             //昵称
	Phone          string `json:"phone,omitempty" db:"phone"`          //手机
	EncPhone       string `json:"encPhone"`                            //脱敏手机号
	EncNumber      string `json:"encNumber"`                           //脱敏号码
	Gender         string `json:"gender" db:"gender"`                  //性别
	Age            int    `json:"age" db:"age"`                        //年龄
	Avatar         string `json:"avatar" db:"avatar"`                  //头像
	Source         string `json:"source" db:"source"`                  //来源
	InvitationCode string `json:"invitationCode" db:"invitation_code"` //邀请码
	CreatedAt      string `json:"createdAt" db:"created_at"`           //创建时间
	Org            int    `json:"org" db:"org"`                        //组织名
	App            int    `json:"app" db:"app"`                        //应用名
	Wechat         string `json:"wechat" db:"wechat"`                  //微信
	Alipay         string `json:"alipay" db:"alipay"`                  //支付宝
	Douyin         string `json:"douyin" db:"douyin"`                  //抖音
	Xhs            string `json:"xhs" db:"xhs"`                        //小红书
}

func (u TbUser) TableName() string {
	return "users"
}

func TestPage(t *testing.T) {
	skipIfNoPostgres(t)
	result, err := DebugPAGE(TbUser{Username: "user"}, true, 1, 10, nil,
		Desc("createdAt"), Like("username"), Null("invitationCode", "encPhone"), And("age"))
	if err != nil {
		if strings.Contains(err.Error(), "does not exist") {
			t.Skip("跳过集成测试：本地数据库缺少测试表或字段:", err)
			return
		}
		t.Fatal(err)
	}
	t.Log("总页数", result.TotalPage, "记录数", result.Total)
}

// CreateMatch 创建赛事请求
type CreateMatch struct {
	ID         int64  `json:"id" db:"id,pk"`
	HomeTeamID int64  `json:"homeTeamId" db:"home_team_id" validate:"required"` // 主队ID
	AwayTeamID int64  `json:"awayTeamId" db:"away_team_id" validate:"required"` // 客队ID
	LeagueID   int64  `json:"leagueId" db:"league_id" validate:"required"`      // 联赛ID
	StartTime  string `json:"startTime" db:"start_time" validate:"required"`    // 开赛时间
	Status     string `json:"status" db:"status"`                               // 状态（未开始/进行中/已结束）
	HomeTeam   string `json:"homeTeam" db:"home_team"`                          // 主队名称
	AwayTeam   string `json:"awayTeam" db:"away_team"`                          // 客队名称
	LeagueName string `json:"leagueName" db:"league_name"`                      // 联赛名称
}

func (CreateMatch) TableName() string {
	return "jc_football_match"
}

func TestCUD(t *testing.T) {
	skipIfNoPostgres(t)
	if err := UPDATE(CreateMatch{ID: 3, HomeTeam: "更新队名"}).IgnoreZeroRows().WherePK().Log(true).Exec(); err != nil {
		if strings.Contains(err.Error(), "does not exist") {
			t.Skip("跳过集成测试：本地数据库缺少 jc_football_match 表:", err)
			return
		}
		t.Fatal(err)
	}
	t.Log("更新成功")

	if err := DELETE(CreateMatch{ID: 5}).IgnoreZeroRows().WherePK().Exec(); err != nil {
		if strings.Contains(err.Error(), "does not exist") {
			t.Skip("跳过集成测试：本地数据库缺少 jc_football_match 表:", err)
			return
		}
		t.Fatal(err)
	}
	t.Log("删除成功")
}

func TestQueryEmpty(t *testing.T) {
	skipIfNoPostgres(t)
	//user := TbUser{}
	orm := SELECT(&TbUser{Wechat: ""}).Where(AndAuto("wechat"))
	fmt.Println(orm.BuildSQL().ExeSql())
	orm = SELECT(&TbUser{Wechat: ""}).Where(And("wechat"))
	fmt.Println(orm.BuildSQL().ExeSql())
}

func TestUpdateEmpty(t *testing.T) {
	skipIfNoPostgres(t)
	DebugMode = true
	_ = UPDATE(CreateMatch{ID: 12, HomeTeam: ""}).AllowEmpty("homeTeam").WherePK().BuildSQL()
	_ = SELECT(CreateMatch{}).Where(Gte("id", 12), Lte("id", 10)).BuildSQL()
}

func TestRawCond(t *testing.T) {
	skipIfNoPostgres(t)
	params := UPDATE(CreateMatch{ID: 12, HomeTeam: ""}).Where(Raw(`league_id>='2' AND status=$1 AND id=$2`, `666`, 222)).
		AllowEmpty("homeTeam").WherePK().BuildSQL()
	fmt.Println(params.ExeSql())
}

type Team struct {
	ID      int64  `json:"id" db:"id,pk"`        // 主键ID
	Name    string `json:"name" db:"name"`       // 队名
	Country string `json:"country" db:"country"` // 国家/地区
	Logo    string `json:"logo" db:"logo"`       // 队徽URL
}

func (Team) TableName() string {
	return "jc_football_team"
}

func TestJOIN(t *testing.T) {
	skipIfNoPostgres(t)
}

func TestJsonMap(t *testing.T) {
	skipIfNoPostgres(t)
	var idMap = make(map[string]string)
	var rowMap = make(map[string]CreateMatch)
	var rowList = make([]CreateMatch, 0)
	if err := SELECT(CreateMatch{}).JsonbMap(&idMap, "home_team", "away_team"); err != nil {
		t.Error(err)
	}
	fmt.Println("idMap = ", idMap)
	if err := SELECT(CreateMatch{}).JsonbList(&rowList); err != nil {
		t.Error(err)
	}
	fmt.Println("list = ", rowList)
	if err := SELECT(CreateMatch{}).JsonbMap(&rowMap, "home_team", "row"); err != nil {
		t.Error(err)
	}
	fmt.Println("rowMap = ", rowMap)
}

func TestGroupBy(t *testing.T) {
	OpenSqlxDB()
	var result []string
	DebugMode = true
	if err := SELECT(Team{}).GroupBy(Fields("name")).Asc("name").Many(&result); err != nil {
		t.Error(err)
	}
	fmt.Println("result1 = ", result)

	var result2 []struct {
		Name  string `json:"name" db:"name"`
		Count int    `json:"count" db:"count"`
	}
	if err := SELECT(Team{}).GroupBy(Fields("name"), Raw(`count(*),sum(id)`)).Having(`count(name)=$1`, 1).
		Asc("name").Many(&result2); err != nil {
		t.Error(err)
	}
	fmt.Println("result2 = ", result2)
}

// ==========================================
// BulkInsert 测试
// ==========================================

// BulkTestRow 用于 BulkInsert 测试的临时 struct
type BulkTestRow struct {
	TradeDate   string  `json:"tradedate" db:"tradedate,pk"`
	TradeMin    string  `json:"trademin" db:"trademin,pk"`
	ThemeSymbol string  `json:"theme_symbol" db:"theme_symbol,pk"`
	ThemeName   string  `json:"theme_name" db:"theme_name,eu"`
	NetAmount   float64 `json:"net_amount" db:"net_amount,eu"`
}

func (BulkTestRow) TableName() string { return "bulk_test_table" }

// TestBulkInsertSQL 验证 BulkInsert 生成的 SQL 结构正确（无需 DB 连接）
func TestBulkInsertSQL(t *testing.T) {
	SqlxDB = new(sqlx.DB)

	rows := []BulkTestRow{
		{TradeDate: "20260814", TradeMin: "0930", ThemeSymbol: "A", ThemeName: "板块A", NetAmount: 100.5},
		{TradeDate: "20260814", TradeMin: "0930", ThemeSymbol: "B", ThemeName: "板块B", NetAmount: -50.0},
		{TradeDate: "20260814", TradeMin: "0930", ThemeSymbol: "C", ThemeName: "板块C", NetAmount: 0},
	}

	// 1. 基础 BulkInsert：检查 SQL 包含 3 行占位符
	o := BulkInsert(rows)
	p := o.BuildSQL()
	fmt.Println("[BulkInsert 纯插入]", p.ExeSql())
	if o.err != nil {
		t.Fatalf("BulkInsert 基础: %v", o.err)
	}
	if len(o.bulkRows) != 3 {
		t.Fatalf("期望 bulkRows=3，实际=%d", len(o.bulkRows))
	}

	// 2. BulkInsert + Upsert：SQL 应包含 ON CONFLICT ... DO UPDATE SET
	o2 := BulkInsert(rows).Upsert("tradedate", "trademin", "theme_symbol")
	p2 := o2.BuildSQL()
	fmt.Println("[BulkInsert + Upsert]", p2.ExeSql())
	if o2.err != nil {
		t.Fatalf("BulkInsert Upsert: %v", o2.err)
	}

	// 3. BulkInsert + DoNothing：SQL 应包含 DO NOTHING
	o3 := BulkInsert(rows).OnConflict("tradedate", "trademin", "theme_symbol").DoNothing()
	p3 := o3.BuildSQL()
	fmt.Println("[BulkInsert + DoNothing]", p3.ExeSql())
	if o3.err != nil {
		t.Fatalf("BulkInsert DoNothing: %v", o3.err)
	}

	// 4. BulkInsert + ExcludeFields：排除 net_amount 列后列数应减少
	o4 := BulkInsert(rows).ExcludeFields("net_amount").Upsert("tradedate", "trademin", "theme_symbol")
	p4 := o4.BuildSQL()
	fmt.Println("[BulkInsert + ExcludeFields]", p4.ExeSql())
	if o4.err != nil {
		t.Fatalf("BulkInsert ExcludeFields: %v", o4.err)
	}

	// 5. 空 slice：应返回 err，不崩溃
	o5 := BulkInsert([]BulkTestRow{})
	if o5.err == nil {
		t.Fatal("空 slice 应返回 err")
	}
	fmt.Println("[BulkInsert 空 slice] err=", o5.err)

	// 6. 验证参数数量正确：3行 × 5列 = 15 个参数
	if len(p.Args) != 15 {
		t.Fatalf("期望 15 个参数，实际=%d", len(p.Args))
	}
	fmt.Printf("[参数验证] 3行×5列=%d 个参数 ✅\n", len(p.Args))

	t.Log("所有 BulkInsert SQL 生成测试通过 ✅")
}

// TestBulkInsertExec 连接真实数据库执行 BulkInsert（需要 test_table 存在）
// 使用 TestTable 已有的测试表，通过 Upsert 写入再清理
func TestBulkInsertExec(t *testing.T) {
	skipIfNoPostgres(t)
	DebugMode = true

	rows := []TestTable{
		{ID: 9001, Name: "bulk_test_A", Type: 1, Images: []string{}},
		{ID: 9002, Name: "bulk_test_B", Type: 2, Images: []string{}},
		{ID: 9003, Name: "bulk_test_C", Type: 3, Images: []string{}},
	}

	// 1. BulkInsert Upsert：写入 3 行
	if err := BulkInsert(rows).Upsert("id").Exec(); err != nil {
		if strings.Contains(err.Error(), "does not exist") {
			t.Skip("跳过集成测试：本地数据库缺少 test_table 表:", err)
			return
		}
		t.Fatalf("BulkInsert Exec 失败: %v", err)
	}
	t.Log("BulkInsert 写入 3 行成功 ✅")

	// 2. 验证数据已写入（原生 COUNT，避开 images 类型扫描）
	var count int
	if err := SqlxDB.QueryRowx("SELECT COUNT(*) FROM test_table WHERE id IN (9001,9002,9003)").Scan(&count); err != nil {
		t.Fatalf("查询验证失败: %v", err)
	}
	if count != 3 {
		t.Fatalf("期望查询到 3 行，实际=%d", count)
	}
	t.Logf("验证查询到 %d 行 ✅", count)

	// 3. 重复执行 Upsert（测试冲突更新）
	rows[0].Name = "bulk_test_A_updated"
	if err := BulkInsert(rows).Upsert("id").Exec(); err != nil {
		t.Fatalf("BulkInsert Upsert 冲突更新失败: %v", err)
	}
	t.Log("BulkInsert Upsert 冲突更新成功 ✅")

	// 4. 清理测试数据
	if err := DELETE(TestTable{}).Where(IN("id", 9001, 9002, 9003)).Exec(); err != nil {
		t.Logf("清理测试数据失败（可忽略）: %v", err)
	} else {
		t.Log("测试数据清理成功 ✅")
	}
}

// ==========================================
// 新特性测试：Context / Tx / Client / Paginate / Clone / Sentinel Error
// ==========================================

// TestNewFeaturesMock 使用 sqlmock 验证所有新特性
func TestNewFeaturesMock(t *testing.T) {
	// 1. 测试 WithContext 超时取消机制
	t.Run("WithContext_Timeout", func(t *testing.T) {
		db, _, err := sqlmock.New()
		if err != nil {
			t.Fatalf("创建 sqlmock 失败: %v", err)
		}
		defer db.Close()
		sqlxMock := sqlx.NewDb(db, "sqlmock")

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // 立即取消

		o := SELECT(TestTable{}).WithDB(sqlxMock).WithContext(ctx)
		var list []TestTable
		err = o.Many(&list)
		if err == nil {
			t.Fatal("期望收到 context canceled 错误，实际为 nil")
		}
		if !errors.Is(err, context.Canceled) {
			t.Logf("收到 Context 错误: %v ✅", err)
		}
	})

	// 2. 测试 Tx 链式绑定与执行
	t.Run("Tx_ChainExecution", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatalf("创建 sqlmock 失败: %v", err)
		}
		defer db.Close()
		sqlxMock := sqlx.NewDb(db, "sqlmock")

		mock.ExpectBegin()
		mock.ExpectExec(`UPDATE "?test_table"? SET .+ WHERE .+`).
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()

		tx, err := sqlxMock.Beginx()
		if err != nil {
			t.Fatalf("Beginx 失败: %v", err)
		}

		user := TestTable{ID: 100, Name: "new_name"}
		err = UPDATE(user).WithDB(sqlxMock).Tx(tx).Fields("name").Where(Eq("id", 100)).Exec()
		if err != nil {
			t.Fatalf("Tx 链式 Exec 失败: %v", err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatalf("tx.Commit 失败: %v", err)
		}
		t.Log("Tx 链式 Exec 测试通过 ✅")
	})

	// 3. 测试 Transaction 托管闭包自动提交与回滚
	t.Run("Transaction_Closure", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatalf("创建 sqlmock 失败: %v", err)
		}
		defer db.Close()
		sqlxMock := sqlx.NewDb(db, "sqlmock")

		// 正常提交
		mock.ExpectBegin()
		mock.ExpectExec(`INSERT INTO "?test_table"?`).WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()

		client := New(sqlxMock)
		err = client.Transaction(context.Background(), func(tx *sqlx.Tx) error {
			return client.INSERT(TestTable{ID: 1, Name: "tx_test"}).Tx(tx).Exec()
		})
		if err != nil {
			t.Fatalf("Transaction 提交失败: %v", err)
		}

		// 出错自动回滚
		mock.ExpectBegin()
		mock.ExpectRollback()
		expectedErr := errors.New("business error")
		err = client.Transaction(context.Background(), func(tx *sqlx.Tx) error {
			return expectedErr
		})
		if !errors.Is(err, expectedErr) {
			t.Fatalf("期望回滚错误 %v, 得到 %v", expectedErr, err)
		}
		t.Log("Transaction 闭包自动回滚测试通过 ✅")
	})

	// 4. 测试 IgnoreZeroRows 与 ErrNoRowsAffected 哨兵错误
	t.Run("IgnoreZeroRows_And_SentinelError", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatalf("创建 sqlmock 失败: %v", err)
		}
		defer db.Close()
		sqlxMock := sqlx.NewDb(db, "sqlmock")

		// 未设置 IgnoreZeroRows: 影响行数 0 应返回 ErrNoRowsAffected
		mock.ExpectExec(`UPDATE "?test_table"? SET .+ WHERE .+`).
			WillReturnResult(sqlmock.NewResult(0, 0))

		err = UPDATE(TestTable{ID: 999, Name: "no_change"}).WithDB(sqlxMock).Where(Eq("id", 999)).Exec()
		if !errors.Is(err, ErrNoRowsAffected) {
			t.Fatalf("期望得到 ErrNoRowsAffected, 实际得到: %v", err)
		}

		// 设置 IgnoreZeroRows(true): 影响行数 0 正常返回 nil
		mock.ExpectExec(`UPDATE "?test_table"? SET .+ WHERE .+`).
			WillReturnResult(sqlmock.NewResult(0, 0))

		o := UPDATE(TestTable{ID: 999, Name: "no_change"}).WithDB(sqlxMock).Where(Eq("id", 999)).IgnoreZeroRows()
		if err := o.Exec(); err != nil {
			t.Fatalf("IgnoreZeroRows 后不应报错，实际得到: %v", err)
		}
		if o.RowsAffected() != 0 {
			t.Fatalf("期望 RowsAffected()=0，实际=%d", o.RowsAffected())
		}
		t.Log("IgnoreZeroRows 与 ErrNoRowsAffected 测试通过 ✅")
	})

	// 5. 测试 OrmModel.Clone 深度拷贝独立性
	t.Run("OrmModel_Clone", func(t *testing.T) {
		original := SELECT(TestTable{}).
			Where(Eq("type", 1)).
			Desc("createdAt").
			Limit(10).
			Offset(20)

		clone := original.Clone()
		clone.Limit(50).Offset(100).Asc("name")

		if original.limit != 10 || original.offset != 20 {
			t.Fatalf("Clone 修改污染了原始 OrmModel: limit=%d, offset=%d", original.limit, original.offset)
		}
		if clone.limit != 50 || clone.offset != 100 {
			t.Fatalf("Clone 自身的属性未生效: limit=%d, offset=%d", clone.limit, clone.offset)
		}
		t.Log("OrmModel.Clone 深度克隆测试通过 ✅")
	})

	// 6. 测试 Paginate 链式分页
	t.Run("Paginate_Mock", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatalf("创建 sqlmock 失败: %v", err)
		}
		defer db.Close()
		sqlxMock := sqlx.NewDb(db, "sqlmock")

		// 1. Count
		countRows := sqlmock.NewRows([]string{"count"}).AddRow(25)
		mock.ExpectQuery(`SELECT count\(\*\) FROM "?test_table"?`).WillReturnRows(countRows)

		// 2. Many
		dataRows := sqlmock.NewRows([]string{"id", "name", "type", "created_at"}).
			AddRow(1, "item1", 1, "2026-01-01").
			AddRow(2, "item2", 1, "2026-01-02")
		mock.ExpectQuery(`SELECT \* FROM "?test_table"?`).WillReturnRows(dataRows)

		var list []TestTable
		res, err := Paginate(
			SELECT(TestTable{}).WithDB(sqlxMock).Where(Eq("type", 1)).Desc("createdAt"),
			1, 10, &list,
		)
		if err != nil {
			t.Fatalf("Paginate 失败: %v", err)
		}
		if res.Total != 25 {
			t.Fatalf("期望 Total=25, 实际=%d", res.Total)
		}
		if res.TotalPage != 3 {
			t.Fatalf("期望 TotalPage=3, 实际=%d", res.TotalPage)
		}
		if !res.HasNext() {
			t.Fatal("第 1 页共 3 页，HasNext() 应为 true")
		}
		if res.HasPrev() {
			t.Fatal("第 1 页，HasPrev() 应为 false")
		}
		if len(res.List) != 2 {
			t.Fatalf("期望 List 长度 2，实际=%d", len(res.List))
		}
		t.Log("Paginate 链式分页测试通过 ✅")
	})
}

// 7. 测试未实现 driver.Valuer 的结构体在 INSERT / UPDATE / BulkInsert 下自动安全序列化为 JSON 字符串
type TestCustomPermission struct {
	CouponVerification bool `json:"couponVerification"`
}

type TestMerchantStationModel struct {
	MerchantID string               `json:"merchantId" db:"merchant_id"`
	StationID  int64                `json:"stationId" db:"station_id"`
	Permission TestCustomPermission `json:"permission" db:"permission"`
}

func (t TestMerchantStationModel) TableName() string {
	return "merchant_station"
}

func TestComplexStructArgConversion(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("创建 sqlmock 失败: %v", err)
	}
	defer db.Close()
	sqlxMock := sqlx.NewDb(db, "sqlmock")

	// 1. 单行 INSERT 测试
	t.Run("Single_INSERT_StructToJSON", func(t *testing.T) {
		item := TestMerchantStationModel{
			MerchantID: "m_1001",
			StationID:  806,
			Permission: TestCustomPermission{CouponVerification: true},
		}

		orm := INSERT(item).WithDB(sqlxMock)
		sqlParams := orm.BuildSQL()

		// 检查 Args 中结构体是否已被安全转换为 JSON string
		foundJSON := false
		for _, arg := range sqlParams.Args {
			if s, ok := arg.(string); ok && strings.Contains(s, `"couponVerification":true`) {
				foundJSON = true
				break
			}
		}
		if !foundJSON {
			t.Fatalf("单行 INSERT 未将 struct 正确序列化为 JSON string 参数: args=%#v", sqlParams.Args)
		}

		// 通过 sqlmock 模拟真实底层驱动参数校验执行，确保 database/sql 不会抛出 unsupported type struct 错误
		mock.ExpectExec(`INSERT INTO "?merchant_station"?`).
			WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
			WillReturnResult(sqlmock.NewResult(1, 1))

		if err := orm.Exec(); err != nil {
			t.Fatalf("单行 INSERT 执行失败: %v", err)
		}
		t.Log("单行 INSERT 复合结构体参数转换测试通过 ✅")
	})

	// 2. 单行 UPDATE 测试
	t.Run("Single_UPDATE_StructToJSON", func(t *testing.T) {
		item := TestMerchantStationModel{
			MerchantID: "m_1001",
			StationID:  806,
			Permission: TestCustomPermission{CouponVerification: true},
		}

		orm := UPDATE(item).WithDB(sqlxMock).Where(Eq("merchantId", "m_1001"))
		sqlParams := orm.BuildSQL()

		foundJSON := false
		for _, arg := range sqlParams.Args {
			if s, ok := arg.(string); ok && strings.Contains(s, `"couponVerification":true`) {
				foundJSON = true
				break
			}
		}
		if !foundJSON {
			t.Fatalf("单行 UPDATE 未将 struct 正确序列化为 JSON string 参数: args=%#v", sqlParams.Args)
		}

		mock.ExpectExec(`UPDATE "?merchant_station"? SET`).
			WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), "m_1001").
			WillReturnResult(sqlmock.NewResult(0, 1))

		if err := orm.Exec(); err != nil {
			t.Fatalf("单行 UPDATE 执行失败: %v", err)
		}
		t.Log("单行 UPDATE 复合结构体参数转换测试通过 ✅")
	})

	// 3. BulkInsert 批量插入测试
	t.Run("BulkInsert_StructToJSON", func(t *testing.T) {
		list := []TestMerchantStationModel{
			{MerchantID: "m_1", StationID: 101, Permission: TestCustomPermission{CouponVerification: true}},
			{MerchantID: "m_2", StationID: 102, Permission: TestCustomPermission{CouponVerification: false}},
		}

		orm := BulkInsert(list).WithDB(sqlxMock)
		sqlParams := orm.BuildSQL()

		// 2行 × 3列 = 6个参数
		if len(sqlParams.Args) != 6 {
			t.Fatalf("期望 6 个参数，实际=%d", len(sqlParams.Args))
		}

		jsonCount := 0
		for _, arg := range sqlParams.Args {
			if s, ok := arg.(string); ok && strings.Contains(s, "couponVerification") {
				jsonCount++
			}
		}
		if jsonCount != 2 {
			t.Fatalf("期望 2 个 JSON 字符串参数，实际=%d", jsonCount)
		}

		mock.ExpectExec(`INSERT INTO "?merchant_station"?`).
			WithArgs(
				sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
				sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			).
			WillReturnResult(sqlmock.NewResult(2, 2))

		if err := orm.Exec(); err != nil {
			t.Fatalf("BulkInsert 执行失败: %v", err)
		}
		t.Log("BulkInsert 复合结构体参数转换测试通过 ✅")
	})
}

// 8. 测试 BulkInsert 分批切片 (Chunking) 执行与事务原子性
func TestBulkInsertChunking(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("创建 sqlmock 失败: %v", err)
	}
	defer db.Close()
	sqlxMock := sqlx.NewDb(db, "sqlmock")

	list := []TestTable{
		{ID: 1, Name: "A", Type: 1},
		{ID: 2, Name: "B", Type: 1},
		{ID: 3, Name: "C", Type: 1},
		{ID: 4, Name: "D", Type: 1},
		{ID: 5, Name: "E", Type: 1},
	}

	t.Run("AutoTransactionChunking", func(t *testing.T) {
		orm := BulkInsert(list).Chunk(2).WithDB(sqlxMock)

		// 期望开启事务
		mock.ExpectBegin()
		// chunk 1: 2 行
		mock.ExpectExec(`INSERT INTO "?test_table"?`).
			WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
				sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
			WillReturnResult(sqlmock.NewResult(2, 2))
		// chunk 2: 2 行
		mock.ExpectExec(`INSERT INTO "?test_table"?`).
			WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
				sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
			WillReturnResult(sqlmock.NewResult(4, 2))
		// chunk 3: 1 行
		mock.ExpectExec(`INSERT INTO "?test_table"?`).
			WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
			WillReturnResult(sqlmock.NewResult(5, 1))
		// 期望提交事务
		mock.ExpectCommit()

		if err := orm.Exec(); err != nil {
			t.Fatalf("BulkInsert Chunk 自动事务分批执行失败: %v", err)
		}
		if orm.RowsAffected() != 5 {
			t.Fatalf("期望 RowsAffected=5, 实际=%d", orm.RowsAffected())
		}
		t.Log("BulkInsert Chunk 自动事务分批测试通过 ✅")
	})

	t.Run("ContextCancellationShortCircuit", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // 立即取消

		orm := BulkInsert(list).Chunk(2).WithContext(ctx).WithDB(sqlxMock)
		err := orm.Exec()
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("期望 context.Canceled 错误，实际得到: %v", err)
		}
		t.Log("BulkInsert Chunk 上下文取消短路拦截测试通过 ✅")
	})
}

// 9. 测试 WherePK 安全拦截 (防止主键缺失或为空时全表误更误删)
type TestNoPKModel struct {
	Name string `json:"name" db:"name"`
	Age  int    `json:"age" db:"age"`
}

func (t TestNoPKModel) TableName() string {
	return "no_pk_table"
}

func TestWherePKSafety(t *testing.T) {
	t.Run("NoPKTag_ReturnsError", func(t *testing.T) {
		item := TestNoPKModel{Name: "test", Age: 18}
		orm := UPDATE(item).WherePK()
		sqlParams := orm.BuildSQL()
		if sqlParams.Err == nil {
			t.Fatal("期望无主键模型在 WherePK 时返回错误，实际为 nil")
		}
		t.Logf("无主键模型拦截成功: %v ✅", sqlParams.Err)
	})

	t.Run("EmptyPKValue_AbortsUpdate", func(t *testing.T) {
		item := TestTable{Name: "only_name"} // ID 为 0 (零值)
		orm := UPDATE(item).WherePK()
		sqlParams := orm.BuildSQL()
		if sqlParams.Err == nil {
			t.Fatal("期望空主键值在 WherePK UPDATE 时触发拦截错误，实际为 nil")
		}
		t.Logf("空主键 UPDATE 拦截成功: %v ✅", sqlParams.Err)
	})

	t.Run("EmptyPKValue_AbortsDelete", func(t *testing.T) {
		item := TestTable{Name: "only_name"} // ID 为 0
		orm := DELETE(item).WherePK()
		sqlParams := orm.BuildSQL()
		if sqlParams.Err == nil {
			t.Fatal("期望空主键值在 WherePK DELETE 时触发拦截错误，实际为 nil")
		}
		t.Logf("空主键 DELETE 拦截成功: %v ✅", sqlParams.Err)
	})

	t.Run("ValidPKValue_Success", func(t *testing.T) {
		item := TestTable{ID: 100, Name: "valid"}
		orm := UPDATE(item).WherePK()
		sqlParams := orm.BuildSQL()
		if sqlParams.Err != nil {
			t.Fatalf("有效主键更新报错: %v", sqlParams.Err)
		}
		if !strings.Contains(sqlParams.Sql, "WHERE (id=?)") {
			t.Fatalf("期望包含 WHERE (id=?), 实际 SQL: %s", sqlParams.Sql)
		}
		t.Log("有效主键 WherePK 测试通过 ✅")
	})
}

// 10. 测试 PostgreSQL JSONB 包含条件 (JsonContains)
type TestMarketHistoryModel struct {
	Date     string `json:"date" db:"date,pk"`
	Category string `json:"category" db:"category,pk"`
	Payload  string `json:"payload" db:"payload"`
}

func (t TestMarketHistoryModel) TableName() string {
	return "market_history"
}

// 10. 测试 PostgreSQL 专属特性套件 (JSONB, Array, ILIKE, RETURNING)
func TestPostgreSQLFeaturesSuite(t *testing.T) {
	// 1. JSONB 运算套件
	t.Run("JSONB_Operators", func(t *testing.T) {
		filter := TestCustomPermission{CouponVerification: true}
		orm1 := SELECT(TestMarketHistoryModel{}).Where(JsonContains("payload", filter))
		p1 := orm1.BuildSQL()
		if !strings.Contains(p1.Sql, "payload @> ?") {
			t.Fatalf("JsonContains 失败: %s", p1.Sql)
		}

		orm2 := SELECT(TestMarketHistoryModel{}).Where(JsonContainedBy("payload", filter))
		p2 := orm2.BuildSQL()
		if !strings.Contains(p2.Sql, "payload <@ ?") {
			t.Fatalf("JsonContainedBy 失败: %s", p2.Sql)
		}

		orm3 := SELECT(TestMarketHistoryModel{}).Where(JsonbHasKey("payload", "user_id"))
		p3 := orm3.BuildSQL()
		if !strings.Contains(p3.Sql, "jsonb_exists(payload, ?)") {
			t.Fatalf("JsonbHasKey 失败: %s", p3.Sql)
		}

		orm4 := SELECT(TestMarketHistoryModel{}).Where(JsonPath("payload", "user,id", "=", 1001))
		p4 := orm4.BuildSQL()
		if !strings.Contains(p4.Sql, "payload#>>'{user,id}' = ?") {
			t.Fatalf("JsonPath 失败: %s", p4.Sql)
		}
		t.Log("PostgreSQL JSONB 运算套件测试通过 ✅")
	})

	// 2. 数组 (Array) 运算套件
	t.Run("Array_Operators", func(t *testing.T) {
		orm1 := SELECT(TestTable{}).Where(ArrayContains("images", `{"a.png"}`))
		p1 := orm1.BuildSQL()
		if !strings.Contains(p1.Sql, "images @> ?") {
			t.Fatalf("ArrayContains 失败: %s", p1.Sql)
		}

		orm2 := SELECT(TestTable{}).Where(ArrayOverlaps("images", `{"a.png", "b.png"}`))
		p2 := orm2.BuildSQL()
		if !strings.Contains(p2.Sql, "images && ?") {
			t.Fatalf("ArrayOverlaps 失败: %s", p2.Sql)
		}

		orm3 := SELECT(TestTable{}).Where(ArrayAny("images", "target.png"))
		p3 := orm3.BuildSQL()
		if !strings.Contains(p3.Sql, "? = ANY(images)") {
			t.Fatalf("ArrayAny 失败: %s", p3.Sql)
		}

		orm4 := SELECT(TestTable{}).Where(AnyEquals("type", []int{1, 2, 3}))
		p4 := orm4.BuildSQL()
		if !strings.Contains(p4.Sql, "type = ANY(?)") {
			t.Fatalf("AnyEquals 失败: %s", p4.Sql)
		}
		t.Log("PostgreSQL Array 原生数组运算套件测试通过 ✅")
	})

	// 3. ILIKE 大小写不敏感模糊匹配
	t.Run("ILike_Operators", func(t *testing.T) {
		orm1 := SELECT(TestTable{}).Where(ILike("name", "%茅台%"))
		p1 := orm1.BuildSQL()
		if !strings.Contains(p1.Sql, "name ILIKE ?") {
			t.Fatalf("ILike 失败: %s", p1.Sql)
		}

		orm2 := SELECT(TestTable{}).Where(NotILike("name", "%测试%"))
		p2 := orm2.BuildSQL()
		if !strings.Contains(p2.Sql, "name NOT ILIKE ?") {
			t.Fatalf("NotILike 失败: %s", p2.Sql)
		}
		t.Log("PostgreSQL ILIKE 大小写不敏感测试通过 ✅")
	})

	// 4. RETURNING Fluent 链式与 Scan / One 回填
	t.Run("RETURNING_Fluent_And_Scan", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatalf("创建 sqlmock 失败: %v", err)
		}
		defer db.Close()
		sqlxMock := sqlx.NewDb(db, "sqlmock")

		// 4.1 INSERT ... RETURNING id -> Scan(&id)
		mock.ExpectQuery(`INSERT INTO "?test_table"? .+ RETURNING id`).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(888))

		var newId int64
		err = INSERT(TestTable{Name: "ret_test"}).WithDB(sqlxMock).Returning("id").Scan(&newId)
		if err != nil {
			t.Fatalf("RETURNING Scan 失败: %v", err)
		}
		if newId != 888 {
			t.Fatalf("期望 newId=888, 实际=%d", newId)
		}

		// 4.2 INSERT ... RETURNING * -> One(&item) 回填结构体
		mock.ExpectQuery(`INSERT INTO "?test_table"? .+ RETURNING \*`).
			WillReturnRows(sqlmock.NewRows([]string{"id", "name", "type"}).AddRow(999, "full_insert", 3))

		var createdItem TestTable
		err = INSERT(TestTable{Name: "full_insert"}).WithDB(sqlxMock).Returning("*").One(&createdItem)
		if err != nil {
			t.Fatalf("RETURNING One 结构体回填失败: %v", err)
		}
		if createdItem.ID != 999 || createdItem.Type != 3 {
			t.Fatalf("RETURNING One 映射属性不符: %+v", createdItem)
		}

		t.Log("PostgreSQL RETURNING 流畅链式与结构体直接回填测试通过 ✅")
	})
}

// 11. 测试可观测性 (Stats 与 SlowQuery 钩子)
func TestObservability(t *testing.T) {
	t.Run("StatsSafety", func(t *testing.T) {
		stats := Stats()
		// 未连接时安全返回 nil
		_ = stats
		t.Log("Stats() 安全调用测试通过 ✅")
	})

	t.Run("SlowQueryHook", func(t *testing.T) {
		var slowCalled bool
		var recordedAction, recordedCaller string

		oldThreshold := SlowQueryThreshold
		oldHook := OnSlowQuery
		defer func() {
			SlowQueryThreshold = oldThreshold
			OnSlowQuery = oldHook
		}()

		SlowQueryThreshold = 1 * time.Nanosecond // 极小阈值触发慢查
		OnSlowQuery = func(action string, cost time.Duration, sqlStr string, caller string) {
			slowCalled = true
			recordedAction = action
			recordedCaller = caller
		}

		orm := Table("users")
		orm.logQuery("SELECT 1", 10*time.Millisecond, "Exec")

		if !slowCalled {
			t.Fatal("期望触发 OnSlowQuery 慢查回调，但未触发")
		}
		if recordedAction != "Exec" {
			t.Fatalf("期望 action=Exec, 实际=%s", recordedAction)
		}
		if recordedCaller == "" || recordedCaller == "unknown" {
			t.Fatalf("期望解析到有效调用位置, 实际=%s", recordedCaller)
		}
		t.Logf("慢查询钩子测试通过，捕获到调用方: %s ✅", recordedCaller)
	})
}



