/**
 * @Time    :2023/5/18 16:21
 * @Author  :Xiaoyu.Zhang
 */

package id

import "github.com/bwmarrin/snowflake"

type DistributedIdGenerator struct {
	TenantId string
	Node     *snowflake.Node
}

// InitDistributedIdGenerator
/**
 *  @Description: 初始化分布式ID生成器
 *  @param tenantId
 *  @param node
 *  @return generator
 *  @return err
 */
func InitDistributedIdGenerator(tenantId string, node int64) (generator *DistributedIdGenerator, err error) {
	newNode, nodeErr := snowflake.NewNode(node)
	if nodeErr != nil {
		err = nodeErr
		return
	}
	generator = &DistributedIdGenerator{
		TenantId: tenantId,
		Node:     newNode,
	}
	return
}

// CreateId
/**
 *  @Description: 创建一个分布式ID（雪花ID）
 *  @return DistributedId
 */
func (generator *DistributedIdGenerator) CreateId() DistributedId {
	id := generator.Node.Generate()
	return DistributedId(id.Int64())
}
