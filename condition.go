package mworm

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/bytedance/sonic"
)

type ConditionType int // ConditionType 条件类型枚举

const (
	and = " AND "
	or  = " OR "
)

const (
	cgTypeAndOr           ConditionType = iota + 10 // cgTypeAndOr: AND/OR
	cgTypeAndOrAutoRemove                           // cgTypeAndOrAutoRemove 空值移除
	cgTypeAnd2F                                     // cgTypeAnd2F: AND 单字段条件
	cgTypeOr2F                                      // cgTypeOr2F: OR 单字段条件
	cgTypeIn                                        // cgTypeIn: IN 查询
	cgTypeNotIn                                     // cgTypeNotIn: NOT IN 查询
	cgTypeBetween                                   // cgTypeBetween: BETWEEN 范围查询
	cgTypeNamedExpress                              // cgTypeNamedExpress: 命名表达式
	cgTypeNull                                      // cgTypeNull: NULL 判断
	cgTypeNotEqualNull                              // cgTypeNotEqualNull: NULL != 判断
	cgTypeLike                                      // cgTypeLike: LIKE 查询
	cgTypeNotEqualLike                              // cgTypeNotEqualLike: LIKE != 查询
	cgTypeAsc                                       // cgTypeAsc: 升序
	cgTypeDesc                                      // cgTypeDesc: 降序
	cgTypeSymbol                                    // cgTypeSymbol: 符号条件
	cgTypeRaw                                       // cgTypeRaw: 原始条件
	cgTypeGroupFields                               // cgTypeGroupFields: 分组字段
	cgTypeOrGroup                                   // cgTypeOrGroup: OR 嵌套分组
	cgTypeAndGroup                                  // cgTypeAndGroup: AND 嵌套分组
	cgTypeExists                                    // cgTypeExists: EXISTS 子查询
	cgTypeNotExists                                 // cgTypeNotExists: NOT EXISTS 子查询
	cgTypeSubQuery                                  // cgTypeSubQuery: WHERE 子查询
	cgAutoFill            = 99                      // cgAutoFill: 自动填充
	cgAutoFillZero        = 100                     // cgAutoFillZero: 自动填充零值
)

// ConditionGroup 条件分组结构体，描述 SQL 查询的条件
type ConditionGroup struct {
	Logic     string           // Logic: 逻辑运算符（AND/OR）
	Symbol    string           // Symbol: 比较符号（=, >, < 等）
	JsonTags  []string         // JsonTags: 参与条件的字段名
	Args      []any            // Args: 参数值
	InArgs    []string         // InArgs: IN 查询参数
	Express   string           // Express: 表达式
	cType     ConditionType    // cType: 条件类型
	SubGroups []ConditionGroup // SubGroups: 子条件分组（用于嵌套 OR/AND）
}

// Transform 转换为 SQL 字符串（未实现）
func (cg ConditionGroup) Transform() string {
	return ""
}

// And 构造 AND 非零条件分组
func And(tag ...string) ConditionGroup {
	return ConditionGroup{Logic: and, JsonTags: tag, cType: cgTypeAndOr}
}

// Or 构造 OR 非零条件分组
func Or(tag ...string) ConditionGroup {
	return ConditionGroup{Logic: or, JsonTags: tag, cType: cgTypeAndOr}
}

// AndAuto 构造 AND 值为空时该条件移除
func AndAuto(tag ...string) ConditionGroup {
	return ConditionGroup{Logic: and, JsonTags: tag, cType: cgTypeAndOrAutoRemove}
}

// OrAuto 构造 OR 值为空时该条件移除
func OrAuto(tag ...string) ConditionGroup {
	return ConditionGroup{Logic: or, JsonTags: tag, cType: cgTypeAndOrAutoRemove}
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
func IN[T any](tag string, args ...T) ConditionGroup {
	interfaceArgs := make([]any, len(args))
	for i, v := range args {
		interfaceArgs[i] = v
	}
	return ConditionGroup{
		JsonTags: []string{tag},
		Args:     interfaceArgs,
		cType:    cgTypeIn,
	}
}

// NotIN 构造 NOT IN 查询条件分组
func NotIN[T any](tag string, args ...T) ConditionGroup {
	interfaceArgs := make([]any, len(args))
	for i, v := range args {
		interfaceArgs[i] = v
	}
	return ConditionGroup{
		JsonTags: []string{tag},
		Args:     interfaceArgs,
		cType:    cgTypeNotIn,
	}
}

// Between 构造 BETWEEN 范围查询条件分组，如 Between("age", 18, 60) → age BETWEEN 18 AND 60
func Between(tag string, min, max any) ConditionGroup {
	return ConditionGroup{
		JsonTags: []string{tag},
		Args:     []any{min, max},
		cType:    cgTypeBetween,
	}
}

// Exp 条件表达式 {table_column_field}=:{name}
func Exp(express string, args ...any) ConditionGroup {
	return ConditionGroup{Express: express, Args: args, cType: cgTypeNamedExpress}
}

// Raw 条件表达式 column1 = 2 AND column2 = 'abc' 或 column1 = $1 AND column2 = $2
func Raw(express string, args ...any) ConditionGroup {
	return ConditionGroup{Express: express, Args: args, cType: cgTypeRaw}
}

// Null 是否为空 And
func Null(tag ...string) ConditionGroup {
	return ConditionGroup{
		Logic:    and,
		JsonTags: tag,
		cType:    cgTypeNull,
	}
}

// NEqNull 不等于空 And
func NEqNull(tag ...string) ConditionGroup {
	return ConditionGroup{
		Logic:    and,
		JsonTags: tag,
		cType:    cgTypeNotEqualNull,
	}
}

// NullOR 是否为空 OR
func NullOR(tag ...string) ConditionGroup {
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

// NEq 不等于
func NEq(tag string, args ...any) ConditionGroup {
	return ConditionGroup{
		Symbol:   "!=",
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

// NEqLike 构造 AND LIKE != 条件分组
func NEqLike(tag ...string) ConditionGroup {
	return ConditionGroup{
		Logic:    and,
		JsonTags: tag,
		cType:    cgTypeNotEqualLike,
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
	if len(zero) > 0 && zero[0] {
		return ConditionGroup{
			cType: cgAutoFillZero,
		}
	}
	return ConditionGroup{
		cType: cgAutoFill,
	}
}

func Fields(tag ...string) ConditionGroup {
	return ConditionGroup{
		JsonTags: tag,
		cType:    cgTypeGroupFields,
	}
}

// OrGroup 构造 OR 嵌套分组，如 OrGroup(Eq("a",1), Eq("b",2)) → (a=1 OR b=2)
func OrGroup(cgs ...ConditionGroup) ConditionGroup {
	return ConditionGroup{
		Logic:     or,
		SubGroups: cgs,
		cType:     cgTypeOrGroup,
	}
}

// AndGroup 构造 AND 嵌套分组，如 AndGroup(Gt("a",1), Lt("b",10)) → (a>1 AND b<10)
func AndGroup(cgs ...ConditionGroup) ConditionGroup {
	return ConditionGroup{
		Logic:     and,
		SubGroups: cgs,
		cType:     cgTypeAndGroup,
	}
}

// Exists 构造 EXISTS 子查询条件，sql 为完整的子查询语句
func Exists(sql string, args ...any) ConditionGroup {
	return ConditionGroup{
		Express: sql,
		Args:    args,
		cType:   cgTypeExists,
	}
}

// NotExists 构造 NOT EXISTS 子查询条件
func NotExists(sql string, args ...any) ConditionGroup {
	return ConditionGroup{
		Express: sql,
		Args:    args,
		cType:   cgTypeNotExists,
	}
}

// SubQuery 构造 WHERE 子查询条件，如 SubQuery("id", "IN", "SELECT user_id FROM orders WHERE amount > $1", 100)
func SubQuery(tag, symbol, sql string, args ...any) ConditionGroup {
	return ConditionGroup{
		JsonTags: []string{tag},
		Symbol:   symbol,
		Express:  sql,
		Args:     args,
		cType:    cgTypeSubQuery,
	}
}

func (o *OrmModel) parseConditionNamed() (string, []any) {
	var conditionArgs []any
	var groupArr []string
	if len(o.namedCGArr) == 0 {
		return "", nil
	}
	for _, cg := range o.namedCGArr {
		switch cg.cType {
		case cgTypeAndOr, cgTypeNull, cgTypeLike, cgTypeNotEqualLike, cgTypeNotEqualNull, cgTypeAndOrAutoRemove:
			var names []string
			for _, j := range cg.JsonTags {
				column := o.columnField(j)
				if column == "" {
					continue
				}
				jv := o.params[column]
				switch cg.cType {
				case cgTypeAndOr, cgTypeAndOrAutoRemove:
					vStr := ValueTypeToStr(jv)
					if (vStr == `` || vStr == `''` || vStr == `0`) && cg.cType == cgTypeAndOrAutoRemove {
						continue
					}
					names = append(names, column+`=?`)
					conditionArgs = append(conditionArgs, jv)
				case cgTypeNull:
					names = append(names, column+` IS NULL`)
				case cgTypeNotEqualNull:
					names = append(names, column+` IS NOT NULL`)
				case cgTypeLike:
					str, b := jv.(string)
					if b && len(str) > 0 {
						names = append(names, column+` LIKE ?`)
						conditionArgs = append(conditionArgs, "%"+str+"%")
					}
				case cgTypeNotEqualLike:
					str, b := jv.(string)
					if b && len(str) > 0 {
						names = append(names, column+` NOT LIKE ?`)
						conditionArgs = append(conditionArgs, "%"+str+"%")
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
				names = append(names, column+`=?`)
				conditionArgs = append(conditionArgs, arg)
			}
			if len(names) > 0 {
				conditionStr := `(` + strings.Join(names, cg.Logic) + `)`
				groupArr = append(groupArr, conditionStr)
			}
		case cgTypeIn, cgTypeNotIn: // IN / NOT IN
			column := o.columnField(cg.JsonTags[0])
			var placeholders []string
			for _, arg := range cg.Args {
				placeholders = append(placeholders, "?")
				conditionArgs = append(conditionArgs, arg)
			}
			keyword := "IN"
			if cg.cType == cgTypeNotIn {
				keyword = "NOT IN"
			}
			conditionStr := column + ` ` + keyword + ` (` + strings.Join(placeholders, ",") + `)`
			groupArr = append(groupArr, conditionStr)
		case cgTypeBetween: // BETWEEN
			column := o.columnField(cg.JsonTags[0])
			if len(cg.Args) >= 2 {
				conditionStr := column + ` BETWEEN ? AND ?`
				conditionArgs = append(conditionArgs, cg.Args[0], cg.Args[1])
				groupArr = append(groupArr, conditionStr)
			}
		case cgTypeNamedExpress: //表达式
			subArr := strings.Split(cg.Express, ":")
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
						cg.Express = strings.Replace(cg.Express, ":"+key, "?", 1)
						conditionArgs = append(conditionArgs, cg.Args[i])
					}
				}
			}
			conditionStr := `(` + cg.Express + `)`
			groupArr = append(groupArr, conditionStr)
		case cgTypeRaw:
			if cg.Express == "" {
				continue
			}
			if len(cg.Args) == 0 {
				conditionStr := `(` + cg.Express + `)`
				groupArr = append(groupArr, conditionStr)
			} else {
				conditionStr := `(` + cg.Express + `)`
				for i, arg := range cg.Args {
					conditionStr = strings.Replace(conditionStr, "$"+strconv.Itoa(i+1), "?", 1)
					conditionArgs = append(conditionArgs, arg)
				}
				groupArr = append(groupArr, conditionStr)
			}
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

			var argValue any
			if len(cg.Args) > 0 {
				argValue = cg.Args[0]
			} else {
				argValue = o.params[column]
			}
			vStr := ValueTypeToStr(argValue)
			if vStr == "" || vStr == `''` {
				continue
			}
			condition := column + cg.Symbol + "?"
			conditionArgs = append(conditionArgs, argValue)
			groupArr = append(groupArr, condition)
		case cgAutoFill, cgAutoFillZero:
			var conditionArr []string
			for _, column := range o.dbFields {
				if len(column) == 0 {
					continue
				}
				jv := o.params[column]
				vStr := ValueTypeToStr(jv)
				if cg.cType == cgAutoFill && (vStr == "" || vStr == `''` || vStr == `0`) {
					continue
				}
				if vStr == "" && cg.cType == cgAutoFillZero {
					continue
				}
				conditionArr = append(conditionArr, column+`=?`)
				conditionArgs = append(conditionArgs, jv)
			}
			if len(conditionArr) > 0 {
				conditionStr := `(` + strings.Join(conditionArr, ` AND `) + `)`
				groupArr = append(groupArr, conditionStr)
			}
		default:
		case cgTypeOrGroup, cgTypeAndGroup: // 嵌套分组
			if len(cg.SubGroups) > 0 {
				var subConditions []string
				for _, sub := range cg.SubGroups {
					subOrm := &OrmModel{
						params:   o.params,
						dbFields: o.dbFields,
					}
					subOrm.namedCGArr = []ConditionGroup{sub}
					subSQL, subArgs := subOrm.parseConditionNamed()
					// parseConditionNamed 返回 " WHERE ..." 格式，去掉前缀
					subSQL = strings.TrimPrefix(subSQL, " WHERE ")
					if subSQL != "" {
						subConditions = append(subConditions, subSQL)
						conditionArgs = append(conditionArgs, subArgs...)
					}
				}
				if len(subConditions) > 0 {
					conditionStr := `(` + strings.Join(subConditions, cg.Logic) + `)`
					groupArr = append(groupArr, conditionStr)
				}
			}
		case cgTypeExists, cgTypeNotExists: // EXISTS / NOT EXISTS
			keyword := "EXISTS"
			if cg.cType == cgTypeNotExists {
				keyword = "NOT EXISTS"
			}
			subSQL := cg.Express
			if len(cg.Args) > 0 {
				for i, arg := range cg.Args {
					subSQL = strings.Replace(subSQL, "$"+strconv.Itoa(i+1), "?", 1)
					conditionArgs = append(conditionArgs, arg)
				}
			}
			conditionStr := fmt.Sprintf(`%s (%s)`, keyword, subSQL)
			groupArr = append(groupArr, conditionStr)
		case cgTypeSubQuery: // WHERE 子查询
			column := o.columnField(cg.JsonTags[0])
			if column == "" {
				continue
			}
			subSQL := cg.Express
			if len(cg.Args) > 0 {
				for i, arg := range cg.Args {
					subSQL = strings.Replace(subSQL, "$"+strconv.Itoa(i+1), "?", 1)
					conditionArgs = append(conditionArgs, arg)
				}
			}
			conditionStr := fmt.Sprintf(`%s %s (%s)`, column, cg.Symbol, subSQL)
			groupArr = append(groupArr, conditionStr)
		}
	}
	var conditionSQL string
	if len(groupArr) > 0 {
		conditionSQL = ` WHERE ` + strings.Join(groupArr, and)
	}
	return conditionSQL, conditionArgs
}

// escapeSQL 转义 SQL 字符串中的特殊字符，防止 SQL 注入
func escapeSQL(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

func ValueTypeToStr(v any) string {
	switch val := v.(type) {
	case nil:
		return ""
	case string:
		if len(val) == 0 {
			return "''"
		}
		escaped := escapeSQL(val)
		var sb strings.Builder
		sb.Grow(len(escaped) + 2)
		sb.WriteByte('\'')
		sb.WriteString(escaped)
		sb.WriteByte('\'')
		return sb.String()
	case *string:
		if val == nil {
			return ""
		}
		escaped := escapeSQL(*val)
		var sb strings.Builder
		sb.Grow(len(escaped) + 2)
		sb.WriteByte('\'')
		sb.WriteString(escaped)
		sb.WriteByte('\'')
		return sb.String()
	case int:
		return strconv.Itoa(val)
	case *int:
		if val == nil {
			return ""
		}
		return strconv.Itoa(*val)
	case int64:
		return strconv.FormatInt(val, 10)
	case *int64:
		if val == nil {
			return ""
		}
		return strconv.FormatInt(*val, 10)
	case int32:
		return strconv.FormatInt(int64(val), 10)
	case *int32:
		if val == nil {
			return ""
		}
		return strconv.FormatInt(int64(*val), 10)
	case int16:
		return strconv.FormatInt(int64(val), 10)
	case *int16:
		if val == nil {
			return ""
		}
		return strconv.FormatInt(int64(*val), 10)
	case uint:
		return strconv.FormatUint(uint64(val), 10)
	case *uint:
		if val == nil {
			return ""
		}
		return strconv.FormatUint(uint64(*val), 10)
	case uint64:
		return strconv.FormatUint(val, 10)
	case *uint64:
		if val == nil {
			return ""
		}
		return strconv.FormatUint(*val, 10)
	case uint32:
		return strconv.FormatUint(uint64(val), 10)
	case uint16:
		return strconv.FormatUint(uint64(val), 10)
	case uint8:
		return strconv.FormatUint(uint64(val), 10)
	case float64:
		return strconv.FormatFloat(val, 'f', -1, 64)
	case *float64:
		if val == nil {
			return ""
		}
		return strconv.FormatFloat(*val, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(val), 'f', -1, 32)
	case *float32:
		if val == nil {
			return ""
		}
		return strconv.FormatFloat(float64(*val), 'f', -1, 32)
	case bool:
		return strconv.FormatBool(val)
	case *bool:
		if val == nil {
			return ""
		}
		return strconv.FormatBool(*val)
	case []byte:
		if len(val) == 0 {
			return ""
		}
		// 转义防止 SQL 注入
		escaped := escapeSQL(string(val))
		var sb strings.Builder
		sb.Grow(len(escaped) + 2)
		sb.WriteByte('\'')
		sb.WriteString(escaped)
		sb.WriteByte('\'')
		return sb.String()
	default:
		jsonStr, err := sonic.MarshalString(v)
		if err != nil || jsonStr == "null" {
			return ""
		}
		// 转义防止 SQL 注入
		escaped := escapeSQL(jsonStr)
		var sb strings.Builder
		sb.Grow(len(escaped) + 2)
		sb.WriteByte('\'')
		sb.WriteString(escaped)
		sb.WriteByte('\'')
		return sb.String()
	}
}
