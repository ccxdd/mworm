package mworm

import (
	"fmt"
	"strings"
)

// JoinType 连接类型
type JoinType int

const (
	InnerJoinType JoinType = iota // INNER JOIN
	LeftJoinType                  // LEFT JOIN
	RightJoinType                 // RIGHT JOIN
)

// String 返回 JOIN 类型的字符串表示
func (j JoinType) String() string {
	switch j {
	case InnerJoinType:
		return "INNER JOIN"
	case LeftJoinType:
		return "LEFT JOIN"
	case RightJoinType:
		return "RIGHT JOIN"
	default:
		return "INNER JOIN"
	}
}

// JoinTable 连接表的结构
type JoinTable struct {
	Type        JoinType         // JOIN 类型
	Table       string           // 表名
	Alias       string           // 表别名
	Conditions  []ConditionGroup // JOIN 条件
	SelectField []string         // 需要查询的字段
}

// NewJoin 创建一个新的 JoinTable
func NewJoin(joinType JoinType, table, alias string) *JoinTable {
	return &JoinTable{
		Type:  joinType,
		Table: table,
		Alias: alias,
	}
}

// LeftJoin 创建一个 LEFT JOIN
func LeftJoin(table, alias string) *JoinTable {
	return NewJoin(LeftJoinType, table, alias)
}

// RightJoin 创建一个 RIGHT JOIN
func RightJoin(table, alias string) *JoinTable {
	return NewJoin(RightJoinType, table, alias)
}

// InnerJoin 创建一个 INNER JOIN
func InnerJoin(table, alias string) *JoinTable {
	return NewJoin(InnerJoinType, table, alias)
}

// On 添加连接条件
// 推荐使用 mworm.Raw("t.id = a.user_id") 或 mworm.JoinOn("t.id = a.user_id")
func (j *JoinTable) On(cgs ...ConditionGroup) *JoinTable {
	j.Conditions = append(j.Conditions, cgs...)
	return j
}

// Select 指定该连接表需要查询的字段
func (j *JoinTable) Select(fields ...string) *JoinTable {
	j.SelectField = append(j.SelectField, fields...)
	return j
}

// Join 添加连接表到 OrmModel
func (o *OrmModel) Join(join *JoinTable) *OrmModel {
	if o.method != methodSelect {
		o.err = fmt.Errorf("JOIN only supports SELECT")
		return o
	}
	o.joinTables = append(o.joinTables, join)
	return o
}

// parseJoinSQL 解析 JOIN SQL
func (o *OrmModel) parseJoinSQL() string {
	if len(o.joinTables) == 0 {
		return ""
	}

	var joinSQL strings.Builder
	for _, join := range o.joinTables {
		// 构建 JOIN 子句
		joinSQL.WriteString(fmt.Sprintf(" %s %s", join.Type.String(), join.Table))
		if join.Alias != "" {
			joinSQL.WriteString(fmt.Sprintf(" AS %s", join.Alias))
		}

		// 构建 ON 条件
		if len(join.Conditions) > 0 {
			joinSQL.WriteString(" ON ")
			var conditions []string
			for _, cond := range join.Conditions {
				switch cond.cType {
				case cgTypeRaw, cgTypeNamedExpress:
					conditions = append(conditions, cond.Express)
				case cgTypeSymbol: // Eq, Gt, Lt...
					if len(cond.JsonTags) > 0 && len(cond.Args) > 0 {
						val := ValueTypeToStr(cond.Args[0])
						conditions = append(conditions, fmt.Sprintf("%s %s %s", cond.JsonTags[0], cond.Symbol, val))
					}
				case cgTypeAndOr: // And/Or
					// 这里主要处理简单的 k=v 场景，但在 JOIN ON 中较少见，除非是常量条件
					// 暂时只支持简单的相等逻辑
					for i, tag := range cond.JsonTags {
						if i > 0 {
							joinSQL.WriteString(cond.Logic)
						}
						// 注意：这里没有像 Where 那样去映射 dbFields，因为 JOIN 的字段通常是明确的 table.column
						// 如果 Args 存在，则视为 column = value
						if len(cond.Args) > i {
							val := ValueTypeToStr(cond.Args[i])
							conditions = append(conditions, fmt.Sprintf("%s = %s", tag, val))
						}
					}
				default:
					// 其他类型暂不支持，建议使用 Raw
				}
			}
			joinSQL.WriteString(strings.Join(conditions, " AND "))
		}
	}
	return joinSQL.String()
}

// JoinOn 创建 JOIN ON 条件 (实际上是 Raw 的别名，为了语义更清晰)
func JoinOn(express string) ConditionGroup {
	return ConditionGroup{
		Express: express,
		cType:   cgTypeNamedExpress,
	}
}
