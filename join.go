package mworm

import (
	"fmt"
	"strings"
)

// JoinType 连接类型
type JoinType int

const (
	InnerJoin JoinType = iota // INNER JOIN
	LeftJoin                  // LEFT JOIN
	RightJoin                 // RIGHT JOIN
)

// String 返回 JOIN 类型的字符串表示
func (j JoinType) String() string {
	switch j {
	case InnerJoin:
		return "INNER JOIN"
	case LeftJoin:
		return "LEFT JOIN"
	case RightJoin:
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

// Join 添加连接表
func (o *OrmModel) Join(i ORMInterface) *OrmModel {
	if o.method != methodSelect {
		o.err = fmt.Errorf("JOIN only supports SELECT")
		return o
	}
	//if o.joinTables == nil {
	//	o.joinTables = make([]*JoinTable, 0)
	//}
	//o.joinTables = append(o.joinTables, joinTable)
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
				case cgTypeAndOrNonZero, cgTypeAndOrZero:
					for i, tag := range cond.JsonTags {
						if i > 0 {
							joinSQL.WriteString(cond.Logic)
						}
						conditions = append(conditions, fmt.Sprintf("%s = %s", tag, o.columnField(tag)))
					}
				case cgTypeNamedExpress:
					conditions = append(conditions, cond.Express)
				default:
					panic("unhandled default case")
				}
			}
			joinSQL.WriteString(strings.Join(conditions, " AND "))
		}
	}
	return joinSQL.String()
}

// JoinOn 创建 JOIN ON 条件
func JoinOn(express string) ConditionGroup {
	return ConditionGroup{
		Express: express,
		cType:   cgTypeNamedExpress,
	}
}

// SelectFields 设置需要查询的字段
func (j *JoinTable) SelectFields(fields ...string) *JoinTable {
	j.SelectField = fields
	return j
}
