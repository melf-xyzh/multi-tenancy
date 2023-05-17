/**
 * @Time    :2023/5/17 11:33
 * @Author  :Xiaoyu.Zhang
 */

package plugin

import (
	"fmt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/utils"
	"reflect"
	"strings"
)

func (mt *MultiTenancy) registerCallbacks(db *gorm.DB) {
	mt.Callback().Create().Before("*").Register("gorm:multi-tenancy", mt.createBeforeCallback)
	mt.Callback().Query().Before("*").Register("gorm:multi-tenancy", mt.queryBeforeCallback)
	mt.Callback().Update().Before("*").Register("gorm:multi-tenancy", mt.updateBeforeCallback)
	mt.Callback().Delete().Before("*").Register("gorm:multi-tenancy", mt.deleteBeforeCallback)
	mt.Callback().Row().Before("*").Register("gorm:multi-tenancy", mt.rowBeforeCallback)
	mt.Callback().Raw().Before("*").Register("gorm:multi-tenancy", mt.rawBeforeCallback)
}

// createBeforeCallback
/**
 *  @Description: 创建记录前的回调函数
 *  @receiver mt
 *  @param db
 */
func (mt *MultiTenancy) createBeforeCallback(db *gorm.DB) {
	if !mt.DataIsolation(db) {
		fmt.Println("不需要进行数据隔离")
		return
	}
	// 获取租户ID
	tenantId := mt.getTenantIdByModel(db)
	if tenantId != "" {
		// 切换数据库连接池
		db.Statement.ConnPool = mt.dbMap[tenantId].ConnPool
	}
	// 在数据库中建表
	err := db.AutoMigrate(db.Statement.Model)
	if err != nil {
		panic(err)
	}
}

func (mt *MultiTenancy) queryBeforeCallback(db *gorm.DB) {
	if !mt.DataIsolation(db) {
		fmt.Println("不需要进行数据隔离")
		return
	}
	// 获取租户ID
	tenantId := mt.getTenantIdBySql(db)
	if tenantId != "" {
		// 切换数据库连接池
		db.Statement.ConnPool = mt.dbMap[tenantId].ConnPool
	}
}

func (mt *MultiTenancy) updateBeforeCallback(db *gorm.DB) {
	if !mt.DataIsolation(db) {
		fmt.Println("不需要进行数据隔离")
		return
	}
	// 获取租户ID
	tenantId := mt.getTenantIdBySql(db)
	if tenantId != "" {
		// 切换数据库连接池
		db.Statement.ConnPool = mt.dbMap[tenantId].ConnPool
	}
}

func (mt *MultiTenancy) deleteBeforeCallback(db *gorm.DB) {
	if !mt.DataIsolation(db) {
		fmt.Println("不需要进行数据隔离")
		return
	}
	// 获取租户ID
	tenantId := mt.getTenantIdBySql(db)
	fmt.Println(tenantId)
	if tenantId != "" {
		// 切换数据库连接池
		db.Statement.ConnPool = mt.dbMap[tenantId].ConnPool
	}
	fmt.Println("数据库切换完成")
}

func (mt *MultiTenancy) rowBeforeCallback(db *gorm.DB) {

}

func (mt *MultiTenancy) rawBeforeCallback(db *gorm.DB) {

}

// DataIsolation
/**
 *  @Description: 是否进行数据隔离
 *  @receiver mt
 *  @param db
 *  @return bool
 */
func (mt *MultiTenancy) DataIsolation(db *gorm.DB) (dataIsolation bool) {
	model := db.Statement.Model
	v := reflect.ValueOf(model)
	// 获取方法
	if f1 := v.MethodByName("DataIsolation"); f1.IsValid() {
		answer := f1.Call([]reflect.Value{})
		if len(answer) == 0 {
			dataIsolation = false
		} else {
			// 将函数的结果传递该变量
			dataIsolation = answer[0].Bool()
		}
	} else {
		// 不存在此方法，认为该表不需要进行数据隔离
		dataIsolation = false
	}
	return
}

// getTenantIdBySql
/**
 *  @Description: 通过Sql获取写入的数据库
 *  @receiver mt
 *  @param db
 *  @return tenantId
 */
func (mt *MultiTenancy) getTenantIdBySql(db *gorm.DB) (tenantId string) {
	defer func() { tenantId = strings.TrimSpace(tenantId) }()
	whereClauses, ok := db.Statement.Clauses["WHERE"].Expression.(clause.Where)
	if !ok {
		panic("未检测到 WHERE 子句")
	}
	var build strings.Builder
	for i, expr := range whereClauses.Exprs {
		v := expr.(clause.Expr)
		sql := v.SQL
		for _, vi := range v.Vars {
			sql = strings.Replace(sql, "?", utils.ToString(vi), 1)
		}
		build.WriteString(sql)
		if i != len(whereClauses.Exprs)-1 {
			build.WriteString(" AND ")
		}
	}
	sql := build.String()
	strs := strings.Split(sql, "AND")
	for _, stri := range strs {
		sqlField := strings.Split(stri, "=")
		if strings.TrimSpace(sqlField[0]) == mt.getTenantTag() {
			tenantId = sqlField[1]
			return
		}
	}
	return
}

// getTenantIdByModel
/**
 *  @Description: 通过Model获取写入的数据库
 *  @receiver mt
 *  @param db
 *  @return tenantId
 */
func (mt *MultiTenancy) getTenantIdByModel(db *gorm.DB) (tenantId string) {
	defer func() { tenantId = strings.TrimSpace(tenantId) }()
	var ok bool
	if db.Statement.Schema != nil {
		for _, field := range db.Statement.Schema.Fields {
			// 判断是否分库字段
			if field.DBName != mt.getTenantTag() {
				continue
			}
			switch db.Statement.ReflectValue.Kind() {
			case reflect.Slice, reflect.Array:
				var tenantIdi string
				for i := 0; i < db.Statement.ReflectValue.Len(); i++ {
					// 获取值
					fieldValue, isZero := field.ValueOf(db.Statement.Context, db.Statement.ReflectValue.Index(i))
					if isZero {
						continue
					}
					tenantIdi, ok = fieldValue.(string)
					if !ok {
						panic("断言失败，仅支持string类型字段分库")
					}
					if tenantId != "" && tenantIdi != tenantId {
						panic("不支持批量插入到不同的数据库")
					}
					tenantId = tenantIdi
				}
				return
			case reflect.Struct:
				// 获取值
				fieldValue, isZero := field.ValueOf(db.Statement.Context, db.Statement.ReflectValue)
				if isZero {
					return
				}
				tenantId, ok = fieldValue.(string)
				if !ok {
					panic("断言失败，仅支持string类型字段分库")
				}
			}
		}
	}
	return
}
