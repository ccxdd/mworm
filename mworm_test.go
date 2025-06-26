package mworm

import (
	"fmt"
	"github.com/jmoiron/sqlx"
	"log"
	"testing"
)

type TestTable struct {
	ID        int    `json:"id" db:"id,pk"`
	Name      string `json:"name" db:"name"`
	Type      int    `json:"type" db:"type"`
	CreatedAt string `json:"createdAt" db:"created_at"`
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
	o := SELECT(TestTable{ID: 10, Name: "name", Type: 11}).Where(AutoFill(), Desc("id"))
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
	s, _ := SELECT(TestTable{}).Where(Exp(`created>='abc' AND abc=:abc`, "123")).With("t").WithAsc("date").FullSQL()
	fmt.Println(s)
	if o.err != nil {
		t.Fail()
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
	result, err := DebugPAGE(TbUser{Username: "user"}, true, 1, 10,
		Desc("createdAt"), Like("username"), IsNull("invitationCode", "encPhone"), And("age"))
	if err != nil {
		t.Fatal(err)
	}
	t.Log("总页数", result.TotalPage, "记录数", result.Total)
}
