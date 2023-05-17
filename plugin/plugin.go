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

// MultiTenancy 多租户数据隔离插件
type MultiTenancy struct {
	tConn TenantDBConn
	*gorm.DB
	tenantTag string
	dbMap     map[string]*gorm.DB
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
