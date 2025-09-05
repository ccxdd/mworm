package mworm

import (
	"fmt"
	"log"
	"testing"

	"github.com/jmoiron/sqlx"
)

type TestTable struct {
	ID        int      `json:"id" db:"id,pk"`
	Name      string   `json:"name" db:"name"`
	Type      int      `json:"type" db:"type"`
	CreatedAt string   `json:"createdAt" db:"created_at"`
	Images    []string `json:"images" db:"images"`
}

type TestStruct struct {
	Name string `json:"name"`
	Age  int    `json:"age"`
	high string `json:"high"`
}

func (t TestTable) TableName() string {
	return "test_table"
}

func OpenSqlxDB() {
	db, err := sqlx.Open("postgres", DBConnectionString())
	if err != nil {
		log.Fatalln(err)
	}
	db.SetMaxOpenConns(100)
	db.SetMaxIdleConns(100)
	err = BindDB(db)
	if err != nil {
		log.Fatalln(err)
	}
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
	OpenSqlxDB()
	result, err := DebugPAGE(TbUser{Username: "user"}, true, 1, 10, nil,
		Desc("createdAt"), Like("username"), IsNull("invitationCode", "encPhone"), And("age"))
	if err != nil {
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
	OpenSqlxDB()
	if err := UPDATE(CreateMatch{ID: 3, HomeTeam: "更新队名"}).WherePK().Log(true).Exec(); err != nil {
		t.Fatal(err)
	}
	t.Log("更新成功")

	if err := DELETE(CreateMatch{ID: 5}).WherePK().Exec(); err != nil {
		t.Fatal(err)
	}
	t.Log("删除成功")
}

func TestUpdateEmpty(t *testing.T) {
	OpenSqlxDB()
	DebugMode = true
	_ = UPDATE(CreateMatch{ID: 12, HomeTeam: ""}).AllowEmpty("homeTeam").WherePK().BuildSQL()
	_ = SELECT(CreateMatch{}).Where(Gte("id", 12), Lte("id", 10)).BuildSQL()
}

func TestRawCond(t *testing.T) {
	OpenSqlxDB()
	params := UPDATE(CreateMatch{ID: 12, HomeTeam: ""}).Where(Raw(`league_id>='2' AND status=$1 AND id=$2`, `666`, 222)).
		AllowEmpty("homeTeam").WherePK().BuildSQL()
	fmt.Println(params.Sql)
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
	OpenSqlxDB()
}

func TestJsonMap(t *testing.T) {
	OpenSqlxDB()
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
