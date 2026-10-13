package model

import (
	"time"

	"gorm.io/gorm"
)

// LockerCellStatus 格口状态
const (
	LockerCellStatusFree   = "FREE"   // 空闲
	LockerCellStatusPaused = "PAUSED" // 停用
)

// LockerCellSize 格口尺寸档位
const (
	LockerCellSizeSmall   = "SMALL"
	LockerCellSizeMedium  = "MEDIUM"
	LockerCellSizeLarge   = "LARGE"
)

// ParcelStatus 包裹状态
const (
	ParcelStatusStored = "STORED" // 已存入
	ParcelStatusPicked = "PICKED" // 已取出
)

// LockerCellSizeOrder 尺寸档位大小顺序
var LockerCellSizeOrder = map[string]int{
	LockerCellSizeSmall:   1,
	LockerCellSizeMedium:  2,
	LockerCellSizeLarge:   3,
}

// LockerCell 快递柜格口
type LockerCell struct {
	ID          uint           `gorm:"primaryKey;column:id" json:"id"`
	CellNo      string         `gorm:"column:cell_no;size:32;not null;uniqueIndex" json:"cellNo"`
	Size        string         `gorm:"column:size;size:16;not null" json:"size"`
	Status      string         `gorm:"column:status;size:16;not null;default:FREE" json:"status"`
	CreateBy    string         `gorm:"column:create_by;size:64" json:"-"`
	CreateTime  time.Time      `gorm:"column:create_time" json:"-"`
	UpdateBy    string         `gorm:"column:update_by;size:64" json:"-"`
	UpdateTime  time.Time      `gorm:"column:update_time" json:"-"`
	DeletedAt   gorm.DeletedAt `gorm:"column:deleted_at;index" json:"-"`
}

func (LockerCell) TableName() string { return "t_locker_cell" }

func (c *LockerCell) BeforeCreate(tx *gorm.DB) error {
	op := OperatorFrom(tx.Statement.Context)
	now := time.Now()
	c.CreateBy, c.CreateTime = op, now
	c.UpdateBy, c.UpdateTime = op, now
	return nil
}

func (c *LockerCell) BeforeUpdate(tx *gorm.DB) error {
	c.UpdateBy, c.UpdateTime = OperatorFrom(tx.Statement.Context), time.Now()
	return nil
}

// Parcel 快递包裹
type Parcel struct {
	ID          uint           `gorm:"primaryKey;column:id" json:"id"`
	ParcelNo    string         `gorm:"column:parcel_no;size:32;not null;uniqueIndex" json:"parcelNo"`
	CellID      uint           `gorm:"column:cell_id;not null" json:"cellId"`
	Phone       string         `gorm:"column:phone;size:32;not null" json:"phone"`
	Size        string         `gorm:"column:size;size:16;not null" json:"size"`
	Status      string         `gorm:"column:status;size:16;not null;default:STORED" json:"status"`
	StoredAt    time.Time      `gorm:"column:stored_at" json:"storedAt"`
	PickedAt    time.Time      `gorm:"column:picked_at" json:"pickedAt"`
	CreateBy    string         `gorm:"column:create_by;size:64" json:"-"`
	CreateTime  time.Time      `gorm:"column:create_time" json:"-"`
	UpdateBy    string         `gorm:"column:update_by;size:64" json:"-"`
	UpdateTime  time.Time      `gorm:"column:update_time" json:"-"`
	DeletedAt   gorm.DeletedAt `gorm:"column:deleted_at;index" json:"-"`
}

func (Parcel) TableName() string { return "t_parcel" }

func (p *Parcel) BeforeCreate(tx *gorm.DB) error {
	op := OperatorFrom(tx.Statement.Context)
	now := time.Now()
	p.CreateBy, p.CreateTime = op, now
	p.UpdateBy, p.UpdateTime = op, now
	p.StoredAt = now
	return nil
}

func (p *Parcel) BeforeUpdate(tx *gorm.DB) error {
	p.UpdateBy, p.UpdateTime = OperatorFrom(tx.Statement.Context), time.Now()
	return nil
}