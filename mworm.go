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

// ORMInterface 数据库表结构体接口，需实现 TableName 方法
// CRUDInterface 数据库操作接口，需实现 CRUDMode 方法
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
	primaryKey = "<pk>"
)

var (
	// SqlxDB 数据库连接对象
	SqlxDB *sqlx.DB
	// TagName 结构体 tag 名称
	TagName = "db"
)

type emptyKey = struct{}

type OrmModel struct {
	params          map[string]interface{}    // 结构体 Key Value
	dbFields        map[string]string         // 数据库字段
	tableName       string                    // 表名
	conditionFields map[string]emptyKey       // 条件字段
	orderFields     []string                  // 排序字段
	excludeFields   map[string]emptyKey       // 排除字段
	requiredFields  map[string]emptyKey       // 必选字段
	method          string                    // SQL 操作方式
	sql             string                    // SQL 语句
	emptyKeyExecute bool                      // 值为空时是否参与 Insert / Update 操作
	err             error                     // 错误提示
	tagIndexCache   map[string]int            // tag 索引缓存
	limit           int64                     // SQL LIMIT
	offset          int64                     // SQL OFFSET
	log             bool                      // true 时输出 log
	withTable       string                    // with 表名
	withSQL         string                    // with SQL
	withOrderFields []string                  // 子查询排序字段
	namedCGArr      map[string]ConditionGroup // Where 条件数组
	namedExec       bool                      // 是否使用了:name变量执行SQL
	returning       string                    // PQ:专用 RETURNING 语句
	pk              string                    //
	rawSQL          bool                      //
	updateFields    []string                  //
	//joinTables      []*JoinTable              // JOIN 表配置
}

// BindDB 绑定数据库
func BindDB(DB *sqlx.DB) error {
	SqlxDB = DB
	return SqlxDB.Ping()
}

// Table 指定表名
func Table(name string) *OrmModel {
	o := &OrmModel{}
	o.init()
	if SqlxDB.DriverName() == "postgres" {
		o.tableName = fmt.Sprintf(`"%s"`, name)
	} else {
		o.tableName = name
	}
	return o
}

// BatchArray 批量插入/更新
func BatchArray(ormArray []*OrmModel) error {
	tx := SqlxDB.MustBegin()
	defer func() { _ = tx.Rollback() }()
	for _, i := range ormArray {
		o := i
		if o == nil {
			continue
		}
		result, err := tx.NamedExec(o.FullSQL())
		if err != nil {
			return err
		}
		if _, err = result.RowsAffected(); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

// Batch 批量插入/更新
func Batch(ormArray ...*OrmModel) error {
	return BatchArray(ormArray)
}

// BatchFunc 批量操作
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

// SELECT 查询
func SELECT(i ORMInterface) *OrmModel {
	return Table(i.TableName()).setMethod(methodSelect, i)
}

// INSERT 插入
func INSERT(i ORMInterface) *OrmModel {
	return Table(i.TableName()).setMethod(methodInsert, i)
}

// UPDATE 更新
func UPDATE(i ORMInterface) *OrmModel {
	return Table(i.TableName()).setMethod(methodUpdate, i)
}

// DELETE 删除
func DELETE(i ORMInterface) *OrmModel {
	return Table(i.TableName()).setMethod(methodDelete, i)
}

// ExecRawSQL 执行原生 SQL
func ExecRawSQL(sql string, args ...any) error {
	_, err := SqlxDB.Exec(sql, args...)
	return err
}

// RawSQL 原生 SQL 查询
func RawSQL(sql string) *OrmModel {
	o := O()
	if len(sql) == 0 {
		o.err = errors.New("invalid sql")
	}
	o.rawSQL = true
	o.sql = sql
	return o
}

func (o *OrmModel) init() {
	o.requiredFields = make(map[string]emptyKey)
	o.excludeFields = make(map[string]emptyKey)
	o.conditionFields = make(map[string]emptyKey)
	o.namedCGArr = make(map[string]ConditionGroup)
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
		dbField := o.dbFields[f]
		if len(dbField) > 0 {
			o.orderFields = append(o.orderFields, dbField+` DESC`)
		}
	}
	return o
}

func (o *OrmModel) Asc(jsonTag ...string) *OrmModel {
	for _, f := range jsonTag {
		dbField := o.dbFields[f]
		if len(dbField) > 0 {
			o.orderFields = append(o.orderFields, dbField)
		}
	}
	return o
}

func (o *OrmModel) EmptyKey(f bool) *OrmModel {
	o.emptyKeyExecute = f
	return o
}

func (o *OrmModel) ExcludeFields(jsonTag ...string) *OrmModel {
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
	if row > 0 {
		o.limit = row
	}
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
			o.withOrderFields = append(o.withOrderFields, fmt.Sprintf(`%s.%s`, o.withTable, f))
		}
	}
	return o
}

func (o *OrmModel) WithDesc(fields ...string) *OrmModel {
	for _, f := range fields {
		if len(f) > 0 {
			o.withOrderFields = append(o.withOrderFields, fmt.Sprintf(`%s.%s DESC`, o.withTable, f))
		}
	}
	return o
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
	var count int64
	if o.rawSQL {
		if len(o.params) > 0 {
			o.err = Exec(o.sql)
		} else {
			count, o.err = SqlxDB.MustExec(o.sql).RowsAffected()
			if count == 0 && o.err == nil {
				o.err = errors.New(`影响行数为0`)
			}
		}
	} else {
		sql, _ := o.FullSQL()
		o.err = Exec(sql)
	}
	return o.err
}

// Count 统计数量
func (o *OrmModel) Count(column string) (int64, error) {
	var result int64
	sql := fmt.Sprintf(`SELECT count(%s) %s %s %s`, column, `FROM`, o.tableName, o.whereSQL())
	if o.log {
		log.Info().Str("sql", o.sql)
	}
	var rows *sqlx.Rows
	rows, o.err = SqlxDB.NamedQuery(sql, o.params)
	if o.err != nil {
		return 0, o.err
	}
	defer func() { _ = rows.Close() }()
	if rows.Next() {
		o.err = rows.Scan(&result)
	}
	return result, o.err
}

// One 查询单条记录
func (o *OrmModel) One(dest interface{}) error {
	if SqlxDB == nil {
		o.err = errors.New(`SqlxDB *sqlx.DB is nil`)
		return o.err
	}
	fieldMap := make(map[string]interface{})
	var rows *sqlx.Rows
	if o.rawSQL {
		if len(o.params) > 0 {
			rows, o.err = SqlxDB.NamedQuery(o.sql, o.params)
		} else {
			rows, o.err = SqlxDB.Queryx(o.sql)
		}
	} else {
		rows, o.err = SqlxDB.NamedQuery(o.Limit(1).FullSQL())
	}
	if o.err != nil {
		return o.err
	}
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
	_ = o.bindRow(t, v, fieldMap)
	return o.err
}

// Many 查询多条记录
func (o *OrmModel) Many(dest interface{}) error {
	if SqlxDB == nil {
		o.err = errors.New(`SqlxDB *sqlx.DB is nil`)
		return o.err
	}
	if (o.method != methodSelect && len(o.returning) == 0) && !o.rawSQL {
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
	if o.rawSQL {
		if len(o.params) > 0 {
			rows, o.err = SqlxDB.NamedQuery(o.sql, o.params)
		} else {
			rows, o.err = SqlxDB.Queryx(o.sql)
		}
	} else {
		rows, o.err = SqlxDB.NamedQuery(o.FullSQL())
	}
	if o.err != nil {
		return o.err
	}
	defer func() { _ = rows.Close() }()
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

// With 关联查询
func (o *OrmModel) With(t string) *OrmModel {
	if o.method != methodSelect {
		o.err = errors.New(`o.method is not [methodSelect]`)
		return o
	}
	if len(t) > 0 {
		o.withTable = t
	}
	return o
}

func (o *OrmModel) JsonbMapString(keys ...string) (string, error) {
	if len(keys) == 0 {
		return "", nil
	}
	var orderBy string
	keysStr := strings.Join(keys, ",")
	sql, _ := o.NamedSQL()
	if len(o.withSQL) > 0 {
		if len(o.withOrderFields) > 0 {
			orderBy = fmt.Sprintf(`ORDER BY %s`, strings.Join(o.withOrderFields, ","))
			subSql := fmt.Sprintf(`SELECT * %s %s %s`, `FROM`, o.withTable, orderBy)
			o.sql = fmt.Sprintf(`%s SELECT jsonb_object_agg(%s) FROM (%s) row`, o.withSQL, keysStr, subSql)
		} else {
			o.sql = fmt.Sprintf(`%s SELECT jsonb_object_agg(%s) FROM %s row`, o.withSQL, keysStr, o.withTable)
		}
	} else {
		o.sql = fmt.Sprintf(`%s(%s) FROM (%s) row`, `SELECT jsonb_object_agg`, keysStr, sql)
	}
	var result string
	if o.log {
		log.Info().Str("sql", o.sql)
	}
	var rows *sqlx.Rows
	rows, o.err = SqlxDB.NamedQuery(o.sql, o.params)
	if o.err != nil {
		return "", o.err
	}
	defer func() { _ = rows.Close() }()
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
	var orderBy string
	sql, _ := o.NamedSQL()
	if len(o.withSQL) > 0 {
		if len(o.withOrderFields) > 0 {
			orderBy = fmt.Sprintf(`ORDER BY %s`, strings.Join(o.withOrderFields, ","))
			subSql := fmt.Sprintf(`SELECT * %s %s %s`, `FROM`, o.withTable, orderBy)
			o.sql = fmt.Sprintf(`%s SELECT jsonb_agg(row) FROM (%s) row`, o.withSQL, subSql)
		} else {
			o.sql = fmt.Sprintf(`%s SELECT jsonb_agg(row) FROM %s row`, o.withSQL, o.withTable)
		}
	} else {
		o.sql = fmt.Sprintf(`SELECT jsonb_agg(row) %s (%s) row`, `FROM`, sql)
	}
	var result string
	if o.log {
		log.Info().Str("sql", o.sql)
	}
	if err := SqlxDB.Get(&result, o.sql); err != nil {
		o.err = err
		fmt.Println(time.Now().Format(time.DateTime+".0000"), "JsonbListString:", err, ", SQL:", o.sql)
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
			if len(dbTag) == 0 {
				continue
			}
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

// Exec 执行带命名参数的 SQL 语句
func Exec(sqlStr string) error {
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
		result, err = SqlxDB.Exec(sqlStr)
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
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		var s string
		switch v := val.(type) {
		case []byte:
			s = fmt.Sprintf(`%v`, string(v))
		default:
			s = fmt.Sprintf(`%v`, v)
		}
		a := utilsgo.StringToInt(s)
		rv.SetInt(a)
	case reflect.Float64, reflect.Float32:
		var s string
		switch v := val.(type) {
		case []byte:
			s = fmt.Sprintf(`%v`, string(v))
		default:
			s = fmt.Sprintf(`%v`, v)
		}
		a := utilsgo.StringToFloat(s)
		rv.SetFloat(a)
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
		case uint64:
			rv.SetUint(typeValue)
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

func StructToMap(item interface{}) (map[string]any, map[string]string) {
	jsonKeys := map[string]any{}
	columnFields := map[string]string{}
	if item == nil {
		return jsonKeys, columnFields
	}
	t := reflect.TypeOf(item)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		panic("item must be a struct")
	}
	reflectValue := reflect.ValueOf(item)
	reflectValue = reflect.Indirect(reflectValue)
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		jsonTag := field.Tag.Get("json")
		jsonName := strings.TrimSpace(strings.Split(jsonTag, ",")[0])
		if !reflectValue.Field(i).CanInterface() {
			continue
		}
		fieldValue := reflectValue.Field(i).Interface()
		if jsonTag != "" && jsonTag != "-" {
			if field.Type.Kind() == reflect.Struct {
				jsonKeys[jsonName], _ = StructToMap(fieldValue)
			} else {
				jsonKeys[jsonName] = fieldValue
			}
		} else if field.Type.Kind() == reflect.Struct {
			subMap, subDbMap := StructToMap(fieldValue)
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
			dbTagArr := strings.SplitN(dbTag, ",", 2)
			dbColumnName := strings.TrimSpace(dbTagArr[0])
			if len(dbTagArr) == 1 {
				columnFields[jsonName] = dbColumnName
			} else if strings.Contains(strings.TrimSpace(dbTagArr[1]), "pk") {
				columnFields[primaryKey] = strings.TrimSpace(jsonName)
				columnFields[jsonName] = dbColumnName
			}
			if jsonName != dbColumnName {
				jsonKeys[dbColumnName] = fieldValue
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

func JsonbBuildObjString(obj interface{}, prefix ...string) string {
	_, dbMap := StructToMap(obj)
	return dbMapBuildObjString(dbMap, prefix...)
}

func JsonTagToJsonbKeys(obj interface{}, prefix string, igTags ...string) string {
	_, dbMap := StructToMap(obj)
	for _, tag := range igTags {
		if dbMap[tag] != "" {
			delete(dbMap, tag)
		}
	}
	return dbMapBuildObjString(dbMap, prefix)
}

func dbMapBuildObjString(dbMap map[string]string, prefix ...string) string {
	var head string
	result := make([]string, 0)
	if len(prefix) > 0 && prefix[0] != "" {
		head = prefix[0] + "."
	}
	for json, column := range dbMap {
		if json == primaryKey {
			continue
		}
		s := fmt.Sprintf(`'%s',%s%s`, json, head, column)
		result = append(result, s)
	}
	return strings.Join(result, ",")
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
