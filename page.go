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

	dest.Page = page
	dest.PageSize = pageSize

	// 第一步：计算总数
	countOrm := SELECT(entity).Where(cgs...).Log(debug)
	count, err := countOrm.Count("*")
	if err != nil {
		return dest, err
	}
	dest.Total = int(count)
	dest.TotalPage = dest.CalcTotalPage()

	if count == 0 {
		return dest, nil
	}

	// 第二步：查询当前页数据
	listOrm := SELECT(entity).Where(cgs...).Log(debug)

	if len(excludeTags) > 0 {
		listOrm.ExcludeFields(excludeTags...)
	}

	err = listOrm.Limit(int64(pageSize)).Offset(int64((page - 1) * pageSize)).Many(&dest.List)
	if err != nil {
		return dest, err
	}

	return dest, nil
}
