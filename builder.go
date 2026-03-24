package mworm

import (
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

func (o *OrmModel) Where(cgs ...ConditionGroup) *OrmModel {
	if o.method == methodInsert {
		return o
	}
	o.namedCGArr = append(o.namedCGArr, cgs...)
	return o
}

// OnConflict 指定 Upsert 冲突列（json tag），用于 INSERT ON CONFLICT
func (o *OrmModel) OnConflict(jsonTags ...string) *OrmModel {
	o.conflictColumns = jsonTags
	return o
}

// DoUpdate 冲突时更新指定字段（json tag），需先调用 OnConflict
func (o *OrmModel) DoUpdate(jsonTags ...string) *OrmModel {
	o.conflictUpdate = jsonTags
	return o
}

// DoNothing 冲突时不做任何操作，需先调用 OnConflict
func (o *OrmModel) DoNothing() *OrmModel {
	o.conflictDoNothing = true
	return o
}

// BuildSQL 构造带命名参数的 SQL 语句
func (o *OrmModel) BuildSQL() SQLParams {
	o.args = make([]any, 0)
	newParams := make(map[string]interface{}, len(o.params))
	for s, i := range o.params {
		newParams[s] = i
	}

	// 排除不参与拼接的 Key
	if len(o.excludeFields) > 0 {
		for k := range o.excludeFields {
			delete(newParams, k)             // JSON key
			delete(newParams, o.dbFields[k]) // column key
		}
	}
	// 保留字段
	if len(o.requiredFields) > 0 {
		fieldValueMap := make(map[string]interface{}, len(o.requiredFields))
		for k := range o.requiredFields {
			if v, ok := newParams[k]; ok {
				fieldValueMap[k] = v
			}
		}
		newParams = fieldValueMap
	} else if o.method == methodUpdate && len(o.updateExpressions) > 0 {
		newParams = make(map[string]interface{})
	}

	// 增删改查
	switch o.method {
	case methodInsert:
		fieldArr := make([]string, 0, len(newParams))
		placeholderArr := make([]string, 0, len(newParams))

		var keys []string
		if len(o.dbFields) > 0 {
			for _, k := range o.dbFields {
				keys = append(keys, k)
			}
		} else {
			for k := range newParams {
				keys = append(keys, k)
			}
			sort.Strings(keys)
		}

		for _, k := range keys {
			v, exists := newParams[k]
			if !exists {
				found := false
				for origK, mappedCol := range o.dbFields {
					if mappedCol == k {
						if vv, ok := newParams[origK]; ok {
							v = vv
							exists = true
							found = true
							break
						}
					}
				}
				if !found {
					continue
				}
			}

			field := o.columnField(k)
			if len(field) == 0 {
				continue
			}

			if o.columnValidate(field, v) {
				placeholderArr = append(placeholderArr, "?")
				fieldArr = append(fieldArr, field)
				o.args = append(o.args, v)
			}
		}

		var sb strings.Builder
		sb.Grow(64 + len(o.tableName) + len(fieldArr)*10)
		sb.WriteString("INSERT INTO ")
		sb.WriteString(o.tableName)
		sb.WriteString(" (")
		sb.WriteString(strings.Join(fieldArr, ", "))
		sb.WriteString(") VALUES (")
		sb.WriteString(strings.Join(placeholderArr, ", "))
		sb.WriteByte(')')
		// ON CONFLICT 子句
		if len(o.conflictColumns) > 0 {
			var conflictCols []string
			for _, tag := range o.conflictColumns {
				col := o.columnField(tag)
				if col != "" {
					conflictCols = append(conflictCols, col)
				}
			}
			if len(conflictCols) > 0 {
				sb.WriteString(fmt.Sprintf(" ON CONFLICT (%s)", strings.Join(conflictCols, ", ")))
				if o.conflictDoNothing {
					sb.WriteString(" DO NOTHING")
				} else if len(o.conflictUpdate) > 0 {
					var updatePairs []string
					for _, tag := range o.conflictUpdate {
						col := o.columnField(tag)
						if col != "" {
							updatePairs = append(updatePairs, fmt.Sprintf("%s=EXCLUDED.%s", col, col))
						}
					}
					if len(updatePairs) > 0 {
						sb.WriteString(" DO UPDATE SET ")
						sb.WriteString(strings.Join(updatePairs, ", "))
					}
				}
			}
		}
		sb.WriteString(o.returning)
		o.sql = sb.String()
	case methodUpdate:
		var keys []string
		if len(o.dbFields) > 0 {
			for _, k := range o.dbFields {
				keys = append(keys, k)
			}
		} else {
			for k := range newParams {
				keys = append(keys, k)
			}
			sort.Strings(keys)
		}

		nameArr := make([]string, 0, len(keys))
		for _, k := range keys {
			v, exists := newParams[k]
			if !exists {
				found := false
				for origK, mappedCol := range o.dbFields {
					if mappedCol == k {
						if vv, ok := newParams[origK]; ok {
							v = vv
							exists = true
							found = true
							break
						}
					}
				}
				if !found {
					continue
				}
			}

			field := o.columnField(k)
			if len(field) == 0 {
				continue
			}
			if o.columnValidate(field, v) {
				var setPair strings.Builder
				setPair.WriteString(field)
				setPair.WriteString("=?")
				nameArr = append(nameArr, setPair.String())
				o.args = append(o.args, v)
			}
		}
		if len(o.updateExpressions) > 0 {
			nameArr = append(nameArr, o.updateExpressions...)
		}

		// 这里处理条件参数顺序
		conditionSQL, conditionArgs := o.parseConditionNamed()
		o.args = append(o.args, conditionArgs...)

		var sb strings.Builder
		sb.Grow(32 + len(o.tableName) + len(conditionSQL) + len(nameArr)*15)
		sb.WriteString("UPDATE ")
		sb.WriteString(o.tableName)
		sb.WriteString(" SET ")
		sb.WriteString(strings.Join(nameArr, ", "))
		sb.WriteString(conditionSQL)
		sb.WriteString(o.returning)
		o.sql = sb.String()
	case methodSelect:
		fieldArr := make([]string, 0)
		if len(o.requiredFields) == 0 && len(o.excludeFields) == 0 {
			if len(o.joinTables) > 0 {
				fieldArr = append(fieldArr, "t.*")
				for _, join := range o.joinTables {
					if len(join.SelectField) > 0 {
						for _, field := range join.SelectField {
							if join.Alias != "" {
								fieldArr = append(fieldArr, fmt.Sprintf("%s.%s", join.Alias, field))
							} else {
								fieldArr = append(fieldArr, fmt.Sprintf("%s.%s", join.Table, field))
							}
						}
					}
				}
			} else {
				fieldArr = append(fieldArr, "*")
			}
		} else {
			uniqueFields := make(map[string]struct{}, len(newParams))
			for k := range newParams {
				field := o.columnField(k)
				if len(field) == 0 {
					continue
				}
				if _, ok := uniqueFields[field]; !ok {
					fieldArr = append(fieldArr, field)
					uniqueFields[field] = struct{}{}
				}
			}
		}

		conditionSQL, conditionArgs := o.parseConditionNamed()
		o.args = append(o.args, conditionArgs...)

		var tmpSql strings.Builder
		if len(o.joinTables) > 0 {
			tmpSql.WriteString(fmt.Sprintf(`SELECT %s%s FROM %s t%s`, o.distinct, strings.Join(fieldArr, `, `),
				o.tableName, o.parseJoinSQL()))
		} else {
			if o.groupBy {
				g := strings.Join(fieldArr, `, `)
				if len(o.groupByRaw) > 0 {
					g += `, ` + o.groupByRaw
				}
				tmpSql.WriteString(fmt.Sprintf(`SELECT %s FROM %s`, g, o.tableName))
			} else {
				distinctPart := ""
				if o.distinct != "" {
					distinctPart = o.distinct + " "
				}
				tmpSql.WriteString(fmt.Sprintf(`SELECT %s%s FROM %s`, distinctPart, strings.Join(fieldArr, `, `),
					o.tableName))
			}
		}

		tmpSql.WriteString(conditionSQL)
		if o.groupBy {
			tmpSql.WriteString(` GROUP BY ` + strings.Join(fieldArr, `,`))
			if len(o.havingRaw) > 0 {
				tmpSql.WriteString(` HAVING ` + o.havingRaw)
			}
		}
		if len(o.orderFields) > 0 {
			tmpSql.WriteString(` ORDER BY ` + strings.Join(o.orderFields, `,`))
		}
		if o.limit > 0 {
			tmpSql.WriteString(fmt.Sprintf(` LIMIT %d`, o.limit))
		}
		if o.offset > 0 {
			tmpSql.WriteString(fmt.Sprintf(` OFFSET %d`, o.offset))
		}
		o.sql = tmpSql.String()
	case methodDelete:
		conditionSQL, conditionArgs := o.parseConditionNamed()
		o.args = append(o.args, conditionArgs...)
		o.sql = fmt.Sprintf(`%s %s %s%s`, `DELETE FROM`, o.tableName, conditionSQL, o.returning)
	}

	if o.log || DebugMode {
		log.Debug().Str("sql", o.sql).Msg("BuildSQL")
	}
	if len(o.withTable) > 0 {
		o.withSQL = fmt.Sprintf(`WITH %s AS (%s)`, o.withTable, o.sql)
	}
	return SQLParams{
		Sql:     o.sql,
		WithSql: o.withSQL,
		Params:  o.params,
		Args:    o.args,
		Err:     o.err,
	}
}

// FullSQL SQL+WithSQL
func (o *OrmModel) FullSQL() SQLParams {
	sqlParams := o.BuildSQL()
	if len(o.withSQL) > 0 {
		var orderBy string
		if len(o.withOrderFields) > 0 {
			orderBy = fmt.Sprintf(`ORDER BY %s`, strings.Join(o.withOrderFields, ","))
		}
		sqlParams.Sql = fmt.Sprintf(`%s SELECT * FROM %s %s`, o.withSQL, o.withTable, orderBy)
	}
	return sqlParams
}

// buildExeSql 将含 ? 占位符的 SQL 与参数列表合并，生成可直接执行的完整 SQL 字符串。
// 仅用于日志/调试或需要直接传字符串给 tx.Exec 的场景，生产写库请优先使用 Sql+Args 形式。
func buildExeSql(sql string, args []any) string {
	if len(args) == 0 {
		return sql
	}
	var sb strings.Builder
	sb.Grow(len(sql) + len(args)*8)
	length := len(sql)
	argIdx := 0
	for i := 0; i < length; i++ {
		if sql[i] == '?' && argIdx < len(args) {
			sb.WriteString(ValueTypeToStr(args[argIdx]))
			argIdx++
		} else {
			sb.WriteByte(sql[i])
		}
	}
	return sb.String()
}

func O() *OrmModel {
	return &OrmModel{}
}

func Query(query string, dest any) error {
	var rows *sqlx.Rows
	var err error
	rows, err = SqlxDB.Queryx(query)
	if err != nil {
		return err
	}
	return rowsMapScan(rows, dest)
}

func rowsMapScan(rows *sqlx.Rows, dest any) error {
	o := O()
	fieldMap := make(map[string]interface{})
	defer func() { _ = rows.Close() }()
	if rows.Next() {
		if o.err = rows.MapScan(fieldMap); o.err != nil {
			return o.err
		}
	}
	t := reflect.TypeOf(dest)
	if t.Kind() != reflect.Ptr {
		o.err = errors.New(`error: t.Kind() != reflect.Prt`)
	} else {
		t = t.Elem()
	}
	v := reflect.ValueOf(dest)
	v = reflect.Indirect(v)
	o.err = o.bindRow(t, v, fieldMap)
	return o.err
}

// 对列值进行校验是否可以执行 INSERT ｜ UPDATE
func (o *OrmModel) columnValidate(column string, value any) bool {
	_, allowEmpty := o.emptyUpdateFields[column]
	switch columnValue := value.(type) {
	case nil:
		return false
	case string:
		if len(columnValue) > 0 || allowEmpty {
			return true
		}
	case int, int16, int32, int64, float32, float64, uint, uint8, uint16, uint32, uint64, bool:
		if !isZeroValue(columnValue) || allowEmpty {
			return true
		}
	//case map[string]interface{}:
	case []byte:
		if len(columnValue) > 0 {
			o.params[column] = string(columnValue)
			return true
		}
	default:
		jsonStr, err := sonic.MarshalString(columnValue)
		if err != nil {
			fmt.Printf("error: methodInsert not processed, because value: %v\n", columnValue)
			return false
		}
		if jsonStr == `null` {
			return false
		}
		o.params[column] = jsonStr
		return true
	}
	return false
}

// isZeroValue 判断数值/布尔类型是否为零值（零分配，替代 fmt.Sprintf 判零）
func isZeroValue(v any) bool {
	switch val := v.(type) {
	case int:
		return val == 0
	case int16:
		return val == 0
	case int32:
		return val == 0
	case int64:
		return val == 0
	case float32:
		return val == 0
	case float64:
		return val == 0
	case uint:
		return val == 0
	case uint8:
		return val == 0
	case uint16:
		return val == 0
	case uint32:
		return val == 0
	case uint64:
		return val == 0
	case bool:
		return !val
	default:
		return false
	}
}

func (o *OrmModel) RETURNING(single any, list any, jsonTag ...string) error {
	if SqlxDB.DriverName() != "postgres" || SqlxDB.DriverName() != "pgx" {
		panic("RETURNING方法不支持")
	}
	if (single != nil && list != nil) || (single == nil && list == nil) {
		err := errors.New("Choose one from {single} and {list}")
		log.Err(err).Msg("RETURNING")
		return err
	}
	var columnArr []string
	for _, j := range jsonTag {
		column := o.columnField(j)
		if len(column) > 0 {
			columnArr = append(columnArr, column)
		}
	}
	if len(columnArr) == 0 {
		columnArr = append(columnArr, "*")
	}
	if len(columnArr) > 0 {
		o.returning = ` RETURNING ` + strings.Join(columnArr, ",")
	}
	if single != nil {
		return o.One(single)
	}
	return o.Many(list)
}

// WherePK 使用dbTag里包含pk字符的jsonTag的字段进行查询。 db:"columnName,pk"
func (o *OrmModel) WherePK() *OrmModel {
	if len(o.pk) > 0 {
		o.conditionFields[o.pk] = emptyKey{}
		if o.method == methodUpdate {
			o.excludeFields[o.pk] = emptyKey{}
		}
		o.namedCGArr = append(o.namedCGArr, ConditionGroup{JsonTags: []string{o.pk}, cType: cgTypeAndOr})
	}
	return o
}

// SetField UPDATE 设置字段值
func (o *OrmModel) SetField(jsonTag string, arg any) *OrmModel {
	var expression string
	column := o.columnField(jsonTag)
	if len(column) > 0 {
		delete(o.requiredFields, column)
		switch t := arg.(type) {
		case string:
			// 转义防止 SQL 注入
			expression = fmt.Sprintf(`%s='%s'`, column, strings.ReplaceAll(t, "'", "''"))
		case nil:
			expression = fmt.Sprintf(`%s=NULL`, column)
		default:
			expression = fmt.Sprintf(`%s=%v`, column, t)
		}
		o.updateExpressions = append(o.updateExpressions, expression)
	}
	return o
}

func ConvertArray[T int | string](array []T) []string {
	var result []string
	if len(array) > 0 {
		i := array[0]
		t := reflect.TypeOf(i)
		switch t.Kind() {
		case reflect.String:
			for _, arg := range array {
				// 转义防止 SQL 注入
				escaped := strings.ReplaceAll(fmt.Sprintf(`%v`, arg), "'", "''")
				result = append(result, fmt.Sprintf(`'%s'`, escaped))
			}
		default:
			for _, arg := range array {
				result = append(result, fmt.Sprintf(`%v`, arg))
			}
		}
	}
	return result
}
