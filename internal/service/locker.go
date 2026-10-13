package service

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"somepro/internal/model"
	"somepro/internal/repository"
)

// redisLockKey 格口锁的 Redis key 前缀
const redisLockKeyPrefix = "locker:cell:lock:"

// tryLockCell 尝试获取格口锁
func (s *LockerService) tryLockCell(ctx context.Context, cellNo string, ttl time.Duration) (bool, error) {
	key := redisLockKeyPrefix + cellNo
	err := s.rdb.SetNX(ctx, key, "1", ttl).Err()
	if err != nil {
		return false, err
	}
	return true, nil
}

// unlockCell 释放格口锁
func (s *LockerService) unlockCell(ctx context.Context, cellNo string) error {
	key := redisLockKeyPrefix + cellNo
	_, err := s.rdb.Del(ctx, key).Result()
	return err
}

// LockerService 快递柜服务
type LockerService struct {
	repo *repository.LockerRepository
	rdb  *redis.Client
}

func NewLockerService(repo *repository.LockerRepository, rdb *redis.Client) *LockerService {
	return &LockerService{repo: repo, rdb: rdb}
}

// CreateCell 创建格口
func (s *LockerService) CreateCell(ctx context.Context, cellNo, size string) (*model.LockerCell, error) {
	// 验证尺寸
	if !isValidCellSize(size) {
		return nil, errors.New("无效的尺寸档，必须是 SMALL、MEDIUM 或 LARGE")
	}

	// 检查格口是否已存在（包括已删除的）
	existing, err := s.repo.FindCellByNo(ctx, cellNo)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		if existing.DeletedAt.Time.IsZero() {
			return nil, errors.New("格口编号已存在")
		}
		// 已删除的可以重新注册
		existing.CellNo = cellNo
		existing.Size = size
		existing.Status = model.LockerCellStatusFree
		existing.DeletedAt = gorm.DeletedAt{}
		if err := s.repo.CreateCell(ctx, existing); err != nil {
			return nil, err
		}
		return existing, nil
	}

	cell := &model.LockerCell{
		CellNo: cellNo,
		Size:   size,
		Status: model.LockerCellStatusFree,
	}
	if err := s.repo.CreateCell(ctx, cell); err != nil {
		return nil, err
	}
	return cell, nil
}

// PauseCell 停用格口
func (s *LockerService) PauseCell(ctx context.Context, cellNo string) error {
	// 查找格口
	cell, err := s.repo.FindActiveCellByNo(ctx, cellNo)
	if err != nil {
		return err
	}
	if cell == nil {
		// 检查是否存在（包括已删除的）
		existing, err := s.repo.FindCellByNo(ctx, cellNo)
		if err != nil {
			return err
		}
		if existing == nil {
			return errors.New("格口不存在")
		}
		// 格口已被删除，不能停用
		return errors.New("格口已被删除")
	}

	// 检查格口中是否有已存入的包裹
	parcel, err := s.repo.FindStoredParcelByCellID(ctx, cell.ID)
	if err != nil {
		return err
	}
	if parcel != nil {
		return errors.New("格口内有包裹，不能停用")
	}

	// 更新格口状态为停用
	cell.Status = model.LockerCellStatusPaused
	if err := s.repo.CreateCell(ctx, cell); err != nil {
		return err
	}
	return nil
}

// StoreParcel 存件
func (s *LockerService) StoreParcel(ctx context.Context, parcelNo, cellNo, phone, size string) (*model.Parcel, error) {
	// 验证参数
	if parcelNo == "" {
		return nil, errors.New("包裹单号不能为空")
	}
	if phone == "" {
		return nil, errors.New("收货手机号不能为空")
	}
	if !isValidParcelSize(size) {
		return nil, errors.New("无效的包裹尺寸，必须是 SMALL、MEDIUM 或 LARGE")
	}

	// 获取分布式锁，防止并发存件
	lockCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	locked, err := s.tryLockCell(lockCtx, cellNo, 10*time.Second)
	if err != nil {
		return nil, errors.New("获取格口锁失败")
	}
	if !locked {
		return nil, errors.New("格口正在被其他操作占用，请稍后重试")
	}
	defer func() {
		_ = s.unlockCell(context.Background(), cellNo)
	}()

	// 查找格口（必须是激活状态且存在）
	cell, err := s.repo.FindActiveCellByNo(ctx, cellNo)
	if err != nil {
		return nil, err
	}
	if cell == nil {
		// 检查是否存在（包括已删除的）
		existing, err := s.repo.FindCellByNo(ctx, cellNo)
		if err != nil {
			return nil, err
		}
		if existing == nil {
			return nil, errors.New("格口不存在")
		}
		if !existing.DeletedAt.Time.IsZero() {
			return nil, errors.New("格口已被删除")
		}
		return nil, errors.New("格口已停用")
	}

	// 检查格口是否为空
	parcelInCell, err := s.repo.FindStoredParcelByCellID(ctx, cell.ID)
	if err != nil {
		return nil, err
	}
	if parcelInCell != nil {
		return nil, errors.New("格口已被占用")
	}

	// 检查包裹尺寸是否不超过格口尺寸
	if !isParcelSizeWithinLimit(size, cell.Size) {
		return nil, errors.New("包裹尺寸超过格口尺寸")
	}

	// 检查包裹单号是否已存在
	existingParcel, err := s.repo.FindParcelByNo(ctx, parcelNo)
	if err != nil {
		return nil, err
	}
	if existingParcel != nil {
		if existingParcel.Status == model.ParcelStatusPicked {
			// 已取出的包裹可以重新存
			// 这里不处理，按重复单号处理
		} else {
			return nil, errors.New("包裹单号已存在")
		}
	}

	// 创建包裹记录
	parcel := &model.Parcel{
		ParcelNo: parcelNo,
		CellID:   cell.ID,
		Phone:    phone,
		Size:     size,
		Status:   model.ParcelStatusStored,
	}
	if err := s.repo.CreateParcel(ctx, parcel); err != nil {
		return nil, err
	}

	return parcel, nil
}

// PickupParcel 取件
func (s *LockerService) PickupParcel(ctx context.Context, parcelNo string) error {
	// 查找包裹
	parcel, err := s.repo.FindStoredParcelByNo(ctx, parcelNo)
	if err != nil {
		return err
	}
	if parcel == nil {
		// 包裹不存在或已被取出，重复取件不报错
		return nil
	}

	// 更新包裹状态为已取出
	parcel.Status = model.ParcelStatusPicked
	parcel.PickedAt = time.Now()
	if err := s.repo.UpdateParcel(ctx, parcel); err != nil {
		return err
	}

	return nil
}

// ListParcelsByPhoneWithCellNo 查询用户在柜包裹（包含格口编号）
func (s *LockerService) ListParcelsByPhoneWithCellNo(ctx context.Context, phone string, pageNum, pageSize int) (map[string]interface{}, error) {
	if pageNum < 1 {
		pageNum = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	var total int64
	// 使用 JOIN 查询
	type ParcelWithCell struct {
		ParcelNo    string
		CellNo      string
		Size        string
		Phone       string
	}
	var rows []ParcelWithCell
	err := s.repo.DB().WithContext(ctx).Table("t_parcel p").
		Joins("JOIN t_locker_cell c ON p.cell_id = c.id").
		Where("p.phone = ? AND p.status = 'STORED' AND p.deleted_at IS NULL AND c.deleted_at IS NULL", phone).
		Order("p.id DESC").
		Offset((pageNum - 1) * pageSize).
		Limit(pageSize).
		Select("p.parcel_no as parcelNo, c.cell_no as cellNo, p.size as size, p.phone as phone").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	// 统计总数
	err = s.repo.DB().WithContext(ctx).Table("t_parcel p").
		Joins("JOIN t_locker_cell c ON p.cell_id = c.id").
		Where("p.phone = ? AND p.status = 'STORED' AND p.deleted_at IS NULL AND c.deleted_at IS NULL", phone).
		Count(&total).Error
	if err != nil {
		return nil, err
	}

	totalPages := 0
	if pageSize > 0 {
		totalPages = int((total + int64(pageSize) - 1) / int64(pageSize))
	}

	// 构造返回数据
	var result []map[string]interface{}
	for _, r := range rows {
		result = append(result, map[string]interface{}{
			"parcelNo": r.ParcelNo,
			"cellNo":   r.CellNo,
			"size":     r.Size,
			"phone":    r.Phone,
		})
	}

	return map[string]interface{}{
		"content":    result,
		"total":      total,
		"pageNum":    pageNum,
		"pageSize":   pageSize,
		"totalPages": totalPages,
	}, nil
}

// isValidCellSize 验证格口尺寸
func isValidCellSize(size string) bool {
	switch size {
	case model.LockerCellSizeSmall, model.LockerCellSizeMedium, model.LockerCellSizeLarge:
		return true
	}
	return false
}

// isValidParcelSize 验证包裹尺寸
func isValidParcelSize(size string) bool {
	return isValidCellSize(size)
}

// isParcelSizeWithinLimit 检查包裹尺寸是否在格口尺寸范围内
func isParcelSizeWithinLimit(parcelSize, cellSize string) bool {
	parcelOrder := model.LockerCellSizeOrder[parcelSize]
	cellOrder := model.LockerCellSizeOrder[cellSize]
	return parcelOrder <= cellOrder
}