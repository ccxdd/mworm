package mworm

import (
	"fmt"
	jsoniter "github.com/json-iterator/go"
	"reflect"
	"strings"
)

type ConditionType int // ConditionType 条件类型枚举

const (
	and = " AND "
	or  = " OR "
)

const (
	cgTypeAndOrNonZero ConditionType = iota + 10 // cgTypeAndOrNonZero: AND/OR 非零值
	cgTypeAndOrZero                              // cgTypeAndOrZero: AND/OR 零值
	cgTypeAnd2F                                  // cgTypeAnd2F: AND 单字段条件
	cgTypeOr2F                                   // cgTypeOr2F: OR 单字段条件
	cgTypeIn                                     // cgTypeIn: IN 查询
	cgTypeNamedExpress                           // cgTypeNamedExpress: 命名表达式
	cgTypeNull                                   // cgTypeNull: NULL 判断
	cgTypeLike                                   // cgTypeLike: LIKE 查询
	cgTypeAsc                                    // cgTypeAsc: 升序
	cgTypeDesc                                   // cgTypeDesc: 降序
	cgTypeSymbol                                 // cgTypeSymbol: 符号条件
	cgAutoFill         = 99                      // cgAutoFill: 自动填充
	cgAutoFillZero     = 100                     // cgAutoFillZero: 自动填充零值
)

// ConditionGroup 条件分组结构体，描述 SQL 查询的条件
type ConditionGroup struct {
	Logic        string        // Logic: 逻辑运算符（AND/OR）
	Symbol       string        // Symbol: 比较符号（=, >, < 等）
	JsonTags     []string      // JsonTags: 参与条件的字段名
	Args         []any         // Args: 参数值
	InArgs       []string      // InArgs: IN 查询参数
	NamedExpress string        // NamedExpress: 命名表达式
	cType        ConditionType // cType: 条件类型
}

// Transform 转换为 SQL 字符串（未实现）
func (cg ConditionGroup) Transform() string {
	return ""
}

// And 构造 AND 非零条件分组
func And(tag ...string) ConditionGroup {
	return ConditionGroup{Logic: and, JsonTags: tag, cType: cgTypeAndOrNonZero}
}

// Or 构造 OR 非零条件分组
func Or(tag ...string) ConditionGroup {
	return ConditionGroup{Logic: or, JsonTags: tag, cType: cgTypeAndOrNonZero}
}

// AndZero 构造 AND 零值条件分组
func AndZero(tag ...string) ConditionGroup {
	return ConditionGroup{Logic: and, JsonTags: tag, cType: cgTypeAndOrZero}
}

// OrZero 构造 OR 零值条件分组
func OrZero(tag ...string) ConditionGroup {
	return ConditionGroup{Logic: or, JsonTags: tag, cType: cgTypeAndOrZero}
}

// And2F 构造 AND 单字段条件分组
func And2F(tag string, arg any) ConditionGroup {
	return ConditionGroup{Logic: and, JsonTags: []string{tag}, Args: []any{arg}, cType: cgTypeAnd2F}
}

// Or2F 构造 OR 单字段条件分组
func Or2F(tag string, args ...any) ConditionGroup {
	return ConditionGroup{Logic: or, JsonTags: []string{tag}, Args: args, cType: cgTypeOr2F}
}

// IN 构造 IN 查询条件分组
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

// Eq 构造等于条件分组
func Eq(tag string, args ...any) ConditionGroup {
	return ConditionGroup{
		Symbol:   "=",
		JsonTags: []string{tag},
		Args:     args,
		cType:    cgTypeSymbol,
	}
}

// Gt 构造大于条件分组
func Gt(tag string, args ...any) ConditionGroup {
	return ConditionGroup{
		Symbol:   ">",
		JsonTags: []string{tag},
		Args:     args,
		cType:    cgTypeSymbol,
	}
}

// Gte 构造大于等于条件分组
func Gte(tag string, args ...any) ConditionGroup {
	return ConditionGroup{
		Symbol:   ">=",
		JsonTags: []string{tag},
		Args:     args,
		cType:    cgTypeSymbol,
	}
}

// Lt 构造小于条件分组
func Lt(tag string, args ...any) ConditionGroup {
	return ConditionGroup{
		Symbol:   "<",
		JsonTags: []string{tag},
		Args:     args,
		cType:    cgTypeSymbol,
	}
}

// Lte 构造小于等于条件分组
func Lte(tag string, args ...any) ConditionGroup {
	return ConditionGroup{
		Symbol:   "<=",
		JsonTags: []string{tag},
		Args:     args,
		cType:    cgTypeSymbol,
	}
}

// Like 构造 AND LIKE 条件分组
func Like(tag ...string) ConditionGroup {
	return ConditionGroup{
		Logic:    and,
		JsonTags: tag,
		cType:    cgTypeLike,
	}
}

// LikeOR 构造 OR LIKE 条件分组
func LikeOR(tag ...string) ConditionGroup {
	return ConditionGroup{
		Logic:    or,
		JsonTags: tag,
		cType:    cgTypeLike,
	}
}

// Asc 构造升序条件分组
func Asc(tag string) ConditionGroup {
	return ConditionGroup{
		JsonTags: []string{tag},
		cType:    cgTypeAsc,
	}
}

// Desc 构造降序条件分组
func Desc(tag string) ConditionGroup {
	return ConditionGroup{
		JsonTags: []string{tag},
		cType:    cgTypeDesc,
	}
}

// AutoFill 自动填充条件分组
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
					vStr := ValueTypeToStr(jv)
					if (vStr == "" || vStr == `''` || vStr == "0") && cg.cType == cgTypeAndOrNonZero {
						continue
					}
					if vStr == "" && cg.cType == cgTypeAndOrZero {
						continue
					}
					names = append(names, fmt.Sprintf(`%s=%s`, column, vStr))
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
			if vStr == "" || vStr == `''` {
				continue
			}
			condition := fmt.Sprintf("%s%s%s", column, cg.Symbol, vStr)
			groupArr = append(groupArr, condition)
		case cgAutoFill, cgAutoFillZero:
			var conditionArr []string
			for json, column := range o.dbFields {
				if len(column) == 0 || json == primaryKey {
					continue
				}
				vStr := ValueTypeToStr(o.params[column])
				if cg.cType == cgAutoFill && (vStr == "" || vStr == `''` || vStr == `0`) {
					continue
				}
				if vStr == "" && cg.cType == cgAutoFillZero {
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
	case int, int16, int32, int64, float32, float64, uint, uint8, uint16, uint32, uint64, bool:
		return fmt.Sprintf(`%v`, v)
	default:
		jsonStr, err := jsoniter.MarshalToString(v)
		if err != nil || jsonStr == "null" {
			return ""
		}
		return fmt.Sprintf(`'%s'`, jsonStr)
	}
}
