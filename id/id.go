/**
 * @Time    :2023/5/18 16:07
 * @Author  :Xiaoyu.Zhang
 */

package id

import (
	"database/sql/driver"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"unsafe"
)

type DistributedId int64

// MarshalJSON
/**
 *  @Description: 重写MarshalJSON方法
 *  @receiver t
 *  @return []byte
 *  @return error
 */
func (t DistributedId) MarshalJSON() ([]byte, error) {
	str := strconv.FormatInt(int64(t), 10)
	// 注意 json 字符串风格要求
	return []byte(fmt.Sprintf("\"%v\"", str)), nil
}

// UnmarshalJSON
/**
 *  @Description: 重写UnmarshalJSON方法
 *  @receiver t
 *  @param b
 *  @return error
 */
func (t *DistributedId) UnmarshalJSON(b []byte) error {
	numStr := *(*string)(unsafe.Pointer(&b))
	numStr = strings.ReplaceAll(numStr, "\"", "")
	num, _ := strconv.ParseInt(numStr, 10, 64)
	*t = DistributedId(num)
	return nil
}

// Value 写入数据库之前，对数据做类型转换
func (t DistributedId) Value() (driver.Value, error) {
	// DistributedId 转换成 int64 类型
	num := int64(t)
	return num, nil
}

// Scan 将数据库中取出的数据，赋值给目标类型
func (t *DistributedId) Scan(v interface{}) error {
	switch v.(type) {
	case []uint8:
		b := v.([]uint8)
		numStr := *(*string)(unsafe.Pointer(&b))
		//numStr := utils.ToString(v.([]uint8))
		num, _ := strconv.ParseInt(numStr, 10, 64)
		*t = DistributedId(num)
	case int64:
		*t = DistributedId(v.(int64))
	default:
		val := reflect.ValueOf(v)
		typ := reflect.Indirect(val).Type()
		return errors.New(typ.Name() + "类型处理错误")
	}
	return nil
}
