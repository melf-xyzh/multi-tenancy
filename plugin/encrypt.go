/**
 * @Time    :2023/6/16 10:10
 * @Author  :Xiaoyu.Zhang
 */

package plugin

import (
	"context"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/schema"
	"gorm.io/gorm/utils"
	"reflect"
	"strings"
)

const DefaultTagName = "mt"

type MultiTenancyTag struct {
	DBName    string
	FieldName string
	FieldType reflect.Type
	tag       string
	Encrypt   bool
}

const (
	encrypt = 1
	decrypt = 2
)

// analyzeMTTag
/**
 *  @Description: 解析Tag
 *  @receiver mt
 *  @param tag
 *  @param mtTag
 */
func (mt *MultiTenancy) analyzeMTTag(tag string, mtTag *MultiTenancyTag) {
	tags := strings.Split(tag, ";")
	for _, ti := range tags {
		if strings.Contains(ti, "encrypt") {
			mtTag.Encrypt = true
		}
	}
}

// analyzeDBModel
/**
 *  @Description: 解析结构体数据
 *  @receiver mt
 *  @param db
 */
func (mt *MultiTenancy) analyzeDBModel(db *gorm.DB) {
	if db.Error != nil {
		return
	}

	// 获取结构体 reflect.Type
	//typeOf := db.Statement.Schema.ModelType
	if db.Statement.Schema != nil {
		for _, field := range db.Statement.Schema.Fields {
			mtTag := MultiTenancyTag{
				DBName:    field.DBName,
				FieldName: field.Name,
				FieldType: field.FieldType,
			}
			value, ok := field.Tag.Lookup(DefaultTagName)
			if ok {
				mtTag.tag = value
			}
			// 解析MTTag
			mt.analyzeMTTag(mtTag.tag, &mtTag)
			mt.tagMap[mtTag.DBName] = mtTag
			if mt.tagMap[mtTag.DBName].Encrypt {
				mt.needEncryptDBFields[mtTag.DBName] = struct{}{}
				mt.needEncryptFields[field.Name] = struct{}{}
			}
		}
	}
	return
}

func (mt *MultiTenancy) getReflectElem(i interface{}) (reflect.Type, reflect.Value) {
	destType := reflect.TypeOf(i)
	destValue := reflect.ValueOf(i)
	for {
		if destType.Elem().Kind() == reflect.Ptr {
			destType = destType.Elem()
			destValue = destValue.Elem()
		} else if destType.Elem().Kind() == reflect.Struct {
			break
		} else {
			return nil, reflect.ValueOf(nil)
		}
	}
	return destType.Elem(), destValue.Elem()
}

func (mt *MultiTenancy) setEncryptData(field *schema.Field, ctx context.Context, valueOf reflect.Value, flag int) (err error) {
	// 获取值
	fieldValue, isZero := field.ValueOf(ctx, valueOf)
	if isZero {
		return
	}
	value, ok := fieldValue.(string)
	if !ok {
		err = mt.newError("断言失败，仅支持string类型字段分库")
		return
	}
	cipherTxt := value
	switch flag {
	case encrypt:
		if mt.encrypt == nil {
			err = mt.newError("未设置加密方法")
			return
		}
		cipherTxt, err = mt.encrypt(value)
		if err != nil {
			err = mt.newError("加密异常：" + err.Error())
			return
		}
	case decrypt:
		if mt.decrypt == nil {
			err = mt.newError("未设置解密方法")
			return
		}
		cipherTxt, err = mt.decrypt(value)
		if err != nil {
			err = mt.newError("解密异常：" + err.Error())
			return
		}
	}
	// Set value to field
	err = field.Set(ctx, valueOf, cipherTxt)
	if err != nil {
		err = mt.newError("对结构体赋值异常：" + err.Error())
		return
	}
	return
}

// encryptCommonCallback
/**
 *  @Description: 结构体加密公共方法
 *  @receiver mt
 *  @param db
 */
func (mt *MultiTenancy) encryptCommonCallback(db *gorm.DB) {
	if db.Error != nil {
		return
	}
	if db.Statement.Schema != nil {
		for _, field := range db.Statement.Schema.Fields {
			// 判断是否需要加密
			mtTag, ok := mt.tagMap[field.DBName]
			if !ok || !mtTag.Encrypt {
				// 未查询到该字段 或 不需要加密
				continue
			}
			switch db.Statement.ReflectValue.Kind() {
			case reflect.Slice, reflect.Array:
				for i := 0; i < db.Statement.ReflectValue.Len(); i++ {
					db.Error = mt.setEncryptData(field, db.Statement.Context, db.Statement.ReflectValue.Index(i), encrypt)
					if db.Error != nil {
						return
					}
				}
				return
			case reflect.Struct:
				db.Error = mt.setEncryptData(field, db.Statement.Context, db.Statement.ReflectValue, encrypt)
				if db.Error != nil {
					return
				}
			}
		}
	}
}

// encryptBySql
/**
 *  @Description: 加密Sql
 *  @receiver mt
 *  @param db
 */
func (mt *MultiTenancy) encryptBySql(db *gorm.DB) {
	if db.Error != nil {
		return
	}
	// 若无需要加密的字段，则不需要解析SQL
	if mt.needEncryptDBFields == nil || len(mt.needEncryptDBFields) == 0 {
		return
	}
	whereClauses, ok := db.Statement.Clauses["WHERE"].Expression.(clause.Where)
	if !ok {
		db.Error = mt.newError("未检测到 WHERE 子句")
		return
	}
	exprs := whereClauses.Exprs
	for i, expr := range exprs {
		switch exprType := expr.(type) {
		case clause.Eq:
			_, ok = mt.needEncryptDBFields[exprType.Column.(string)]
			if !ok {
				_, ok = mt.needEncryptDBFields[exprType.Column.(clause.Column).Name]
			}
			if ok {
				expri := clause.Eq{
					Column: exprType.Column,
				}
				expri.Value, db.Error = mt.encrypt(utils.ToString(exprType.Value))
				if db.Error != nil {
					return
				}
				exprs[i] = expri
			}
		case clause.IN:
		case clause.Expr:
			if exprType.Vars == nil {
				continue
			}
			sql := exprType.SQL
			for fields, _ := range mt.needEncryptDBFields {
				if strings.Contains(sql, fields+" ") {
					switch {
					case strings.Contains(sql, fields+" = ?"):
						// 获取sql片段
						subSql := fields + " = ?"
						// 获取该片段出现的位置
						index := strings.Index(sql, subSql)
						// 或者该字段对应的?的索引
						count := strings.Count(sql[:index+len(subSql)], "?")
						value := utils.ToString(exprType.Vars[count-1])
						exprType.Vars[count-1], db.Error = mt.encrypt(value)
					case strings.Contains(sql, fields+" != ?"):
						// 获取sql片段
						subSql := fields + " != ?"
						// 获取该片段出现的位置
						index := strings.Index(sql, subSql)
						// 或者该字段对应的?的索引
						count := strings.Count(sql[:index+len(subSql)], "?")
						value := utils.ToString(exprType.Vars[count-1])
						exprType.Vars[count-1], db.Error = mt.encrypt(value)
					case strings.Contains(sql, fields+" in ?"):
						// 获取sql片段
						subSql := fields + " in ?"
						// 获取该片段出现的位置
						index := strings.Index(sql, subSql)
						// 或者该字段对应的?的索引
						count := strings.Count(sql[:index+len(subSql)], "?")
						values, ok := exprType.Vars[count-1].([]string)
						if ok {
							for j, value := range values {
								values[j], db.Error = mt.encrypt(value)
								if db.Error != nil {
									return
								}
							}
						}
						exprType.Vars[count-1] = values
					case strings.Contains(sql, fields+" not in ?"):
						// 获取sql片段
						subSql := fields + " not in ?"
						// 获取该片段出现的位置
						index := strings.Index(sql, subSql)
						// 或者该字段对应的?的索引
						count := strings.Count(sql[:index+len(subSql)], "?")
						values, ok := exprType.Vars[count-1].([]string)
						if ok {
							for j, value := range values {
								values[j], db.Error = mt.encrypt(value)
								if db.Error != nil {
									return
								}
							}
						}
						exprType.Vars[count-1] = values
					default:
						db.Error = mt.newError(fields + "字段已经开启加密，仅支持精确匹配查询")
					}
				}
			}
		}
	}
	return
}

func (mt *MultiTenancy) encryptCreateBeforeCallback(db *gorm.DB) {
	if db.Error != nil {
		return
	}
	// 若未开启加密保存，则不执行后续操作
	if !mt.encryptedSave {
		return
	}
	// 对Tag进行解析
	mt.analyzeDBModel(db)
	// 加密结构体数据
	mt.encryptCommonCallback(db)
}

func (mt *MultiTenancy) decryptQueryAfterCallback(db *gorm.DB) {
	if db.Error != nil {
		return
	}
	// 若未开启加密保存，则不执行后续操作
	if !mt.encryptedSave {
		return
	}
	// 对Tag进行解析
	mt.analyzeDBModel(db)
	if db.Error != nil {
		return
	}
	if db.Statement.Schema != nil {
		for _, field := range db.Statement.Schema.Fields {
			// 判断是否需要解密
			mtTag, ok := mt.tagMap[field.DBName]
			if !ok || !mtTag.Encrypt {
				// 未查询到该字段 或 不需要加密
				continue
			}
			switch db.Statement.ReflectValue.Kind() {
			case reflect.Slice, reflect.Array:
				for i := 0; i < db.Statement.ReflectValue.Len(); i++ {
					db.Error = mt.setEncryptData(field, db.Statement.Context, db.Statement.ReflectValue.Index(i), decrypt)
					if db.Error != nil {
						return
					}
				}
			case reflect.Struct:
				// 获取值
				db.Error = mt.setEncryptData(field, db.Statement.Context, db.Statement.ReflectValue, decrypt)
				if db.Error != nil {
					return
				}
			default:
			}
		}
	}
}

func (mt *MultiTenancy) encryptQueryBeforeCallback(db *gorm.DB) {
	if db.Error != nil {
		return
	}
	// 若未开启加密保存，则不执行后续操作
	if !mt.encryptedSave {
		return
	}
	// 对Tag进行解析
	mt.analyzeDBModel(db)
	// 加密sql
	mt.encryptBySql(db)
}

func (mt *MultiTenancy) encryptUpdateBeforeCallback(db *gorm.DB) {
	if db.Error != nil {
		return
	}
	// 若未开启加密保存，则不执行后续操作
	if !mt.encryptedSave {
		return
	}
	// 对Tag进行解析
	mt.analyzeDBModel(db)

	if db.Statement.Schema == nil {
		return
	}
	if updateInfo, ok := db.Statement.Dest.(map[string]interface{}); ok {
		for updateColumn := range updateInfo {
			_, ok = mt.needEncryptDBFields[updateColumn]
			if !ok {
				// 不需要加密的字段提前跳出循环
				continue
			}
			updateV := updateInfo[updateColumn]
			var newValue string
			newValue, db.Error = mt.encrypt(utils.ToString(updateV))
			if db.Error != nil {
				return
			}
			updateInfo[updateColumn] = newValue
		}
		return
	}
	typeOf, valueOf := mt.getReflectElem(db.Statement.Dest)

	if typeOf != nil {
		for i := 0; i < typeOf.NumField(); i++ {
			field := typeOf.Field(i)
			_, ok := mt.needEncryptFields[field.Name]
			if !ok {
				continue
			}
			val := valueOf.Field(i).String()
			if len(val) == 0 {
				continue
			}
			var cipherTxt string
			cipherTxt, db.Error = mt.encrypt(utils.ToString(val))
			valueOf.Field(i).SetString(cipherTxt)
		}
	}
}
