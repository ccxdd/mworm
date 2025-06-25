package mworm

import (
	"fmt"
	"reflect"
	"strings"
)

type ConditionType int

const (
	and = " AND "
	or  = " OR "
)

const (
	cgTypeArgs ConditionType = iota + 10
	cgTypeIn
	cgTypeNamedExpress
	cgTypeNull
	cgTypeLike
	cgTypeAsc
	cgTypeDesc
)

type ConditionGroup struct {
	Logic        string
	JsonTags     []string
	Args         []any
	InArgs       []string
	NamedExpress string // named 表达式
	cType        ConditionType
}

func (cg ConditionGroup) Transform() string {
	return ""
}

func condition(logic string, jsonTag []string, args ...any) ConditionGroup {
	cg := ConditionGroup{Logic: logic, JsonTags: jsonTag, Args: args}
	return cg
}

func And(jsonTag ...string) ConditionGroup {
	return condition(and, jsonTag)
}

func Or(jsonTag ...string) ConditionGroup {
	return condition(or, jsonTag)
}

func And2F(jsonTag string, arg any) ConditionGroup {
	return condition(and, []string{jsonTag}, arg)
}

func Or2F(jsonTag string, args ...any) ConditionGroup {
	return condition(or, []string{jsonTag}, args...)
}

func IN[T int | string](jsonTag string, args ...T) ConditionGroup {
	var result []string
	if len(args) > 0 {
		i := args[0]
		t := reflect.TypeOf(i)
		switch t.Kind() {
		case reflect.String:
			for _, arg := range args {
				result = append(result, fmt.Sprintf(`'%v'`, arg))
			}
		default:
			for _, arg := range args {
				result = append(result, fmt.Sprintf(`%v`, arg))
			}
		}
	}
	cg := ConditionGroup{
		JsonTags: []string{jsonTag},
		InArgs:   result,
		cType:    cgTypeIn,
	}
	return cg
}

// Exp 条件表达式 {table_column_field}=:{name}
func Exp(express string, args ...any) ConditionGroup {
	cg := ConditionGroup{NamedExpress: express, Args: args, cType: cgTypeNamedExpress}
	return cg
}

// ISNull 是否为空 And
func ISNull(jsonTag ...string) ConditionGroup {
	cg := condition(and, jsonTag)
	cg.cType = cgTypeNull
	return cg
}

// ISNullOr 是否为空 Or
func ISNullOr(jsonTag ...string) ConditionGroup {
	cg := condition(or, jsonTag)
	cg.cType = cgTypeNull
	return cg
}

func Like(jsonTag ...string) ConditionGroup {
	return ConditionGroup{
		Logic:    and,
		JsonTags: jsonTag,
		cType:    cgTypeLike,
	}
}

func LikeOr(jsonTag ...string) ConditionGroup {
	return ConditionGroup{
		Logic:    or,
		JsonTags: jsonTag,
		cType:    cgTypeLike,
	}
}

func Asc(j string) ConditionGroup {
	return ConditionGroup{
		JsonTags: []string{j},
		cType:    cgTypeAsc,
	}
}

func Desc(j string) ConditionGroup {
	return ConditionGroup{
		JsonTags: []string{j},
		cType:    cgTypeDesc,
	}
}

func (o *OrmModel) parseConditionNamed() string {
	var conditionSQL string
	var groupArr []string
	if len(o.namedCGArr) == 0 {
		return ""
	}
	for _, cg := range o.namedCGArr {
		switch {
		case len(cg.JsonTags) > 0 && len(cg.Args) == 0 && cg.cType < cgTypeAsc: //多字段
			var names []string
			for _, j := range cg.JsonTags {
				column := o.columnField(j)
				jsonValue := o.params[j]
				if column == "" {
					continue
				}
				switch cg.cType {
				case cgTypeNull:
					names = append(names, fmt.Sprintf(`%s IS NULL`, column))
				case cgTypeLike:
					str, b := jsonValue.(string)
					if b && len(str) > 0 {
						names = append(names, fmt.Sprintf(`%s LIKE '%%%v%%'`, column, jsonValue))
					}
				default:
					names = append(names, fmt.Sprintf(`%s=:%s`, column, j))
				}
			}
			if len(names) > 0 {
				conditionStr := `(` + strings.Join(names, cg.Logic) + `)`
				groupArr = append(groupArr, conditionStr)
			}
		case len(cg.JsonTags) == 1 && len(cg.Args) >= 1: //单字段
			column := o.columnField(cg.JsonTags[0])
			if column == "" {
				continue
			}
			var names []string
			for i, arg := range cg.Args {
				key := fmt.Sprintf(`%s%d`, cg.JsonTags[0], i+1)
				names = append(names, fmt.Sprintf(`%s=:%s`, column, key))
				o.params[key] = arg
			}
			if len(names) > 0 {
				conditionStr := `(` + strings.Join(names, cg.Logic) + `)`
				groupArr = append(groupArr, conditionStr)
			}
		case cg.cType == cgTypeIn: // IN
			column := o.columnField(cg.JsonTags[0])
			conditionStr := fmt.Sprintf(`%s IN (%s)`, column, strings.Join(cg.InArgs, ","))
			groupArr = append(groupArr, conditionStr)
		case cg.cType == cgTypeNamedExpress: //表达式
			//db_column1=:name1 OR db_column2=:name2
			subArr := strings.Split(cg.NamedExpress, ":")
			nameKeys := subArr[1:]
			if len(nameKeys) > 0 {
				var keys []string
				for _, s := range nameKeys {
					names := strings.SplitN(s, " ", 2)
					if len(names) > 0 {
						key := strings.TrimSpace(names[0])
						keys = append(keys, key)
					}
				}
				if len(keys) > 0 && len(keys) == len(cg.Args) {
					for i, key := range keys {
						o.params[key] = cg.Args[i]
					}
				}
			}
			conditionStr := `(` + cg.NamedExpress + `)`
			groupArr = append(groupArr, conditionStr)
		case cg.cType == cgTypeAsc:
			column := o.columnField(cg.JsonTags[0])
			if len(column) > 0 {
				o.orderFields = append(o.orderFields, column)
			}
		case cg.cType == cgTypeDesc:
			column := o.columnField(cg.JsonTags[0])
			if len(column) > 0 {
				o.orderFields = append(o.orderFields, column+` DESC`)
			}
		}
	}
	if len(groupArr) > 0 {
		conditionSQL = ` WHERE ` + strings.Join(groupArr, and)
	}
	return conditionSQL
}
