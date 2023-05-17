# multi-tenancy
gorm多租户插件，实现租户数据库层面的数据隔离

### 说明

场景的数据隔离方案：

- 独立数据库

  为每一个租户配置一个独立的数据库，各租户之间的业务数据完全隔离。

- 共享数据库，独立 Schema

  不同的租户使用同一个数据库，每个租户独立分配一个Schema（也可叫做一个user）

- 共享数据库，共享 Schema，共享数据表

  在数据表中新增`TenantID`字段，通过字段进行数据隔离

### 安装

```bash
go get -u github.com/melf-xyzh/multi-tenancy
```

### 使用方法

初始化多个数据库链接

```go
dsn := "root:123456789@tcp(127.0.0.1:3306)/test1?charset=utf8mb4&parseTime=True&loc=Local"
db1, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})

dsn = "root:123456789@tcp(127.0.0.1:3306)/test2?charset=utf8mb4&parseTime=True&loc=Local"
db2, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})

dsn = "root:123456789@tcp(127.0.0.1:3306)/test3?charset=utf8mb4&parseTime=True&loc=Local"
db3, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
```

注册并使用插件

```go
dbMap := map[string]*gorm.DB{
    "1": db1,
    "2": db2,
}
mt := &plugin.MultiTenancy{}
mt.Register(dbMap, "merchant_no")
mt.AddDB("3", db3)
db1.Use(mt)
```

简单使用

```go
type User struct {
	gorm.Model
	MerchantNo string
}

func (User) TableName() string {
	return "user_table_name"
}

// 可以通过实现这个方法控制对该表是否开启数据隔离
func (User) DataIsolation() bool {
	return true
}
```





