package model

type SyncState struct {
	ID           uint   `gorm:"primaryKey"`
	LastBlockNum uint64 `gorm:"column:last_block_num"` // 记录最后处理成功的高度
}