package mworm

import (
	"errors"
	"fmt"
)

type PageResult[T any] struct {
	Total     int `json:"total" db:"total"`          //总记录数
	TotalPage int `json:"totalPage" db:"total_page"` //总页数
	Page      int `json:"page" db:"page"`            //当前页
	PageSize  int `json:"pageSize" db:"page_size"`   //页面数量
	List      []T `json:"list" db:"list"`            //分页数据
}

func (pr PageResult[T]) CalcTotalPage() int {
	mod := pr.Total % pr.PageSize
	if mod == 0 {
		return pr.TotalPage / pr.PageSize
	} else {
		return pr.TotalPage/pr.PageSize + 1
	}
}

func PAGE[T ORMInterface](entity T, page, pageSize int, cgs ...ConditionGroup) (PageResult[T], error) {
	var dest PageResult[T]
	if pageSize < 1 {
		return dest, errors.New("page size must be greater than zero")
	}
	orm := SELECT(entity).Where(cgs...)
	tableSql, _ := orm.NamedSQL()
	//fmt.Println(tableSql)
	jsonKeys := JsonbBuildObjString(entity, `row`)
	sql := `
	WITH t AS (%s),
	t1 AS (SELECT count(*) as total FROM t),
	t2 AS (SELECT jsonb_agg(jsonb_build_object(%s)) list FROM (SELECT * FROM t LIMIT %d OFFSET %d) row),
	t3 AS (SELECT t2.*, t1.* FROM t2 CROSS JOIN t1)
	SELECT * FROM t3;
	`
	sql = fmt.Sprintf(sql, tableSql, jsonKeys, pageSize, (page-1)*pageSize)
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
