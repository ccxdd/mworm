package mworm

import (
	"fmt"
)

// PageResult 用于分页查询结果的结构体，包含总数、总页数、当前页、每页数量和数据列表。
type PageResult[T any] struct {
	Total     int `json:"total" db:"total"`          //总记录数
	TotalPage int `json:"totalPage" db:"total_page"` //总页数
	Page      int `json:"page" db:"page"`            //当前页
	PageSize  int `json:"pageSize" db:"page_size"`   //页面数量
	List      []T `json:"list" db:"list"`            //分页数据
}

// CalcTotalPage 计算总页数
func (pr PageResult[T]) CalcTotalPage() int {
	mod := pr.Total % pr.PageSize
	if mod == 0 {
		return pr.Total / pr.PageSize
	} else {
		return pr.Total/pr.PageSize + 1
	}
}

// Error 包含了详细的错误信息
type Error struct {
	Code    int    // 错误码
	Message string // 错误信息
	SQL     string // SQL语句
}

func (e Error) Error() string {
	return fmt.Sprintf("Code: %d, Message: %s, SQL: %s", e.Code, e.Message, e.SQL)
}

// 定义常见错误
var (
	ErrInvalidPageSize = &Error{Code: 1001, Message: "page size must be greater than zero"}
	ErrNilDB           = &Error{Code: 1002, Message: "database connection is nil"}
	ErrEmptySQL        = &Error{Code: 1003, Message: "SQL statement is empty"}
	ErrNoEffect        = &Error{Code: 1004, Message: "no rows affected"}
)

// PAGE 分页查询方法，支持排除指定的json tag字段
func PAGE[T ORMInterface](entity T, page, pageSize int, excludeTags []string, cgs ...ConditionGroup) (PageResult[T], error) {
	return DebugPAGE(entity, false, page, pageSize, excludeTags, cgs...)
}

// DebugPAGE 分页查询方法，支持调试和排除指定的json tag字段
func DebugPAGE[T ORMInterface](entity T, debug bool, page, pageSize int, excludeTags []string, cgs ...ConditionGroup) (PageResult[T], error) {
	var dest PageResult[T]
	if pageSize < 1 {
		return dest, ErrInvalidPageSize
	}
	orm := SELECT(entity).Where(cgs...).Log(debug)
	sqlParams := orm.BuildSQL()
	//fmt.Println(tableSql)
	var jsonKeys string
	if len(excludeTags) > 0 {
		jsonKeys = JsonTagToJsonbKeys(entity, `row`, excludeTags...)
	} else {
		jsonKeys = JsonbBuildObjString(entity, `row`)
	}
	sql := `
	WITH t AS (%s),
	t1 AS (SELECT count(*) as total FROM t),
	t2 AS (SELECT jsonb_agg(jsonb_build_object(%s)) list FROM (SELECT * FROM t LIMIT %d OFFSET %d) row),
	t3 AS (SELECT t2.*, t1.* FROM t2 CROSS JOIN t1)
	SELECT * FROM t3;
	`
	sql = fmt.Sprintf(sql, sqlParams.Sql, jsonKeys, pageSize, (page-1)*pageSize)
	//fmt.Println(sql)
	if err := NamedQueryWithMap(sql, orm.params, &dest); err != nil {
		return dest, err
	}
	if len(dest.List) > 0 {
		dest.Page = page
		dest.PageSize = pageSize
		dest.TotalPage = dest.CalcTotalPage()
	}
	return dest, nil
}
