package repository

import (
	"context"

	"gorm.io/gorm"

	"somepro/internal/model"
)

// LockerRepository 快递柜仓储
type LockerRepository struct {
	db *gorm.DB
}

// DB 获取底层 DB 对象（用于复杂查询）
func (r *LockerRepository) DB() *gorm.DB {
	return r.db
}

func NewLockerRepository(db *gorm.DB) *LockerRepository {
	return &LockerRepository{db: db}
}

// CreateCell 创建格口
func (r *LockerRepository) CreateCell(ctx context.Context, cell *model.LockerCell) error {
	return r.db.WithContext(ctx).Create(cell).Error
}

// FindCellByNo 根据编号查找格口（包含已删除的）
func (r *LockerRepository) FindCellByNo(ctx context.Context, cellNo string) (*model.LockerCell, error) {
	var cell model.LockerCell
	if err := r.db.WithContext(ctx).First(&cell, "cell_no = ?", cellNo).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &cell, nil
}

// FindActiveCellByNo 根据编号查找激活状态的格口
func (r *LockerRepository) FindActiveCellByNo(ctx context.Context, cellNo string) (*model.LockerCell, error) {
	var cell model.LockerCell
	if err := r.db.WithContext(ctx).First(&cell, "cell_no = ? AND deleted_at IS NULL AND status != 'PAUSED'", cellNo).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &cell, nil
}

// FindCellByID 根据ID查找格口（用于检查格口是否存在）
func (r *LockerRepository) FindCellByID(ctx context.Context, cellID uint) (*model.LockerCell, error) {
	var cell model.LockerCell
	if err := r.db.WithContext(ctx).First(&cell, "id = ? AND deleted_at IS NULL", cellID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &cell, nil
}

// FindCellByIDUnscoped 无条件查找格口（包括已删除的）
func (r *LockerRepository) FindCellByIDUnscoped(ctx context.Context, cellID uint) (*model.LockerCell, error) {
	var cell model.LockerCell
	if err := r.db.WithContext(ctx).Unscoped().First(&cell, "id = ?", cellID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &cell, nil
}

// FindParcelByNo 根据包裹单号查找包裹
func (r *LockerRepository) FindParcelByNo(ctx context.Context, parcelNo string) (*model.Parcel, error) {
	var parcel model.Parcel
	if err := r.db.WithContext(ctx).First(&parcel, "parcel_no = ?", parcelNo).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &parcel, nil
}

// FindStoredParcelByNo 查找状态为已存入的包裹
func (r *LockerRepository) FindStoredParcelByNo(ctx context.Context, parcelNo string) (*model.Parcel, error) {
	var parcel model.Parcel
	if err := r.db.WithContext(ctx).First(&parcel, "parcel_no = ? AND status = 'STORED' AND deleted_at IS NULL", parcelNo).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &parcel, nil
}

// FindStoredParcelByCellID 查找格口中的已存入包裹
func (r *LockerRepository) FindStoredParcelByCellID(ctx context.Context, cellID uint) (*model.Parcel, error) {
	var parcel model.Parcel
	if err := r.db.WithContext(ctx).First(&parcel, "cell_id = ? AND status = 'STORED' AND deleted_at IS NULL", cellID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &parcel, nil
}

// CreateParcel 创建包裹记录
func (r *LockerRepository) CreateParcel(ctx context.Context, parcel *model.Parcel) error {
	return r.db.WithContext(ctx).Create(parcel).Error
}

// UpdateParcel 更新包裹状态
func (r *LockerRepository) UpdateParcel(ctx context.Context, parcel *model.Parcel) error {
	return r.db.WithContext(ctx).Save(parcel).Error
}

// FindStoredParcelsByPhone 分页查询用户的已存入包裹
func (r *LockerRepository) FindStoredParcelsByPhone(ctx context.Context, phone string, pageNum, pageSize int) ([]model.Parcel, int64, error) {
	if pageNum < 1 {
		pageNum = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	var total int64
	cq := r.db.WithContext(ctx).Model(&model.Parcel{}).Where("phone = ? AND status = 'STORED' AND deleted_at IS NULL", phone)
	if err := cq.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var rows []model.Parcel
	q := r.db.WithContext(ctx).Model(&model.Parcel{}).Where("phone = ? AND status = 'STORED' AND deleted_at IS NULL", phone)
	if err := q.Order("id DESC").Offset((pageNum - 1) * pageSize).Limit(pageSize).Find(&rows).Error; err != nil {
		return nil, 0, err
	}

	return rows, total, nil
}