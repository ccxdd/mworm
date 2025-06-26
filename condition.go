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
	cgTypeAndOrNonZero ConditionType = iota + 10
	cgTypeAndOrZero
	cgTypeAnd2F
	cgTypeOr2F
	cgTypeIn
	cgTypeNamedExpress
	cgTypeNull
	cgTypeLike
	cgTypeAsc
	cgTypeDesc
	cgTypeSymbol
	cgAutoFill     = 99
	cgAutoFillZero = 100
)

type ConditionGroup struct {
	Logic        string
	Symbol       string
	JsonTags     []string
	Args         []any
	InArgs       []string
	NamedExpress string // named 表达式
	cType        ConditionType
}

func (cg ConditionGroup) Transform() string {
	return ""
}

func And(tag ...string) ConditionGroup {
	return ConditionGroup{Logic: and, JsonTags: tag, cType: cgTypeAndOrNonZero}
}

func Or(tag ...string) ConditionGroup {
	return ConditionGroup{Logic: or, JsonTags: tag, cType: cgTypeAndOrNonZero}
}

func AndZero(tag ...string) ConditionGroup {
	return ConditionGroup{Logic: and, JsonTags: tag, cType: cgTypeAndOrZero}
}

func OrZero(tag ...string) ConditionGroup {
	return ConditionGroup{Logic: or, JsonTags: tag, cType: cgTypeAndOrZero}
}

func And2F(tag string, arg any) ConditionGroup {
	return ConditionGroup{Logic: and, JsonTags: []string{tag}, Args: []any{arg}, cType: cgTypeAnd2F}
}

func Or2F(tag string, args ...any) ConditionGroup {
	return ConditionGroup{Logic: or, JsonTags: []string{tag}, Args: args, cType: cgTypeOr2F}
}

func IN[T int | string](tag string, args ...T) ConditionGroup {
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
	return ConditionGroup{
		JsonTags: []string{tag},
		InArgs:   result,
		cType:    cgTypeIn,
	}
}

// Exp 条件表达式 {table_column_field}=:{name}
func Exp(express string, args ...any) ConditionGroup {
	return ConditionGroup{NamedExpress: express, Args: args, cType: cgTypeNamedExpress}
}

// IsNull 是否为空 And
func IsNull(tag ...string) ConditionGroup {
	return ConditionGroup{
		Logic:    and,
		JsonTags: tag,
		cType:    cgTypeNull,
	}
}

// IsNullOR 是否为空 OR
func IsNullOR(tag ...string) ConditionGroup {
	return ConditionGroup{
		Logic:    or,
		JsonTags: tag,
		cType:    cgTypeNull,
	}
}

func Eq(tag string, args ...any) ConditionGroup {
	return ConditionGroup{
		Symbol:   "=",
		JsonTags: []string{tag},
		Args:     args,
		cType:    cgTypeSymbol,
	}
}

func Gt(tag string, args ...any) ConditionGroup {
	return ConditionGroup{
		Symbol:   ">",
		JsonTags: []string{tag},
		Args:     args,
		cType:    cgTypeSymbol,
	}
}

func Gte(tag string, args ...any) ConditionGroup {
	return ConditionGroup{
		Symbol:   ">=",
		JsonTags: []string{tag},
		Args:     args,
		cType:    cgTypeSymbol,
	}
}

func Lt(tag string, args ...any) ConditionGroup {
	return ConditionGroup{
		Symbol:   "<",
		JsonTags: []string{tag},
		Args:     args,
		cType:    cgTypeSymbol,
	}
}

func Lte(tag string, args ...any) ConditionGroup {
	return ConditionGroup{
		Symbol:   "<=",
		JsonTags: []string{tag},
		Args:     args,
		cType:    cgTypeSymbol,
	}
}

func Like(tag ...string) ConditionGroup {
	return ConditionGroup{
		Logic:    and,
		JsonTags: tag,
		cType:    cgTypeLike,
	}
}

func LikeOR(tag ...string) ConditionGroup {
	return ConditionGroup{
		Logic:    or,
		JsonTags: tag,
		cType:    cgTypeLike,
	}
}

func Asc(tag string) ConditionGroup {
	return ConditionGroup{
		JsonTags: []string{tag},
		cType:    cgTypeAsc,
	}
}

func Desc(tag string) ConditionGroup {
	return ConditionGroup{
		JsonTags: []string{tag},
		cType:    cgTypeDesc,
	}
}

func AutoFill(zero ...bool) ConditionGroup {
	if len(zero) > 0 && zero[0] == true {
		return ConditionGroup{
			cType: cgAutoFillZero,
		}
	}
	return ConditionGroup{
		cType: cgAutoFill,
	}
}

func (o *OrmModel) parseConditionNamed() string {
	var conditionSQL string
	var groupArr []string
	if len(o.namedCGArr) == 0 {
		return ""
	}
	for _, cg := range o.namedCGArr {
		switch cg.cType {
		case cgTypeAndOrNonZero, cgTypeNull, cgTypeLike, cgTypeAndOrZero:
			var names []string
			for _, j := range cg.JsonTags {
				column := o.columnField(j)
				if column == "" {
					continue
				}
				jv := o.params[column]
				switch cg.cType {
				case cgTypeAndOrNonZero, cgTypeAndOrZero:
					switch jv.(type) {
					case string:
						names = append(names, fmt.Sprintf(`%s='%s'`, column, jv))
					case int, int64, uint, uint64, float32, float64:
						if jv == 0 && cg.cType == cgTypeAndOrNonZero {
							continue
						}
						names = append(names, fmt.Sprintf(`%s=%v`, column, jv))
					}
				case cgTypeNull:
					names = append(names, fmt.Sprintf(`%s IS NULL`, column))
				case cgTypeLike:
					str, b := jv.(string)
					if b && len(str) > 0 {
						names = append(names, fmt.Sprintf(`%s LIKE '%%%s%%'`, column, jv))
					}
				default:
				}
			}
			if len(names) > 0 {
				conditionStr := `(` + strings.Join(names, cg.Logic) + `)`
				groupArr = append(groupArr, conditionStr)
			}
		case cgTypeOr2F, cgTypeAnd2F:
			column := o.columnField(cg.JsonTags[0])
			if column == "" {
				continue
			}
			var names []string
			for _, arg := range cg.Args {
				names = append(names, fmt.Sprintf(`%s=%s`, column, ValueTypeToStr(arg)))
			}
			if len(names) > 0 {
				conditionStr := `(` + strings.Join(names, cg.Logic) + `)`
				groupArr = append(groupArr, conditionStr)
			}
		case cgTypeIn: // IN
			column := o.columnField(cg.JsonTags[0])
			conditionStr := fmt.Sprintf(`%s IN (%s)`, column, strings.Join(cg.InArgs, ","))
			groupArr = append(groupArr, conditionStr)
		case cgTypeNamedExpress: //表达式
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
				if len(keys) > 0 && len(keys) <= len(cg.Args) {
					for i, key := range keys {
						cg.NamedExpress = strings.Replace(cg.NamedExpress, ":"+key, ValueTypeToStr(cg.Args[i]), 1)
					}
				}
			}
			conditionStr := `(` + cg.NamedExpress + `)`
			groupArr = append(groupArr, conditionStr)
		case cgTypeAsc:
			column := o.columnField(cg.JsonTags[0])
			if len(column) > 0 {
				o.orderFields = append(o.orderFields, column)
			}
		case cgTypeDesc:
			column := o.columnField(cg.JsonTags[0])
			if len(column) > 0 {
				o.orderFields = append(o.orderFields, column+` DESC`)
			}
		case cgTypeSymbol:
			column := o.columnField(cg.JsonTags[0])
			if column == "" {
				continue
			}
			var vStr string
			if len(cg.Args) > 0 {
				vStr = ValueTypeToStr(cg.Args[0])
			} else {
				vStr = ValueTypeToStr(o.params[column])
			}
			if vStr == "" || vStr == "''" {
				continue
			}
			condition := fmt.Sprintf("%s%s%s", column, cg.Symbol, vStr)
			groupArr = append(groupArr, condition)
		case cgAutoFill, cgAutoFillZero:
			var conditionArr []string
			for column, _ := range o.dbFields {
				if len(column) == 0 {
					continue
				}
				vStr := ValueTypeToStr(o.params[column])
				if cg.cType == cgAutoFill && (vStr == "" || vStr == "''" || vStr == "0") {
					continue
				}
				conditionArr = append(conditionArr, fmt.Sprintf(`%s=%v`, column, vStr))
			}
			if len(conditionArr) > 0 {
				conditionStr := `(` + strings.Join(conditionArr, ` AND `) + `)`
				groupArr = append(groupArr, conditionStr)
			}
		default:
		}
	}
	if len(groupArr) > 0 {
		conditionSQL = ` WHERE ` + strings.Join(groupArr, and)
	}
	return conditionSQL
}

func ValueTypeToStr(v any) string {
	switch v.(type) {
	case nil:
		return ""
	case string:
		return fmt.Sprintf(`'%v'`, v)
	case *string:
		pf := v.(*string)
		if pf == nil {
			return ""
		}
		return fmt.Sprintf(`'%s'`, *v.(*string))
	default:
		return fmt.Sprintf(`%v`, v)
	}
}
