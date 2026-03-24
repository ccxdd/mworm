package mworm

import (
	"testing"

	"github.com/jmoiron/sqlx"
)

// 初始化模拟 DB，用于基准测试
func init() {
	if SqlxDB == nil {
		SqlxDB = &sqlx.DB{}
	}
	DebugMode = false
}

// 测试结构体
type BenchmarkEntity struct {
	ID        int64   `json:"id" db:"id,pk"`
	Name      string  `json:"name" db:"name"`
	Email     string  `json:"email" db:"email"`
	Age       int     `json:"age" db:"age"`
	Score     float64 `json:"score" db:"score"`
	IsActive  bool    `json:"isActive" db:"is_active"`
	CreatedAt string  `json:"createdAt" db:"created_at"`
	UpdatedAt string  `json:"updatedAt" db:"updated_at"`
}

func (BenchmarkEntity) TableName() string {
	return "benchmark_table"
}

// BenchmarkStructToMap 测试 structToMap 性能
func BenchmarkStructToMap(b *testing.B) {
	entity := BenchmarkEntity{
		ID:        1,
		Name:      "test user",
		Email:     "test@example.com",
		Age:       25,
		Score:     95.5,
		IsActive:  true,
		CreatedAt: "2024-01-01 00:00:00",
		UpdatedAt: "2024-01-01 00:00:00",
	}

	b.ResetTimer()
	DebugMode = false
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		StructToMap(entity)
	}
}

// BenchmarkBuildSQL_SELECT 测试 SELECT SQL 构建性能
func BenchmarkBuildSQL_SELECT(b *testing.B) {
	entity := BenchmarkEntity{
		ID:       1,
		Name:     "test",
		IsActive: true,
	}

	b.ResetTimer()
	DebugMode = false
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		SELECT(entity).Where(And("id", "name"), Gt("age", 18)).BuildSQL()
	}
}

// BenchmarkBuildSQL_INSERT 测试 INSERT SQL 构建性能
func BenchmarkBuildSQL_INSERT(b *testing.B) {
	entity := BenchmarkEntity{
		ID:        1,
		Name:      "test user",
		Email:     "test@example.com",
		Age:       25,
		Score:     95.5,
		IsActive:  true,
		CreatedAt: "2024-01-01 00:00:00",
	}

	b.ResetTimer()
	DebugMode = false
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		INSERT(entity).BuildSQL()
	}
}

// BenchmarkBuildSQL_UPDATE 测试 UPDATE SQL 构建性能
func BenchmarkBuildSQL_UPDATE(b *testing.B) {
	entity := BenchmarkEntity{
		ID:       1,
		Name:     "updated name",
		IsActive: false,
	}

	b.ResetTimer()
	DebugMode = false
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		UPDATE(entity).WherePK().BuildSQL()
	}
}

// BenchmarkValueTypeToStr 测试类型转字符串性能
func BenchmarkValueTypeToStr_String(b *testing.B) {
	v := "test string value"
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		ValueTypeToStr(v)
	}
}

func BenchmarkValueTypeToStr_Int(b *testing.B) {
	v := 12345678
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		ValueTypeToStr(v)
	}
}

func BenchmarkValueTypeToStr_Int64(b *testing.B) {
	v := int64(1234567890123)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		ValueTypeToStr(v)
	}
}

func BenchmarkValueTypeToStr_Float64(b *testing.B) {
	v := 3.14159265358979
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		ValueTypeToStr(v)
	}
}

// BenchmarkTypeCache 测试类型缓存效果
func BenchmarkTypeCache(b *testing.B) {
	entity := BenchmarkEntity{}

	b.ResetTimer()
	DebugMode = false
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		SELECT(entity).Where(And("id")).BuildSQL()
	}
}

// BenchmarkConditionParsing 测试条件解析性能
func BenchmarkConditionParsing(b *testing.B) {
	entity := BenchmarkEntity{
		ID:       1,
		Name:     "test",
		Age:      25,
		IsActive: true,
	}

	b.ResetTimer()
	DebugMode = false
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		SELECT(entity).Where(
			And("id", "name"),
			Or("age", "isActive"),
			Gt("score", 80),
			Like("email"),
		).BuildSQL()
	}
}
