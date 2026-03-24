package mworm

import (
	dbsql "database/sql"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bytedance/sonic"
	utilsgo "github.com/ccxdd/utils-go"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jmoiron/sqlx"
	"github.com/jmoiron/sqlx/reflectx"
	"github.com/rs/zerolog/log"
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
	//
	emptyUpdateFlag = "eu"
	emptyInsertFlag = "ei"
	autoUpdateFlag  = "at"
	primaryKeyFlag  = "pk"
)

var (
	// SqlxDB 数据库连接对象
	SqlxDB *sqlx.DB
	// TagName 结构体 tag 名称
	TagName   = "db"
	DebugMode bool
)

// 结构体字段缓存信息
type structFieldInfo struct {
	jsonName   string   // JSON 标签名
	columnName string   // 数据库列名
	index      int      // 字段索引
	flags      []string // 额外标志 (pk, eu, ei, at)
}

// 结构体缓存
type structCache struct {
	fields    []structFieldInfo // 所有字段信息
	jsonMap   map[string]int    // json tag -> field index
	columnMap map[string]int    // column -> field index
	pkField   string            // 主键字段名
}

// 全局类型缓存
var typeCache sync.Map // map[reflect.Type]*structCache

type emptyKey = struct{}

type OrmModel struct {
	params            map[string]interface{} // 结构体 Key Value
	dbFields          map[string]string      // 数据库字段
	tableName         string                 // 表名
	conditionFields   map[string]emptyKey    // 条件字段
	orderFields       []string               // 排序字段 column
	excludeFields     map[string]emptyKey    // 排除字段 json
	requiredFields    map[string]emptyKey    // 必选字段 json
	emptyUpdateFields map[string]emptyKey    // 为空时也更新字段 column
	autoUpdateFields  map[string]emptyKey    // 自动更新字段 column
	method            string                 // SQL 操作方式
	sql               string                 // SQL 语句
	err               error                  // 错误提示
	tagIndexCache     map[string]int         // tag 索引缓存
	limit             int64                  // SQL LIMIT
	offset            int64                  // SQL OFFSET
	log               bool                   // true 时输出 log
	withTable         string                 // with 表名
	withSQL           string                 // with SQL
	withOrderFields   []string               // 子查询排序字段
	namedCGArr        []ConditionGroup       // Where 条件数组
	returning         string                 // PQ:专用 RETURNING 语句
	pk                string                 // primary key column
	rawSQL            bool                   //
	distinct          string                 //
	updateExpressions []string               // 更新字段 表达式
	groupBy           bool                   //
	groupByRaw        string                 //
	havingRaw         string                 //
	joinTables        []*JoinTable           // JOIN 表配置
	args              []any                  // Parameterized query args
	conflictColumns   []string               // ON CONFLICT 冲突列
	conflictUpdate    []string               // DO UPDATE 字段 (json tag)
	conflictDoNothing bool                   // DO NOTHING 标志
}

type SQLParams struct {
	Sql     string // 含占位符 ? 的 SQL（配合 Args 使用）
	WithSql string
	Params  map[string]interface{}
	Args    []any
	Err     error
}

func (sp SQLParams) ExeSql() string {
	return buildExeSql(sp.Sql, sp.Args)
}

func (sp SQLParams) TxExec(tx *sqlx.Tx) (dbsql.Result, error) {
	return tx.Exec(SqlxDB.Rebind(sp.Sql), sp.Args...)
}

func (sp SQLParams) TxMustExec(tx *sqlx.Tx) dbsql.Result {
	return tx.MustExec(SqlxDB.Rebind(sp.Sql), sp.Args...)
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
	if SqlxDB != nil && SqlxDB.DriverName() == "postgres" {
		o.tableName = fmt.Sprintf(`"%s"`, name)
	} else {
		o.tableName = name
	}
	return o
}

// BatchArray 批量插入/更新
func BatchArray(ormArray []*OrmModel) error {
	tx, err := SqlxDB.Beginx()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for _, o := range ormArray {
		if o == nil {
			continue
		}
		p := o.FullSQL()
		sqlStr := SqlxDB.Rebind(p.Sql)
		result, err := tx.Exec(sqlStr, p.Args...)
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
	tx, err := SqlxDB.Beginx()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	f(tx)
	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

// SELECT 查询
func SELECT(i ORMInterface, distinct ...bool) *OrmModel {
	return Table(i.TableName()).setMethod(methodSelect, i, distinct...)
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
	o.emptyUpdateFields = make(map[string]emptyKey)
	o.autoUpdateFields = make(map[string]emptyKey)
	o.namedCGArr = make([]ConditionGroup, 0)
}

// getOrCreateStructCache 获取或创建结构体缓存
func getOrCreateStructCache(t reflect.Type) *structCache {
	if cached, ok := typeCache.Load(t); ok {
		return cached.(*structCache)
	}

	// 创建新的缓存
	cache := &structCache{
		fields:    make([]structFieldInfo, 0, t.NumField()),
		jsonMap:   make(map[string]int, t.NumField()),
		columnMap: make(map[string]int, t.NumField()),
	}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		// 解析 json tag
		jsonTag := field.Tag.Get("json")
		jsonName := strings.TrimSpace(strings.Split(jsonTag, ",")[0])
		if jsonName == "" || jsonName == "-" {
			// 检查是否嵌套结构体
			if field.Type.Kind() == reflect.Struct && jsonTag == "" {
				continue // 嵌套结构体，跳过
			}
			continue
		}

		// 解析 db tag
		dbTag := field.Tag.Get(TagName)
		if dbTag == "" || dbTag == "-" {
			continue
		}

		var columnName string
		var flags []string
		dbTagArr := strings.Split(dbTag, ",")
		columnName = strings.TrimSpace(dbTagArr[0])
		if len(dbTagArr) > 1 {
			flags = dbTagArr[1:]
		}

		info := structFieldInfo{
			jsonName:   jsonName,
			columnName: columnName,
			index:      i,
			flags:      flags,
		}
		cache.fields = append(cache.fields, info)
		cache.jsonMap[jsonName] = len(cache.fields) - 1

		if columnName != "" {
			cache.columnMap[columnName] = len(cache.fields) - 1
		}

		// 检查主键标志
		for _, flag := range flags {
			if flag == primaryKeyFlag {
				cache.pkField = jsonName
				break
			}
		}
	}

	// 存储到缓存
	typeCache.Store(t, cache)
	return cache
}

func (o *OrmModel) Select(i interface{}, distinct ...bool) *OrmModel {
	return o.setMethod(methodSelect, i, distinct...)
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

func (o *OrmModel) setMethod(method string, i interface{}, distinct ...bool) *OrmModel {
	o.structToMap(i)
	o.method = method
	if o.method == methodSelect && len(distinct) > 0 && distinct[0] {
		o.distinct = "DISTINCT"
	}
	//if crud, b := i.(CRUDInterface); b {
	//	crud.CRUDMode(method)
	//}
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

func (o *OrmModel) AllowEmpty(jsonTag ...string) *OrmModel {
	for _, j := range jsonTag {
		dbField := o.dbFields[j]
		if len(dbField) > 0 {
			o.emptyUpdateFields[dbField] = emptyKey{}
		}
	}
	return o
}

func (o *OrmModel) ExcludeFields(jsonTag ...string) *OrmModel {
	for _, j := range jsonTag {
		o.excludeFields[j] = emptyKey{}
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
	for _, j := range jsonTag {
		o.requiredFields[j] = emptyKey{}
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
		column := o.dbFields[f]
		if len(column) > 0 {
			o.withOrderFields = append(o.withOrderFields, fmt.Sprintf(`%s.%s`, o.withTable, column))
		}
	}
	return o
}

func (o *OrmModel) WithDesc(fields ...string) *OrmModel {
	for _, f := range fields {
		column := o.dbFields[f]
		if len(column) > 0 {
			o.withOrderFields = append(o.withOrderFields, fmt.Sprintf(`%s.%s DESC`, o.withTable, column))
		}
	}
	return o
}

func (o *OrmModel) whereSQL() string {
	where, args := o.parseConditionNamed()
	o.args = append(o.args, args...)
	return where
}

// Exec 执行由 OrmModel 生成的 SQL 查询，并在出现错误时返回错误。
//
// 该函数不接受任何参数。
// 它返回一个错误。
func (o *OrmModel) Exec() error {
	var count int64
	if o.rawSQL {
		count, o.err = SqlxDB.MustExec(o.sql).RowsAffected()
		if count == 0 && o.err == nil {
			o.err = errors.New(`影响行数为0`)
		}
	} else {
		fullParams := o.FullSQL()
		sqlStr := SqlxDB.Rebind(fullParams.Sql)
		var result dbsql.Result
		result, o.err = SqlxDB.Exec(sqlStr, o.args...)
		if o.err == nil {
			count, o.err = result.RowsAffected()
			if count == 0 {
				o.err = errors.New(`影响行数为0`)
			}
		}
	}
	return o.err
}

// Count 统计数量
func (o *OrmModel) Count(column string) (int64, error) {
	var result int64
	o.sql = fmt.Sprintf(`SELECT count(%s) %s %s %s`, column, `FROM`, o.tableName, o.whereSQL())
	if o.log || DebugMode {
		log.Debug().Str("sql", o.FullSQL().ExeSql()).Msg("Count")
	}
	var rows *sqlx.Rows
	sqlStr := SqlxDB.Rebind(o.sql)
	rows, o.err = SqlxDB.Queryx(sqlStr, o.args...)
	if o.err != nil {
		return 0, o.err
	}
	defer func() { _ = rows.Close() }()
	if rows.Next() {
		o.err = rows.Scan(&result)
	}
	return result, o.err
}

// aggregate 通用聚合查询（SUM/AVG/MIN/MAX）
func (o *OrmModel) aggregate(fn, column string) (float64, error) {
	var result float64
	o.sql = fmt.Sprintf(`SELECT %s(%s) %s %s %s`, fn, column, `FROM`, o.tableName, o.whereSQL())
	if o.log || DebugMode {
		log.Debug().Str("sql", o.FullSQL().ExeSql()).Msg(fn)
	}
	var rows *sqlx.Rows
	sqlStr := SqlxDB.Rebind(o.sql)
	rows, o.err = SqlxDB.Queryx(sqlStr, o.args...)
	if o.err != nil {
		return 0, o.err
	}
	defer func() { _ = rows.Close() }()
	if rows.Next() {
		var nullVal *float64
		o.err = rows.Scan(&nullVal)
		if nullVal != nil {
			result = *nullVal
		}
	}
	return result, o.err
}

// Sum 求和
func (o *OrmModel) Sum(column string) (float64, error) {
	return o.aggregate("SUM", column)
}

// Avg 平均值
func (o *OrmModel) Avg(column string) (float64, error) {
	return o.aggregate("AVG", column)
}

// Min 最小值
func (o *OrmModel) Min(column string) (float64, error) {
	return o.aggregate("MIN", column)
}

// Max 最大值
func (o *OrmModel) Max(column string) (float64, error) {
	return o.aggregate("MAX", column)
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
		rows, o.err = SqlxDB.Queryx(o.sql)
	} else {
		fullParams := o.Limit(1).FullSQL()
		sqlStr := SqlxDB.Rebind(fullParams.Sql)
		rows, o.err = SqlxDB.Queryx(sqlStr, o.args...)
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
		rows, o.err = SqlxDB.Queryx(o.sql)
	} else {
		fullParams := o.FullSQL()
		sqlStr := SqlxDB.Rebind(fullParams.Sql)
		rows, o.err = SqlxDB.Queryx(sqlStr, o.args...)
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

func (o *OrmModel) GroupBy(cgs ...ConditionGroup) *OrmModel {
	o.requiredFields = make(map[string]emptyKey)
	o.groupBy = true
	for _, c := range cgs {
		switch c.cType {
		case cgTypeGroupFields:
			for _, f := range c.JsonTags {
				o.requiredFields[f] = emptyKey{}
			}
		case cgTypeRaw:
			if len(c.Args) == 0 {
				o.groupByRaw = c.Express
			} else {
				tmp := c.Express
				for i, arg := range c.Args {
					vStr := ValueTypeToStr(arg)
					tmp = strings.Replace(tmp, "$"+strconv.Itoa(i+1), vStr, 1)
				}
				o.groupByRaw = tmp
			}
		default:
			panic("unhandled default case")
		}
	}
	return o
}

func (o *OrmModel) Having(exp string, args ...any) *OrmModel {
	if exp == "" {
		return o
	}
	if len(args) == 0 {
		o.groupByRaw = exp
	} else {
		tmp := exp
		for i, arg := range args {
			vStr := ValueTypeToStr(arg)
			tmp = strings.Replace(tmp, "$"+strconv.Itoa(i+1), vStr, 1)
		}
		o.havingRaw = tmp
	}
	return o
}

func (o *OrmModel) JsonbMapString(keys ...string) (string, error) {
	if len(keys) == 0 {
		return "", nil
	}
	var orderBy string
	var columns = make([]string, len(keys))
	for i, key := range keys {
		column := o.columnField(key)
		if len(column) > 0 {
			columns[i] = column
		} else {
			columns[i] = key
		}
		if key == "row" {
			columns[i] = fmt.Sprintf(`jsonb_build_object(%s)`, dbMapBuildObjString(o.dbFields))
		}
	}
	keysStr := strings.Join(columns, ",")
	sqlParams := o.BuildSQL()
	if len(o.withSQL) > 0 {
		if len(o.withOrderFields) > 0 {
			orderBy = fmt.Sprintf(`ORDER BY %s`, strings.Join(o.withOrderFields, ","))
			subSql := fmt.Sprintf(`SELECT * %s %s %s`, `FROM`, o.withTable, orderBy)
			o.sql = fmt.Sprintf(`%s SELECT jsonb_object_agg(%s) FROM (%s) row`, o.withSQL, keysStr, subSql)
		} else {
			o.sql = fmt.Sprintf(`%s SELECT jsonb_object_agg(%s) FROM %s row`, o.withSQL, keysStr, o.withTable)
		}
		o.args = sqlParams.Args
	} else {
		o.sql = fmt.Sprintf(`%s(%s) FROM (%s) row`, `SELECT jsonb_object_agg`, keysStr, sqlParams.Sql)
		o.args = sqlParams.Args
	}
	var result string
	if o.log || DebugMode {
		log.Debug().Str("sql", o.sql).Msg("JsonbMapString")
	}
	var rows *sqlx.Rows
	sqlStr := SqlxDB.Rebind(o.sql)
	rows, o.err = SqlxDB.Queryx(sqlStr, o.args...)
	if o.err != nil {
		return "", o.err
	}
	defer func() { _ = rows.Close() }()
	if rows.Next() {
		m := map[string]interface{}{}
		o.err = rows.MapScan(m)
		if m["jsonb_object_agg"] != nil {
			result = string(m["jsonb_object_agg"].([]uint8))
		}
	}
	return result, o.err
}

func (o *OrmModel) JsonbMap(dest interface{}, columns ...string) error {
	var jsonStr, err = o.JsonbMapString(columns...)
	if len(jsonStr) > 0 {
		return sonic.UnmarshalString(jsonStr, dest)
	}
	return err
}
func (o *OrmModel) JsonbListString() (string, error) {
	var orderBy string
	sqlParams := o.BuildSQL()
	rowKeys := fmt.Sprintf(`jsonb_build_object(%s)`, dbMapBuildObjString(o.dbFields))
	if len(o.withSQL) > 0 {
		if len(o.withOrderFields) > 0 {
			orderBy = fmt.Sprintf(`ORDER BY %s`, strings.Join(o.withOrderFields, ","))
			subSql := fmt.Sprintf(`SELECT * %s %s %s`, `FROM`, o.withTable, orderBy)
			o.sql = fmt.Sprintf(`%s SELECT jsonb_agg(%s) FROM (%s) row`, o.withSQL, rowKeys, subSql)
		} else {
			o.sql = fmt.Sprintf(`%s SELECT jsonb_agg(%s) FROM %s row`, o.withSQL, rowKeys, o.withTable)
		}
		o.args = sqlParams.Args
	} else {
		o.sql = fmt.Sprintf(`SELECT jsonb_agg(%s) %s (%s) row`, rowKeys, `FROM`, sqlParams.Sql)
		o.args = sqlParams.Args
	}
	var result string
	if o.log || DebugMode {
		log.Debug().Str("sql", o.FullSQL().ExeSql()).Msg("JsonbListString")
	}
	var rows *sqlx.Rows
	sqlStr := SqlxDB.Rebind(o.sql)
	rows, o.err = SqlxDB.Queryx(sqlStr, o.args...)
	if o.err != nil {
		return "", o.err
	}
	defer func() { _ = rows.Close() }()
	if rows.Next() {
		m := map[string]interface{}{}
		o.err = rows.MapScan(m)
		if m["jsonb_agg"] != nil {
			result = string(m["jsonb_agg"].([]uint8))
		}
	}
	return result, o.err
}

func (o *OrmModel) JsonbList(dest interface{}) error {
	// 直接走标准的 Many，完全避免强迫 DB 去执行 jsonb_agg 等计算，减轻数据库 CPU 压力，并确保了多库兼容
	return o.Many(dest)
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

	cache := getOrCreateStructCache(t)
	for _, fieldInfo := range cache.fields {
		if fieldInfo.columnName == "" {
			continue
		}
		if val, ok := values[fieldInfo.columnName]; ok {
			destField := v.Field(fieldInfo.index)
			if o.err = setStructValue(destField, val); o.err != nil {
				return o.err
			}
		}
	}
	return nil
}

// Exec 执行带命名参数的 SQL 语句
func Exec(sqlStr string) error {
	if SqlxDB == nil {
		return errors.New(`SqlxDB *sqlx.DB is nil`)
	}
	result, err := SqlxDB.Exec(sqlStr)
	if err != nil {
		if pgErr, ok := err.(*pgconn.PgError); ok {
			log.Error().Msg(pgErr.Message)
		} else {
			log.Error().Msgf("%v", err)
		}
		return err
	}
	count, err := result.RowsAffected()
	if count == 0 && err == nil {
		return errors.New(`影响行数为0`)
	}
	return err
}

func (o *OrmModel) columnField(json string) string {
	if column, ok := o.dbFields[json]; ok {
		return column
	}
	return json
}

func setStructValue(rv reflect.Value, val interface{}) error {
	if val == nil {
		return nil
	}
	kind := rv.Kind()
	switch kind {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		switch v := val.(type) {
		case int64:
			rv.SetInt(v)
		case int:
			rv.SetInt(int64(v))
		case []byte:
			rv.SetInt(utilsgo.StringToInt(string(v)))
		case string:
			rv.SetInt(utilsgo.StringToInt(v))
		default:
			rv.SetInt(utilsgo.StringToInt(fmt.Sprint(v)))
		}
	case reflect.Float64, reflect.Float32:
		switch v := val.(type) {
		case float64:
			rv.SetFloat(v)
		case float32:
			rv.SetFloat(float64(v))
		case []byte:
			rv.SetFloat(utilsgo.StringToFloat(string(v)))
		case string:
			rv.SetFloat(utilsgo.StringToFloat(v))
		default:
			rv.SetFloat(utilsgo.StringToFloat(fmt.Sprint(v)))
		}
	case reflect.String:
		switch v := val.(type) {
		case string:
			rv.SetString(v)
		case []byte:
			rv.SetString(string(v))
		case time.Time:
			if v.Hour() > 0 && v.Minute() > 0 {
				rv.SetString(v.Format("2006-01-02 15:04:05"))
			} else {
				rv.SetString(v.Format("2006-01-02"))
			}
		default:
			rv.SetString(fmt.Sprint(v))
		}
	case reflect.Bool:
		switch v := val.(type) {
		case bool:
			rv.SetBool(v)
		case int64:
			rv.SetBool(v != 0)
		case []byte:
			rv.SetBool(string(v) == "true" || string(v) == "1")
		default:
			rv.SetBool(fmt.Sprint(v) == "true")
		}
	case reflect.Ptr:
		if rv.Type().String() == "*string" {
			if s, ok := val.(string); ok {
				rv.Set(reflect.ValueOf(&s))
			} else if b, ok := val.([]byte); ok {
				s := string(b)
				rv.Set(reflect.ValueOf(&s))
			}
		} else {
			if b, ok := val.([]byte); ok {
				r := rv.Addr().Interface()
				if err := sonic.Unmarshal(b, r); err != nil {
					return err
				}
			}
		}
	default:
		switch v := val.(type) {
		case time.Time:
			if rv.Type().String() == "string" {
				if v.Hour() > 0 && v.Minute() > 0 {
					rv.SetString(v.Format("2006-01-02 15:04:05"))
				} else {
					rv.SetString(v.Format("2006-01-02"))
				}
			} else {
				rv.Set(reflect.ValueOf(v))
			}
		case []byte:
			r := rv.Addr().Interface()
			if err := sonic.Unmarshal(v, r); err != nil {
				return err
			}
		default:
			tryValue := reflect.ValueOf(val)
			if tryValue.Type().AssignableTo(rv.Type()) {
				rv.Set(tryValue)
			} else {
				return fmt.Errorf("error: (%s) type not processed, because value: %v", rv.Type().String(), val)
			}
		}
	}
	return nil
}

func (o *OrmModel) structToMap(item any) (map[string]any, map[string]string) {
	if item == nil {
		return map[string]any{}, map[string]string{}
	}
	t := reflect.TypeOf(item)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		panic("item must be a struct")
	}

	// 使用缓存
	cache := getOrCreateStructCache(t)

	// 预分配容量
	jsonKeys := make(map[string]any, len(cache.fields))
	columnFields := make(map[string]string, len(cache.fields))

	reflectValue := reflect.ValueOf(item)
	reflectValue = reflect.Indirect(reflectValue)

	// 使用缓存的字段信息
	for _, fieldInfo := range cache.fields {
		if !reflectValue.Field(fieldInfo.index).CanInterface() {
			continue
		}
		fieldValue := reflectValue.Field(fieldInfo.index).Interface()

		// 设置 json 键值
		jsonKeys[fieldInfo.jsonName] = fieldValue

		// 设置 column 映射
		if fieldInfo.columnName != "" {
			columnFields[fieldInfo.jsonName] = fieldInfo.columnName

			// 处理 db 标志
			for _, flag := range fieldInfo.flags {
				switch flag {
				case primaryKeyFlag:
					o.pk = fieldInfo.jsonName
				case emptyUpdateFlag:
					o.emptyUpdateFields[fieldInfo.columnName] = emptyKey{}
				case autoUpdateFlag:
					o.autoUpdateFields[fieldInfo.columnName] = emptyKey{}
				}
			}

			// 如果 jsonName != columnName，添加额外映射
			if fieldInfo.jsonName != fieldInfo.columnName {
				jsonKeys[fieldInfo.columnName] = fieldValue
			}
		}
	}

	// 设置主键（如果有缓存）
	if cache.pkField != "" && o.pk == "" {
		o.pk = cache.pkField
	}

	o.params, o.dbFields = jsonKeys, columnFields
	return jsonKeys, columnFields
}

func StructToMap(item any) (map[string]any, map[string]string) {
	orm := new(OrmModel)
	return orm.structToMap(item)
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
	if len(dbMap) == 0 {
		return ""
	}
	var head string
	if len(prefix) > 0 && prefix[0] != "" {
		head = prefix[0] + "."
	}

	var builder strings.Builder
	first := true
	for json, column := range dbMap {
		if !first {
			builder.WriteString(",")
		}
		builder.WriteString("'")
		builder.WriteString(json)
		builder.WriteString("',")
		builder.WriteString(head)
		builder.WriteString(column)
		first = false
	}
	return builder.String()
}
