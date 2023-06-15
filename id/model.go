/**
 * @Time    :2023/5/25 11:22
 * @Author  :Xiaoyu.Zhang
 */

package id

type Model struct {
	ID         DistributedId `json:"id"                     form:"id"     gorm:"column:id;primary_key;type:bigint"`
	CreateTime string        `json:"createTime"             form:"-"       gorm:"column:create_time;index;type:varchar(20)"`
	UpdateTime string        `json:"updateTime,omitempty"   form:"-"       gorm:"column:update_time;type:varchar(20)"`
}
