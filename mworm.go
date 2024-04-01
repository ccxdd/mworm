package mworm

import (
	dbsql "database/sql"
	"errors"
	"fmt"
	utilsgo "github.com/ccxdd/utils-go"
	"github.com/jmoiron/sqlx"
	"github.com/jmoiron/sqlx/reflectx"
	jsoniter "github.com/json-iterator/go"
	"github.com/lib/pq"
	"github.com/rs/zerolog/log"
	"reflect"
	"strconv"
	"strings"
	"time"
)

type ORMInterface interface {
	TableName() string
}

type CRUDInterface interface {
	CRUDMode(mode string) string
}

const (
	methodInsert = "INSERT"
	methodUpdate = "UPDATE"
	methodSelect = "SELECT"
	methodDelete = "DELETE"
	//主键
	primaryKey = "primaryKey"
)

var (
	SqlxDB  *sqlx.DB
	TagName = "db"
)

type emptyKey = struct{}

type OrmModel struct {
	// 结构体 Key
	params map[string]interface{}
	// 数据库字段
	dbFields map[string]string
	// 表名
	tableName string
	// 条件字段
	conditionFields map[string]emptyKey
	// 排序字段
	orderFields []string
	// 排除字段
	excludeFields map[string]emptyKey
	// 必选字段
	requiredFields map[string]emptyKey
	// SQL 操作方式
	method string
	// SQL 语句
	sql string
	// 值为空时是否参与 Insert / Update 操作
	emptyKeyExecute bool
	// 错误提示
	err error
	// tag 索引缓存
	tagIndexCache map[string]int
	limit         int64
	offset        int64
	// true 时输出 log
	log bool
	// with 表名
	withTable string
	// with SQL
	withSQL string
	// 子查询排序字段
	subOrderFields []string
	// WhereCG 条件数组
	namedCGs map[string]ConditionGroup
	// 是否使用了:name变量执行SQL
	namedExec bool
	// PQ:专用 RETURNING 语句
	returning string
	usePK     bool
	pk        string
}

func BindDB(DB *sqlx.DB) error {
	SqlxDB = DB
	return SqlxDB.Ping()
}

func Table(name string) *OrmModel {
	o := &OrmModel{}
	o.init()
	o.tableName = name
	return o
}

func BatchArray(ormArray []*OrmModel) error {
	tx := SqlxDB.MustBegin()
	defer func() { _ = tx.Rollback() }()
	for _, i := range ormArray {
		var result dbsql.Result
		result, _ = tx.NamedExec(i.NamedSQL(), i.params)
		if _, err := result.RowsAffected(); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func Batch(ormArray ...*OrmModel) error {
	tx := SqlxDB.MustBegin()
	defer func() { _ = tx.Rollback() }()
	for _, i := range ormArray {
		var result dbsql.Result
		result, _ = tx.NamedExec(i.NamedSQL(), i.params)
		if _, err := result.RowsAffected(); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func BatchFunc(f func(tx *sqlx.Tx)) error {
	if f == nil {
		return nil
	}
	tx := SqlxDB.MustBegin()
	defer func() { _ = tx.Rollback() }()
	f(tx)
	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func SELECT(i ORMInterface) *OrmModel {
	return Table(i.TableName()).setMethod(methodSelect, i)
}

func INSERT(i ORMInterface) *OrmModel {
	return Table(i.TableName()).setMethod(methodInsert, i)
}

func UPDATE(i ORMInterface) *OrmModel {
	return Table(i.TableName()).setMethod(methodUpdate, i)
}

func DELETE(i ORMInterface) *OrmModel {
	return Table(i.TableName()).setMethod(methodDelete, i)
}

func RawSQL(sql string, args ...interface{}) error {
	var err error
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
		count, err = SqlxDB.MustExec(sql, args...).RowsAffected()
		if count == 0 && err == nil {
			err = errors.New(`影响行数为0`)
		}
	}
	f()
	return err
}

func (o *OrmModel) init() {
	o.requiredFields = make(map[string]emptyKey)
	o.excludeFields = make(map[string]emptyKey)
	o.conditionFields = make(map[string]emptyKey)
	o.namedCGs = make(map[string]ConditionGroup)
}

func (o *OrmModel) Select(i interface{}) *OrmModel {
	return o.setMethod(methodSelect, i)
}

func (o *OrmModel) Insert(i interface{}) *OrmModel {
	return o.setMethod(methodInsert, i)
}

func (o *OrmModel) Update(i interface{}) *OrmModel {
	return o.setMethod(methodUpdate, i)
}

func (o *OrmModel) Delete(i interface{}) *OrmModel {
	return o.setMethod(methodDelete, i)
}

func (o *OrmModel) setMethod(method string, i interface{}) *OrmModel {
	o.params, o.dbFields = StructToMap(i)
	o.method = method
	crud, b := i.(CRUDInterface)
	if b {
		crud.CRUDMode(method)
	}
	return o
}

func (o *OrmModel) Desc(jsonTag ...string) *OrmModel {
	for _, f := range jsonTag {
		if len(f) > 0 {
			o.orderFields = append(o.orderFields, f+` DESC`)
		}
	}
	return o
}

func (o *OrmModel) Asc(jsonTag ...string) *OrmModel {
	for _, f := range jsonTag {
		if len(f) > 0 {
			o.orderFields = append(o.orderFields, f)
		}
	}
	return o
}

func (o *OrmModel) EmptyKey(f bool) *OrmModel {
	o.emptyKeyExecute = f
	return o
}

func (o *OrmModel) Exclude(jsonTag ...string) *OrmModel {
	for _, s := range jsonTag {
		o.excludeFields[s] = emptyKey{}
	}
	return o
}

func (o *OrmModel) If(ifFunc func(o *OrmModel)) *OrmModel {
	if ifFunc != nil {
		ifFunc(o)
	}
	return o
}

func (o *OrmModel) Fields(jsonTag ...string) *OrmModel {
	for _, s := range jsonTag {
		o.requiredFields[s] = emptyKey{}
	}
	return o
}

func (o *OrmModel) Limit(row int64) *OrmModel {
	o.limit = row
	return o
}

func (o *OrmModel) Offset(row int64) *OrmModel {
	o.offset = row
	return o
}

func (o *OrmModel) Log(l bool) *OrmModel {
	o.log = l
	return o
}

func (o *OrmModel) WithAsc(fields ...string) *OrmModel {
	for _, f := range fields {
		if len(f) > 0 {
			o.subOrderFields = append(o.subOrderFields, fmt.Sprintf(`%s.%s`, o.withTable, f))
		}
	}
	return o
}

func (o *OrmModel) WithDesc(fields ...string) *OrmModel {
	for _, f := range fields {
		if len(f) > 0 {
			o.subOrderFields = append(o.subOrderFields, fmt.Sprintf(`%s.%s DESC`, o.withTable, f))
		}
	}
	return o
}

func (o *OrmModel) SQL() string {
	if len(o.withSQL) > 0 {
		return o.subSQL()
	}
	newParams := make(map[string]interface{})
	for s, i := range o.params {
		newParams[s] = i
	}
	var conditionSQL = o.whereSQL()
	// 排除不参与拼接的 Key
	if len(o.excludeFields) > 0 {
		for k := range o.excludeFields {
			delete(newParams, k)
		}
	}
	// 保留字段
	fieldMap := make(map[string]interface{})
	if len(o.requiredFields) > 0 {
		for k := range o.requiredFields {
			if v, ok := newParams[k]; ok {
				fieldMap[k] = v
			}
		}
		newParams = fieldMap
	}
	// 增删改查
	switch o.method {
	case methodInsert:
		var keyArr, valueArr []string
		for k, v := range newParams {
			var fieldValue string
			field := o.columnField(k)
			if len(field) == 0 {
				continue
			}
			switch v.(type) {
			case string:
				fieldValue = valToString(v, `'%s'`)
			default:
				fieldValue = valToString(v, ``)
			}
			if (len(fieldValue) > 0 && fieldValue != `''`) || o.emptyKeyExecute {
				keyArr = append(keyArr, field)
				valueArr = append(valueArr, fieldValue)
			}
		}
		o.sql = fmt.Sprintf(`%s "%s" (%s) VALUES (%s)%s`, `INSERT INTO`, o.tableName, strings.Join(keyArr, `, `),
			strings.Join(valueArr, `, `), o.returning)
	case methodUpdate:
		var valueArr []string
		for k, v := range newParams {
			var fieldValue, conditionValue string
			field := o.columnField(k)
			if len(field) == 0 {
				continue
			}
			switch v.(type) {
			case string, *string:
				fieldValue = valToString(v, `'%s'`)
				conditionValue = fmt.Sprintf(`%s = %s`, field, fieldValue)
			default:
				fieldValue = valToString(v, ``)
				conditionValue = fmt.Sprintf(`%s = %s`, field, fieldValue)
			}
			if (len(fieldValue) > 0 && fieldValue != `''`) || o.emptyKeyExecute {
				valueArr = append(valueArr, conditionValue)
			}
		}
		o.sql = fmt.Sprintf(`UPDATE "%s" %s %s%s%s`, o.tableName, `SET`, strings.Join(valueArr, `, `), conditionSQL,
			o.returning)
	case methodSelect:
		fieldArr := make([]string, 0)
		for k := range newParams {
			field := o.columnField(k)
			if len(field) == 0 {
				continue
			}
			fieldArr = append(fieldArr, field)
		}
		if len(fieldArr) == 0 {
			fieldArr = append(fieldArr, "*")
		} else if len(fieldArr) == 1 && o.usePK {
			fieldArr[0] = "*"
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

func (o *OrmModel) whereSQL() string {
	where := o.parseConditionNamed()
	return where
}

// Exec 执行由 OrmModel 生成的 SQL 查询，并在出现错误时返回错误。
//
// 该函数不接受任何参数。
// 它返回一个错误。
func (o *OrmModel) Exec() error {
	o.namedExec = true
	sql := o.NamedSQL()
	o.err = NamedExec(sql, o.params)
	//sql := o.SQL()
	//o.err = RawSQL(sql)
	return o.err
}

func (o *OrmModel) Count(column string) (int64, error) {
	var result int64
	sql := fmt.Sprintf(`SELECT count(%s) %s "%s" %s`, column, `FROM`, o.tableName, o.whereSQL())
	if o.log {
		log.Info().Str("sql", o.sql)
	}
	var rows *sqlx.Rows
	rows, o.err = SqlxDB.NamedQuery(sql, o.params)
	defer func() { _ = rows.Close() }()
	if o.err != nil {
		return 0, o.err
	}
	if rows.Next() {
		o.err = rows.Scan(&result)
	}
	return result, o.err
}

func (o *OrmModel) Get(dest interface{}) error {
	if SqlxDB == nil {
		o.err = errors.New(`SqlxDB *sqlx.DB is nil`)
		return o.err
	}
	fieldMap := make(map[string]interface{})
	var rows *sqlx.Rows
	rows, o.err = SqlxDB.NamedQuery(o.Limit(1).NamedSQL(), o.params)
	defer func() { _ = rows.Close() }()
	if o.err != nil {
		return o.err
	}
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
	_ = o.bindRow(t, v, fieldMap)
	return o.err
}

func (o *OrmModel) List(dest interface{}) error {
	if SqlxDB == nil {
		o.err = errors.New(`SqlxDB *sqlx.DB is nil`)
		return o.err
	}
	if o.method != methodSelect && len(o.returning) == 0 {
		o.err = errors.New(`o.method must be [methodSelect]`)
		return o.err
	}
	// 目标类型
	destValue := reflect.ValueOf(dest)
	if destValue.IsNil() {
		return errors.New("nil pointer passed to StructScan destination")
	}
	if destValue.Kind() != reflect.Ptr {
		o.err = errors.New(`error: Kind() != reflect.Prt`)
		return o.err
	} else {
		if destValue.Elem().Kind() != reflect.Slice {
			o.err = errors.New(`error: Kind() != reflect.Slice`)
			return o.err
		}
	}

	var rowType reflect.Type
	var isPtr bool
	// 获取目标地址中的类型值
	destValue = reflect.Indirect(destValue)
	// 类型值对应的类型
	rowType = reflectx.Deref(destValue.Type())
	// 类型是否数组
	if rowType.Kind() == reflect.Slice {
		// 数组子类型
		rowType = rowType.Elem()
		isPtr = rowType.Kind() == reflect.Ptr
		// 子类型是否指针
		if isPtr {
			rowType = rowType.Elem()
		}
	}
	// rows
	var rows *sqlx.Rows
	rows, o.err = SqlxDB.NamedQuery(o.NamedSQL(), o.params)
	defer func() { _ = rows.Close() }()
	if o.err != nil {
		return o.err
	}
	var rowValue, rowValuePtr reflect.Value
	for rows.Next() {
		fieldMap := make(map[string]interface{})
		o.err = rows.MapScan(fieldMap)
		rowValuePtr = reflect.New(rowType)
		rowValue = reflect.Indirect(rowValuePtr)
		_ = o.bindRow(rowType, rowValue, fieldMap)
		if isPtr {
			destValue.Set(reflect.Append(destValue, rowValuePtr))
		} else {
			destValue.Set(reflect.Append(destValue, rowValue))
		}
	}
	return o.err
}

func (o *OrmModel) With(t string) *OrmModel {
	if o.method != methodSelect {
		o.err = errors.New(`o.method is not [methodSelect]`)
		return o
	}
	if len(t) > 0 {
		sql := o.NamedSQL()
		o.withTable = t
		o.withSQL = fmt.Sprintf(`With %s as (%s)`, t, sql)
	}
	return o
}

func (o *OrmModel) subSQL() string {
	var orderBy string
	if len(o.subOrderFields) > 0 {
		orderBy = fmt.Sprintf(`ORDER BY %s`, strings.Join(o.subOrderFields, ","))
	}
	subSql := fmt.Sprintf(`SELECT * %s %s %s`, `FROM`, o.withTable, orderBy)
	sql := fmt.Sprintf(`%s SELECT * FROM (%s) row`, o.withSQL, subSql)
	if o.log {
		log.Info().Str("sql", o.sql)
	}
	return sql
}

func (o *OrmModel) JsonbMapString(keys ...string) (string, error) {
	if len(keys) == 0 {
		return "", nil
	}
	var orderBy, sql string
	keysStr := strings.Join(keys, ",")
	if len(o.withSQL) > 0 {
		if len(o.subOrderFields) > 0 {
			orderBy = fmt.Sprintf(`ORDER BY %s`, strings.Join(o.subOrderFields, ","))
			subSql := fmt.Sprintf(`SELECT * %s %s %s`, `FROM`, o.withTable, orderBy)
			sql = fmt.Sprintf(`%s SELECT jsonb_object_agg(%s) FROM (%s) row`, o.withSQL, keysStr, subSql)
		} else {
			sql = fmt.Sprintf(`%s SELECT jsonb_object_agg(%s) FROM %s row`, o.withSQL, keysStr, o.withTable)
		}
	} else {
		subSql := o.NamedSQL()
		sql = fmt.Sprintf(`%s(%s) FROM (%s) row`, `SELECT jsonb_object_agg`, keysStr, subSql)
	}
	var result string
	if o.log {
		log.Info().Str("sql", o.sql)
	}
	var rows *sqlx.Rows
	rows, o.err = SqlxDB.NamedQuery(sql, o.params)
	defer func() { _ = rows.Close() }()
	if o.err != nil {
		return "", o.err
	}
	if rows.Next() {
		o.err = rows.Scan(&result)
	}
	return result, o.err
}

func (o *OrmModel) JsonbMap(dest interface{}, columns ...string) error {
	var jsonStr, err = o.JsonbMapString(columns...)
	if len(jsonStr) > 0 {
		return jsoniter.UnmarshalFromString(jsonStr, dest)
	}
	return err
}

func (o *OrmModel) JsonbListString() (string, error) {
	var orderBy, sql string
	if len(o.withSQL) > 0 {
		if len(o.subOrderFields) > 0 {
			orderBy = fmt.Sprintf(`ORDER BY %s`, strings.Join(o.subOrderFields, ","))
			subSql := fmt.Sprintf(`SELECT * %s %s %s`, `FROM`, o.withTable, orderBy)
			sql = fmt.Sprintf(`%s SELECT jsonb_agg(row) FROM (%s) row`, o.withSQL, subSql)
		} else {
			sql = fmt.Sprintf(`%s SELECT jsonb_agg(row) FROM %s row`, o.withSQL, o.withTable)
		}
	} else {
		subSql := o.NamedSQL()
		sql = fmt.Sprintf(`SELECT jsonb_agg(row) %s (%s) row`, `FROM`, subSql)
	}
	var result string
	if o.log {
		log.Info().Str("sql", o.sql)
	}
	if err := SqlxDB.Get(&result, sql); err != nil {
		o.err = err
		fmt.Println(time.Now().Format(time.DateTime+".0000"), "JsonbListString:", err, ", SQL:", sql)
	}
	return result, o.err
}

func (o *OrmModel) JsonbList(dest interface{}) error {
	var jsonStr, err = o.JsonbListString()
	if len(jsonStr) > 0 {
		return jsoniter.UnmarshalFromString(jsonStr, dest)
	}
	return err
}

func (o *OrmModel) bindRow(t reflect.Type, v reflect.Value, values map[string]interface{}) error {
	if t.Kind() != reflect.Struct {
		for _, i := range values {
			if o.err = setStructValue(v, i); o.err != nil {
				return o.err
			}
		}
		return nil
	}
	if len(o.tagIndexCache) == 0 {
		o.tagIndexCache = make(map[string]int)
		for i := 0; i < t.NumField(); i++ {
			dbTag := t.Field(i).Tag.Get(TagName)
			arr := strings.Split(dbTag, ",")
			dbTag = strings.TrimSpace(arr[0])
			o.tagIndexCache[dbTag] = i
		}
	}
	for tag, i := range o.tagIndexCache {
		destField := v.Field(i)
		if o.err = setStructValue(destField, values[tag]); o.err != nil {
			return o.err
		}
	}
	return nil
}

func valToString(v interface{}, format string) string {
	var typeValue string
	switch vv := v.(type) {
	case nil:
		return ""
	case string:
		typeValue = vv
	case *string:
		typeValue = *vv
	case int64:
		typeValue = strconv.FormatInt(vv, 10)
	case *int64:
		typeValue = strconv.FormatInt(*vv, 10)
	case uint64:
		typeValue = strconv.FormatUint(vv, 10)
	case float64:
		typeValue = strconv.FormatFloat(vv, 'f', -1, 64)
	case *float64:
		typeValue = strconv.FormatFloat(*vv, 'f', -1, 64)
	case bool:
		typeValue = strconv.FormatBool(vv)
	case *bool:
		typeValue = strconv.FormatBool(*vv)
	case []byte:
		if len(vv) > 0 {
			typeValue = fmt.Sprintf(`'%s'`, string(vv))
		}
	default:
		jsonStr, err := jsoniter.MarshalToString(vv)
		if err != nil {
			fmt.Println(fmt.Sprintf("error: valToString not processed, because value: %v", v))
			return ""
		}
		typeValue = fmt.Sprintf(`'%s'`, jsonStr)
	}
	if len(format) > 0 {
		return fmt.Sprintf(format, typeValue)
	}
	return typeValue
}

func (o *OrmModel) columnField(json string) string {
	if column, ok := o.dbFields[json]; ok {
		return column
	}
	return ""
}

func setStructValue(rv reflect.Value, val interface{}) error {
	if val == nil {
		return nil
	}
	kind := rv.Kind()
	fieldType := rv.Type().String()
	switch kind {
	case reflect.Ptr:
		if val == nil {
			break
		}
		switch fieldType {
		case "*string":
			a := val.(string)
			rv.Set(reflect.ValueOf(&a))
		default:
			switch val.(type) {
			case []byte:
				r := rv.Addr().Interface()
				if err := jsoniter.Unmarshal(val.([]byte), r); err != nil {
					return err
				}
			default:
				return fmt.Errorf("error: (%s) type not processed, because value: %v", fieldType, val)
			}
		}
	default:
		switch typeValue := val.(type) {
		case string:
			rv.SetString(typeValue)
		case int64:
			rv.SetInt(typeValue)
		case float64:
			rv.SetFloat(typeValue)
		case bool:
			rv.SetBool(typeValue)
		case []byte: // PQ Field: jsonb, numeric
			switch fieldType {
			case "float64":
				a := string(typeValue)
				rv.SetFloat(utilsgo.StringToFloat(a))
			case "string":
				a := string(typeValue)
				rv.SetString(a)
			case "int64":
				a := string(typeValue)
				rv.SetInt(utilsgo.FloatToInt(utilsgo.StringToFloat(a)))
			default:
				r := rv.Addr().Interface()
				if err := jsoniter.Unmarshal(typeValue, r); err != nil {
					return err
				}
			}
		case time.Time:
			t := typeValue.Format(utilsgo.YYYYMMDDHHMMSS)
			rv.SetString(t)
		default:
			return fmt.Errorf("error: (%s) type not processed, because value: %v", fieldType, val)
		}
	}
	return nil
}

func StructToMap(item interface{}) (map[string]interface{}, map[string]string) {
	jsonKeys := map[string]interface{}{}
	columnFields := map[string]string{}
	if item == nil {
		return jsonKeys, columnFields
	}
	t := reflect.TypeOf(item)
	reflectValue := reflect.ValueOf(item)
	reflectValue = reflect.Indirect(reflectValue)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	for i := 0; i < t.NumField(); i++ {
		jsonTag := t.Field(i).Tag.Get("json")
		jsonName := strings.TrimSpace(strings.Split(jsonTag, ",")[0])
		field := reflectValue.Field(i).Interface()
		if jsonTag != "" && jsonTag != "-" {
			if t.Field(i).Type.Kind() == reflect.Struct {
				jsonKeys[jsonName], _ = StructToMap(field)
			} else {
				jsonKeys[jsonName] = field
			}
		} else if t.Field(i).Type.Kind() == reflect.Struct {
			subMap, subDbMap := StructToMap(field)
			for kk, vv := range subMap {
				jsonKeys[kk] = vv
			}
			for kk, vv := range subDbMap {
				columnFields[kk] = vv
			}
		}
		// db Tag
		dbTag := t.Field(i).Tag.Get(TagName)
		if dbTag != "" && dbTag != "-" {
			arr := strings.SplitN(dbTag, ",", 2)
			if len(arr) == 1 {
				columnFields[jsonName] = strings.TrimSpace(arr[0])
			} else if strings.Contains(strings.TrimSpace(arr[1]), "pk") {
				columnFields[primaryKey] = jsonTag
				columnFields[jsonName] = strings.TrimSpace(arr[0])
			}
		}
	}
	return jsonKeys, columnFields
}

func (o *OrmModel) Error() error {
	if o == nil {
		return nil
	}
	return o.err
}

func UnmarshalGetPath(json []byte, val interface{}, path ...interface{}) error {
	//return UnmarshalStringGetPath(string(json), val, path)
	return jsoniter.UnmarshalFromString(jsoniter.Get(json, path...).ToString(), val)
}

/*func UnmarshalStringGetPath(jsonString string, val interface{}, path ...interface{}) error {
	node, err := sonic.GetFromString(jsonString, path...)
	if err != nil {
		return err
	}
	byteArr, err := node.MarshalJSON()
	if err != nil {
		return err
	}
	err = sonic.Unmarshal(byteArr, val)
	return err
}*/
