package mworm

import (
	"fmt"
	"log"
	"testing"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
)

type TestUser struct {
	ID           int64  `json:"id" db:"id,pk"`
	MemberCode   string `json:"memberCode" db:"member_code"`
	RealName     string `json:"realName" db:"real_name"`
	NickName     string `json:"nickName" db:"nick_name"`
	Status       string `json:"status" db:"status"`
	Avatar       string `json:"avatar" db:"avatar"`
	Mobile       string `json:"mobile" db:"mobile"`
	Platform     string `json:"platform" db:"platform"`
	TenantID     int64  `json:"tenantId" db:"tenant_id"`
	CreatedOrgID int64  `json:"createdOrgId" db:"created_org_id"`
	CreatedBy    int64  `json:"createdBy" db:"created_by"`
	CreatedTime  string `json:"createdTime" db:"created_time"`
	UpdatedBy    int64  `json:"updatedBy" db:"updated_by"`
	UpdatedTime  string `json:"updatedTime" db:"updated_time"`
	Remark       string `json:"remark" db:"remark"`
	Sex          string `json:"sex" db:"sex"`
}

// TableName 返回表名
func (tu TestUser) TableName() string {
	return "c_user"
}

func connectMysql() {
	db, err := sqlx.Open("mysql", "ttit:ttit@412@tcp(192.168.9.45:3306)/lamp_column_cyb_dev")
	if err != nil {
		log.Fatalln(err)
	}
	db.SetMaxOpenConns(500)
	db.SetMaxIdleConns(400)
	err = BindDB(db)
	if err != nil {
		log.Fatalln(err)
	}
}

func TestMysqlTable(t *testing.T) {
	connectMysql()
	var users []TestUser
	if err := SELECT(TestUser{}).Asc("createdTime").Limit(2).Many(&users); err != nil {
		t.Fail()
	}
	fmt.Println(users)
	var user TestUser
	if err := SELECT(TestUser{ID: 512355444833460228}).One(&user); err != nil {
		t.Fail()
	}
	fmt.Println(user)
	if err := DELETE(TestUser{ID: 512355444833460222}).WherePK().Exec(); err != nil {
		t.Fail()
	}
}
