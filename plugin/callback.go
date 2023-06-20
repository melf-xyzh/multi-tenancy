/**
 * @Time    :2023/5/17 11:33
 * @Author  :Xiaoyu.Zhang
 */

package plugin

import (
	"errors"
	"fmt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/utils"
	"reflect"
	"strings"
)

type Model interface {
	TableName() string
	DataIsolation() bool
	AutoMigrate(db *gorm.DB, tableName string) error
}

func (mt *MultiTenancy) registerCallbacks(db *gorm.DB) {
	// 分库存储
	mt.Callback().Create().Before("*").Register("gorm:multi-tenancy", mt.createBeforeCallback)
	mt.Callback().Query().Before("*").Register("gorm:multi-tenancy", mt.queryBeforeCallback)
	mt.Callback().Update().Before("*").Register("gorm:multi-tenancy", mt.updateBeforeCallback)
	mt.Callback().Delete().Before("*").Register("gorm:multi-tenancy", mt.deleteBeforeCallback)
	mt.Callback().Row().Before("*").Register("gorm:multi-tenancy", mt.rowBeforeCallback)
	mt.Callback().Raw().Before("*").Register("gorm:multi-tenancy", mt.rawBeforeCallback)
	// 加密存储
	mt.Callback().Create().Before("*").Register("gorm:multi-tenancy-encrypt", mt.encryptCreateBeforeCallback)
	mt.Callback().Query().Before("*").Register("gorm:multi-tenancy-encrypt", mt.encryptQueryBeforeCallback)
	mt.Callback().Update().Before("*").Register("gorm:multi-tenancy-encrypt", mt.encryptUpdateBeforeCallback)
	mt.Callback().Query().After("*").Register("gorm:multi-tenancy-decrypt", mt.decryptQueryAfterCallback)
}

func (mt *MultiTenancy) createBeforeCallback(db *gorm.DB) {
	mt.commonCallback(db, mt.getTenantIdByModel)
}

func (mt *MultiTenancy) queryBeforeCallback(db *gorm.DB) {
	mt.commonCallback(db, mt.getTenantIdBySql)
}

func (mt *MultiTenancy) commonCallback(db *gorm.DB, getTenantId func(db *gorm.DB) (tenantId string)) {
	if db.Error != nil {
		return
	}
	model, ok := mt.DataIsolation(db)
	if !ok {
		return
	}
	// 获取租户ID
	tenantId := getTenantId(db)
	if db.Error != nil {
		return
	}
	// 根据租户ID切换数据库
	mt.getAndSwitchDBConnPool(db, tenantId)
	if db.Error != nil {
		return
	}
	mt.AutoMigrate(db, tenantId, model)

}

// AutoMigrate
/**
 *  @Description: 自动迁移
 *  @receiver mt
 *  @param db
 *  @param tenantId
 *  @param model
 */
func (mt *MultiTenancy) AutoMigrate(db *gorm.DB, tenantId string, model Model) {
	if mt.tableMap == nil {
		mt.tableMap = make(map[string]map[string]struct{})
	}
	_, ok := mt.tableMap[tenantId]
	if !ok {
		mt.tableMap[tenantId] = make(map[string]struct{})
	}
	table := db.Statement.Table
	_, ok = mt.tableMap[tenantId][table]
	if ok {
		// 不需要建表
		return
	}
	// 对数据库进行迁移
	createDB, _ := mt.GetDBByTenantId(tenantId)
	db.Error = model.AutoMigrate(createDB, db.Statement.Table)
}

func (mt *MultiTenancy) updateBeforeCallback(db *gorm.DB) {
	mt.commonCallback(db, mt.getTenantIdBySql)
}

func (mt *MultiTenancy) deleteBeforeCallback(db *gorm.DB) {
	mt.commonCallback(db, mt.getTenantIdBySql)
}

func (mt *MultiTenancy) rowBeforeCallback(db *gorm.DB) {}

func (mt *MultiTenancy) rawBeforeCallback(db *gorm.DB) {}

// SetDataIsolation
/**
 *  @Description: 获取数据隔离字段标识
 *  @receiver mt
 *  @return string
 */
func (mt *MultiTenancy) SetDataIsolation(model ...Model) (err error) {
	if mt.dataIsolation == nil {
		mt.dataIsolation = make(map[string]Model)
	}
	for _, m := range model {
		mt.dataIsolation[m.TableName()] = m
	}
	return
}

// DataIsolation
/**
 *  @Description: 是否进行数据隔离
 *  @receiver mt
 *  @param db
 *  @return dataIsolation
 */
func (mt *MultiTenancy) DataIsolation(db *gorm.DB) (model Model, dataIsolation bool) {
	var ok bool
	model, ok = mt.dataIsolation[db.Statement.Table]
	if ok {
		dataIsolation = model.DataIsolation()
		return
	}
	model, ok = mt.dataIsolation[db.Statement.Schema.Table]
	if ok {
		dataIsolation = model.DataIsolation()
		return
	}
	return
}

// newError
/**
 *  @Description: 封装Err
 *  @receiver mt
 *  @param errorStr
 *  @return err
 */
func (mt *MultiTenancy) newError(errorStr string) (err error) {
	return errors.New(fmt.Sprintf("【gorm:multi-tenancy】%s", errorStr))
}

// getDBConnPool
/**
 *  @Description: 获取连接池
 *  @receiver mt
 *  @param tenantId
 *  @return connPoll
 */
func (mt *MultiTenancy) getAndSwitchDBConnPool(db *gorm.DB, tenantId string) {
	// 根据租户ID切换数据库
	if tenantId == "" {
		db.Error = mt.newError("未检测到租户标识")
		return
	}
	// 获取数据库连接
	_, ok := mt.dbMap[tenantId]
	if !ok {
		// 如果该数据库没有连接，则创建数据库连接
		conn, errDB := mt.tConn.CreateDBConn(tenantId)
		if errDB != nil {
			db.Error = mt.newError(errDB.Error())
			return
		}
		mt.dbMap[tenantId] = conn
	}
	// 切换数据库连接池
	db.Statement.ConnPool = mt.dbMap[tenantId].ConnPool
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
		db.Error = mt.newError("未检测到 WHERE 子句")
		return
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
						db.Error = mt.newError("断言失败，仅支持string类型字段分库")
						return
					}
					if tenantId != "" && tenantIdi != tenantId {
						db.Error = mt.newError("不支持批量插入到不同的数据库")
						return
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
					db.Error = mt.newError("断言失败，仅支持string类型字段分库")
					return
				}
			}
		}
	}
	return
}
