package mworm

import (
	"fmt"
	"testing"
)

type TestTable struct {
	ID        int    `json:"id" db:"id,pk"`
	Name      string `json:"name" db:"name"`
	Type      int    `json:"type" db:"type"`
	CreatedAt string `json:"createdAt" db:"created_at"`
}

func (t TestTable) TableName() string {
	return "test_table"
}

func TestOrm(t *testing.T) {
	o := SELECT(TestTable{}).Where(Exp(`created>='abc' AND abc=:abc`, "123"))
	fmt.Println(o.FullSQL())
	if o.err != nil {
		t.Fatal(o.err)
	}
	//
	o = SELECT(TestTable{}).Where(Exp(`created>='abc' AND abc=:abc`, "123", 123))
	fmt.Println(o.FullSQL())
	if o.err == nil {
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
