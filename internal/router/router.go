package router

import (
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"somepro/internal/handler"
	"somepro/internal/middleware"
	"somepro/internal/repository"
	"somepro/internal/service"
)

// New 组装路由。验收测试契约无关：运行时用 gin 的 Routes() 扫路由，不写死类名/方法名。
func New(db *gorm.DB, rdb *redis.Client) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery(), middleware.OperatorAuth())

	// 初始化快递柜相关仓储和服务
	lockerRepo := repository.NewLockerRepository(db)
	lockerSvc := service.NewLockerService(lockerRepo, rdb)
	lockerHdl := handler.NewLockerHandler(lockerSvc)

	// demo 路由
	itemRepo := repository.NewItem(db)
	itemSvc := service.NewItem(itemRepo, rdb)
	itemHdl := handler.NewItem(itemSvc)
	api := r.Group("/api/demo")
	{
		api.GET("/ping", itemHdl.Ping)
		api.POST("/item", itemHdl.CreateItem)
		api.GET("/list", itemHdl.List)
		api.GET("/cache", itemHdl.Cache)
	}

	// 快递柜路由
	lockerAPI := r.Group("/api/locker")
	{
		// 格口管理
		lockerAPI.POST("/cell", lockerHdl.CreateCell)          // 登记格口
		lockerAPI.POST("/cell/disable", lockerHdl.PauseCell)  // 停用格口

		// 包裹管理
		lockerAPI.POST("/parcel", lockerHdl.StoreParcel)          // 存件
		lockerAPI.POST("/parcel/pickup", lockerHdl.PickupParcel)  // 取件
		lockerAPI.GET("/parcel/list", lockerHdl.ListParcelsByPhone) // 按手机号查在柜包裹
	}

	return r
}