package mworm

import (
	"crypto/md5"
	dbsql "database/sql"
	"encoding/hex"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/jmoiron/sqlx"
	jsoniter "github.com/json-iterator/go"
	"github.com/lib/pq"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

func (o *OrmModel) Where(cgs ...ConditionGroup) *OrmModel {
	if o.method == methodInsert {
		return o
	}
	for _, cg := range cgs {
		digest := md5.Sum([]byte(strings.Join(cg.JsonTags, "") + cg.Express + cg.Logic + fmt.Sprintf(`%v`, cg.cType)))
		o.namedCGArr[hex.EncodeToString(digest[:])] = cg
	}
	o.namedExec = true
	return o
}

// BuildSQL 构造带命名参数的 SQL 语句
func (o *OrmModel) BuildSQL() SQLParams {
	o.namedExec = true
	newParams := make(map[string]interface{})
	fieldValueMap := make(map[string]interface{})
	for s, i := range o.params {
		newParams[s] = i
	}
	var conditionSQL = o.parseConditionNamed()
	if o.err != nil {
		return SQLParams{Err: o.err}
	}
	// 排除不参与拼接的 Key
	if len(o.excludeFields) > 0 {
		for k := range o.excludeFields {
			delete(newParams, k)
		}
	}
	// 保留字段
	if len(o.requiredFields) > 0 {
		for k := range o.requiredFields {
			if v, ok := newParams[k]; ok {
				fieldValueMap[k] = v
			}
		}
		newParams = fieldValueMap
	} else if o.method == methodUpdate && len(o.updateFields) > 0 {
		newParams = make(map[string]interface{})
	}
	// 增删改查
	switch o.method {
	case methodInsert:
		var fieldArr, nameArr []string
		for k, v := range newParams {
			field := o.columnField(k)
			if len(field) == 0 {
				continue
			}
			if o.columnValidate(field, v) {
				vStr := ValueTypeToStr(v)
				if vStr == "" {
					continue
				}
				nameArr = append(nameArr, fmt.Sprintf(`%s`, vStr))
				fieldArr = append(fieldArr, field)
			}
		}
		o.sql = fmt.Sprintf(`%s %s (%s) VALUES (%s)%s`, `INSERT INTO`, o.tableName, strings.Join(fieldArr, `, `),
			strings.Join(nameArr, `, `), o.returning)
	case methodUpdate:
		var nameArr []string
		for k, v := range newParams {
			field := o.columnField(k)
			if len(field) == 0 {
				continue
			}
			if o.columnValidate(field, v) {
				vStr := ValueTypeToStr(v)
				if vStr == "" {
					continue
				}
				nameArr = append(nameArr, fmt.Sprintf(`%s=%s`, field, vStr))
			}
		}
		if len(o.updateFields) > 0 {
			nameArr = append(nameArr, o.updateFields...)
		}
		o.sql = fmt.Sprintf(`UPDATE %s %s %s%s%s`, o.tableName, `SET`, strings.Join(nameArr, `, `), conditionSQL,
			o.returning)
	case methodSelect:
		fieldArr := make([]string, 0)
		if len(o.requiredFields) == 0 && len(o.excludeFields) == 0 {
			if len(o.joinTables) > 0 {
				// 对于 JOIN 查询，给主表添加别名 t
				fieldArr = append(fieldArr, "t.*")
				// 添加 JOIN 表的字段
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
			for k := range newParams {
				field := o.columnField(k)
				if len(field) == 0 {
					continue
				}
				fieldArr = append(fieldArr, field)
			}
		}

		if len(o.joinTables) > 0 {
			// 构建 JOIN SQL
			o.sql = fmt.Sprintf(`SELECT %s FROM %s t %s`,
				strings.Join(fieldArr, `, `),
				o.tableName,
				o.parseJoinSQL())
		} else {
			o.sql = fmt.Sprintf(`SELECT %s FROM %s`,
				strings.Join(fieldArr, `, `),
				o.tableName)
		}

		o.sql += conditionSQL
		if len(o.orderFields) > 0 {
			o.sql += ` ORDER BY ` + strings.Join(o.orderFields, `,`)
		}
		if o.limit > 0 {
			o.sql += fmt.Sprintf(` LIMIT %d`, o.limit)
		}
		if o.offset > 0 {
			o.sql += fmt.Sprintf(` OFFSET %d`, o.offset)
		}
	case methodDelete:
		o.sql = fmt.Sprintf(`%s %s %s%s`, `DELETE FROM`, o.tableName, conditionSQL, o.returning)
	}
	if o.log {
		log.Info().Str("sql", o.sql)
		fmt.Println("sql:", o.sql)
	}
	// WITH
	if len(o.withTable) > 0 {
		o.withSQL = fmt.Sprintf(`WITH %s AS (%s)`, o.withTable, o.sql)
	}
	return SQLParams{
		Sql:     o.sql,
		WithSql: o.withSQL,
		Params:  o.params,
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

// NamedExec 执行带命名参数的 SQL 语句
func NamedExec(sqlStr string, params map[string]interface{}) error {
	var err error
	var result dbsql.Result
	f := func() {
		var count int64
		if SqlxDB == nil {
			err = errors.New(`SqlxDB *sqlx.DB is nil`)
		}
		defer func() {
			if e := recover(); e != nil {
				err = errors.New(e.(*pq.Error).Message)
				log.Error().Msg(e.(*pq.Error).Message)
			}
		}()
		result, err = SqlxDB.NamedExec(sqlStr, params)
		if err != nil {
			return
		}
		count, err = result.RowsAffected()
		if count == 0 && err == nil {
			err = errors.New(`影响行数为0`)
		}
	}
	f()
	return err
}

func O() *OrmModel {
	return &OrmModel{}
}

// NamedQuery 执行带命名参数的 SQL 查询并映射结果
func NamedQuery(query string, params any, dest any) error {
	fieldMap, _ := StructToMap(params)
	keys := make([]string, 0, len(fieldMap))
	for k := range fieldMap {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return len(keys[i]) > len(keys[j])
	})
	for _, k := range keys {
		name := fmt.Sprintf(`:%s`, k)
		v := fieldMap[k]
		newValue := ""
		switch v.(type) {
		case string:
			newValue = fmt.Sprintf(`'%v'`, v)
		default:
			newValue = fmt.Sprintf("%v", v)
		}
		if len(newValue) == 0 {
			continue
		}
		query = strings.ReplaceAll(query, name, newValue)
	}
	return Query(query, dest)
}

// NamedQueryWithMap 执行带命名参数的 SQL 查询并映射结果
func NamedQueryWithMap(query string, fieldMap map[string]any, dest any) error {
	keys := make([]string, 0, len(fieldMap))
	for k := range fieldMap {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return len(keys[i]) > len(keys[j])
	})
	for _, k := range keys {
		name := fmt.Sprintf(`:%s`, k)
		v := fieldMap[k]
		newValue := ""
		switch v.(type) {
		case string:
			newValue = fmt.Sprintf(`'%v'`, v)
		default:
			newValue = fmt.Sprintf("%v", v)
		}
		if len(newValue) == 0 {
			continue
		}
		query = strings.ReplaceAll(query, name, newValue)
	}
	return Query(query, dest)
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
		if fmt.Sprintf(`%v`, columnValue) != "0" || allowEmpty {
			return true
		}
	//case map[string]interface{}:
	case []byte:
		if len(columnValue) > 0 {
			o.params[column] = string(columnValue)
			return true
		}
	default:
		jsonStr, err := jsoniter.MarshalToString(columnValue)
		if err != nil {
			fmt.Println(fmt.Sprintf("error: methodInsert not processed, because value: %v", columnValue))
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

func (o *OrmModel) RETURNING(single any, list any, jsonTag ...string) error {
	if SqlxDB.DriverName() != "postgres" {
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

// WherePK WHERE 条件里使用主键进行查询 db:"columnName,pk"
func (o *OrmModel) WherePK() *OrmModel {
	o.setPK()
	return o
}

// PK 使用dbTag里包含pk字符的jsonTag
func (o *OrmModel) setPK() *OrmModel {
	if len(o.pk) > 0 {
		o.conditionFields[o.pk] = emptyKey{}
		if o.method == methodUpdate {
			o.excludeFields[o.pk] = emptyKey{}
		}
		digest := md5.Sum([]byte(o.pk))
		o.namedCGArr[hex.EncodeToString(digest[:])] = ConditionGroup{JsonTags: []string{o.pk}, cType: cgTypeAndOrNonZero}
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
			expression = fmt.Sprintf(`%s='%s'`, column, t)
		case nil:
			expression = fmt.Sprintf(`%s=NULL`, column)
		default:
			expression = fmt.Sprintf(`%s=%v`, column, t)
		}
		o.updateFields = append(o.updateFields, expression)
	}
	return o
}

func RawNamedSQL(sql string, params any) *OrmModel {
	o := RawSQL(sql)
	o.setMethod(o.method, params)
	if params != nil {
		o.namedExec = true
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
				result = append(result, fmt.Sprintf(`'%v'`, arg))
			}
		default:
			for _, arg := range array {
				result = append(result, fmt.Sprintf(`%v`, arg))
			}
		}
	}
	return result
}
