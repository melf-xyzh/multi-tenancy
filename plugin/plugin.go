/**
 * @Time    :2023/5/16 10:11
 * @Author  :Xiaoyu.Zhang
 */

package plugin

import (
	"gorm.io/gorm"
)

const (
	// 默认数据隔离标识
	defaultTenantTag = "tenant_id"
)

// TenantDBConn 数据库连接接口
type TenantDBConn interface {
	CreateDBConn(tenant string) (db *gorm.DB, err error)
}

var MTPlugin *MultiTenancy

// MultiTenancy 多租户数据隔离插件
type MultiTenancy struct {
	tConn TenantDBConn
	*gorm.DB
	tenantTag     string
	dbMap         map[string]*gorm.DB
	tableMap      map[string]map[string]struct{}
	dataIsolation map[string]interface{}
}

func (mt *MultiTenancy) Name() string {
	return "gorm:multi-tenancy"
}

func (mt *MultiTenancy) Initialize(db *gorm.DB) error {
	mt.DB = db
	mt.registerCallbacks(db)
	return nil
}

// Register
/**
 *  @Description: 注册数据隔离插件
 *  @receiver mt
 *  @param dbMap 数据库Map
 *  @param tenantTag 数据隔离字段标识
 *  @return *MultiTenancy
 */
func (mt *MultiTenancy) Register(tenantTag string, conn TenantDBConn) *MultiTenancy {
	mt.dbMap = make(map[string]*gorm.DB)
	mt.tConn = conn
	mt.tenantTag = tenantTag
	MTPlugin = mt
	return mt
}

// AddDB
/**
 *  @Description: 注册数据库
 *  @receiver mt
 *  @param tenantId
 *  @param db
 */
func (mt *MultiTenancy) AddDB(tenantId string, db *gorm.DB) {
	if mt.dbMap == nil {
		mt.dbMap = make(map[string]*gorm.DB)
	}
	mt.dbMap[tenantId] = db
	return
}

// GetDBByTenantId
/**
 *  @Description: 利用租户标识获取数据库
 *  @receiver mt
 *  @param tenantId
 *  @return db
 *  @return err
 */
func (mt *MultiTenancy) GetDBByTenantId(tenantId string) (db *gorm.DB, err error) {
	if mt.dbMap == nil {
		mt.dbMap = make(map[string]*gorm.DB)
	}
	var ok bool
	// 获取数据库连接
	db, ok = mt.dbMap[tenantId]
	if !ok {
		// 如果该数据库没有连接，则创建数据库连接
		conn, errDB := mt.tConn.CreateDBConn(tenantId)
		if errDB != nil {
			db.Error = mt.newError(errDB.Error())
			return
		}
		mt.dbMap[tenantId] = conn
		db = conn
	}
	return
}

// getTenantTag
/**
 *  @Description: 获取数据隔离字段标识
 *  @receiver mt
 *  @return string
 */
func (mt *MultiTenancy) getTenantTag() string {
	if mt.tenantTag != "" {
		return mt.tenantTag
	}
	return defaultTenantTag
}
