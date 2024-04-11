package mworm

import (
	"crypto/md5"
	dbsql "database/sql"
	"encoding/hex"
	"fmt"
	"github.com/jmoiron/sqlx"
	jsoniter "github.com/json-iterator/go"
	"github.com/lib/pq"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"reflect"
	"sort"
	"strings"
)

const (
	and = " AND "
	or  = " OR "
	in  = " IN "
)

type ConditionGroup struct {
	Logic        string
	JsonTags     []string
	Args         []any
	InArgs       []string
	NamedExpress string
	IsNull       bool
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
		case reflect.Int:
			for _, arg := range args {
				result = append(result, fmt.Sprintf(`%d`, arg))
			}
		default:
			for _, arg := range args {
				result = append(result, fmt.Sprintf(`'%s'`, arg))
			}
		}
	}
	cg := condition(in, []string{jsonTag}, nil)
	cg.InArgs = result
	return cg
}

// Exp 条件表达式 {table_column_field}=:{name}
func Exp(express string, args ...any) ConditionGroup {
	cg := ConditionGroup{NamedExpress: express, Args: args}
	return cg
}

// ISNull 是否为空 And
func ISNull(jsonTag ...string) ConditionGroup {
	cg := condition(and, jsonTag)
	cg.IsNull = true
	return cg
}

// ISNullOr 是否为空 Or
func ISNullOr(jsonTag ...string) ConditionGroup {
	cg := condition(or, jsonTag)
	cg.IsNull = true
	return cg
}

func (o *OrmModel) Where(cgs ...ConditionGroup) *OrmModel {
	if o.method == methodInsert {
		return o
	}
	for _, cg := range cgs {
		digest := md5.Sum([]byte(strings.Join(cg.JsonTags, "") + cg.NamedExpress))
		o.namedCGs[hex.EncodeToString(digest[:])] = cg
	}
	o.namedExec = true
	return o
}

func (o *OrmModel) parseConditionNamed() string {
	var conditionSQL string
	var groupArr []string
	if len(o.namedCGs) == 0 {
		return ""
	}
	for _, cg := range o.namedCGs {
		if len(cg.JsonTags) > 0 && len(cg.Args) == 0 && cg.Logic != in { //多字段
			var names []string
			for _, f := range cg.JsonTags {
				column := o.columnField(f)
				if len(column) > 0 {
					if cg.IsNull {
						names = append(names, fmt.Sprintf(`%s IS NULL`, column))
					} else {
						names = append(names, fmt.Sprintf(`%s=:%s`, column, f))
					}
				}
			}
			if len(names) > 0 {
				conditionStr := `(` + strings.Join(names, cg.Logic) + `)`
				groupArr = append(groupArr, conditionStr)
			}
		} else if len(cg.JsonTags) == 1 && len(cg.Args) >= 1 { //单字段 | IN
			column := o.columnField(cg.JsonTags[0])
			if len(column) > 0 {
				if cg.Logic != in {
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
				} else { // IN
					conditionStr := fmt.Sprintf(`%s IN (%s)`, column, strings.Join(cg.InArgs, ","))
					groupArr = append(groupArr, conditionStr)
				}
			}
		} else if len(cg.NamedExpress) > 0 && len(cg.Args) > 0 { // 表达式
			//db_column1=:name1 OR db_column2=:name2
			subArr := strings.Split(cg.NamedExpress, ":")
			if len(subArr) > 0 {
				var keys []string
				for _, s := range subArr[1:] {
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
					conditionStr := `(` + cg.NamedExpress + `)`
					groupArr = append(groupArr, conditionStr)
				} else {
					o.err = errors.Errorf("fields and args do not match. exp: %s", cg.NamedExpress)
					return ""
				}
			}
		}
	}
	conditionSQL = ` WHERE ` + strings.Join(groupArr, and)
	return conditionSQL
}

func (o *OrmModel) NamedSQL() string {
	o.namedExec = true
	newParams := make(map[string]interface{})
	for s, i := range o.params {
		newParams[s] = i
	}
	var conditionSQL = o.parseConditionNamed()
	// 排除不参与拼接的 Key
	if len(o.excludeFields) > 0 {
		for k := range o.excludeFields {
			delete(newParams, k)
		}
	}
	// 保留字段
	fieldValueMap := make(map[string]interface{})
	if len(o.requiredFields) > 0 {
		for k := range o.requiredFields {
			if v, ok := newParams[k]; ok {
				fieldValueMap[k] = v
			}
		}
		newParams = fieldValueMap
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
			if o.columnValidate(k, v) {
				nameArr = append(nameArr, fmt.Sprintf(`:%s`, k))
				fieldArr = append(fieldArr, field)
			}
		}
		o.sql = fmt.Sprintf(`%s "%s" (%s) VALUES (%s)%s`, `INSERT INTO`, o.tableName, strings.Join(fieldArr, `, `),
			strings.Join(nameArr, `, `), o.returning)
	case methodUpdate:
		var nameArr []string
		for k, v := range newParams {
			field := o.columnField(k)
			if len(field) == 0 {
				continue
			}
			if o.columnValidate(k, v) {
				nameArr = append(nameArr, fmt.Sprintf(`%s=:%s`, field, k))
			}
		}
		o.sql = fmt.Sprintf(`UPDATE "%s" %s %s%s%s`, o.tableName, `SET`, strings.Join(nameArr, `, `), conditionSQL,
			o.returning)
	case methodSelect:
		fieldArr := make([]string, 0)
		if len(o.requiredFields) == 0 && len(o.requiredFields) == 0 {
			fieldArr = append(fieldArr, "*")
		} else {
			for k := range newParams {
				field := o.columnField(k)
				if len(field) == 0 {
					continue
				}
				fieldArr = append(fieldArr, field)
			}
		}
		o.sql = fmt.Sprintf(`SELECT %s %s "%s"`, strings.Join(fieldArr, `, `), `FROM`, o.tableName)
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
		o.sql = fmt.Sprintf(`%s "%s"%s%s`, `DELETE FROM`, o.tableName, conditionSQL, o.returning)
	}
	if o.log {
		log.Info().Str("sql", o.sql)
	}
	return o.sql
}

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
	switch columnValue := value.(type) {
	case nil:
		return false
	case string:
		if len(columnValue) > 0 || o.emptyKeyExecute {
			return true
		}
	case int, int16, int32, int64, float32, float64, uint, uint8, uint16, uint32, uint64, bool:
		return true
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
		o.params[column] = jsonStr
		return true
	}
	return false
}

func (o *OrmModel) RETURNING(single any, list any, jsonTag ...string) error {
	if SqlxDB.DriverName() != "postgres" {
		panic("RETURNING方法不支持")
	}
	if len(jsonTag) == 0 {
		return errors.New("jsonTag is empty")
	}
	if (single != nil && list != nil) || (single == nil && list == nil) {
		err := errors.New("Choose one from {single} and {list}")
		log.Err(err).Msg("RETURNING")
		return err
	}
	var columnArr []string
	for _, s := range jsonTag {
		column := o.columnField(s)
		if len(column) > 0 {
			columnArr = append(columnArr, column)
		} else if s == "*" {
			columnArr = append(columnArr, "*")
		}
	}
	if len(columnArr) > 0 {
		o.returning = ` RETURNING ` + strings.Join(columnArr, ",")
	}
	if single != nil {
		return o.Get(single)
	}
	return o.List(list)
}

// WherePK WHERE 条件里使用主键进行查询 db:"xx,pk"
func (o *OrmModel) WherePK() *OrmModel {
	o.setPK()
	return o
}

// PK 使用dbTag里包含pk字符的jsonTag
func (o *OrmModel) setPK() *OrmModel {
	if len(o.dbFields[primaryKey]) > 0 {
		o.pk = o.dbFields[primaryKey]
		o.usePK = true
		o.conditionFields[o.pk] = emptyKey{}
		digest := md5.Sum([]byte(o.pk))
		o.namedCGs[hex.EncodeToString(digest[:])] = ConditionGroup{JsonTags: []string{o.pk}}
	}
	return o
}
