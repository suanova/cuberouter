package model

import (
	"errors"
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// videoPriceUsdMigratedKey 标记 VideoPrice 已从 ¥/s 迁移为 USD/s。
const videoPriceUsdMigratedKey = "VideoPriceUsdMigrated"

// legacyVideoUsdRate 2026-09-08 前视频价格表以 ¥/秒 存储,计费时 ÷7.3 折算;
// 迁移把存量值 ÷7.3 写回为 USD/s,之后 VideoPriceModelPrice 不再折算。
const legacyVideoUsdRate = 7.3

// migrateVideoPriceUsdToUSD 把 options 表中 VideoPrice option 的价格字段从 ¥/s
// 改写为 USD/s(每个 normal_price / off_peak_price ÷7.3)。幂等:
//   - 完成标记(videoPriceUsdMigratedKey)已存在 → 直接返回;
//   - 有存量行时,事务内先对 VideoPrice 行取 FOR UPDATE、拿到行锁后再复查标记
//     (inspect-after-lock,同 prefill_group_migration.go):存量数据行锁才是多主
//     节点并发启动时真正的串行点 —— 标记行通常不存在,对不存在的行 FOR UPDATE
//     在 PostgreSQL 上锁不住任何东西,在 InnoDB 里只会取彼此兼容的 gap lock;
//     锁等待期间其他节点已完成时,后到者复查到标记即干净跳过,不会二次 ÷7.3;
//   - 无存量行(全新库)时,探针事务先回滚释放 InnoDB REPEATABLE READ 下探针
//     遗留的 gap lock,再在独立事务里写标记:不加锁的 INSERT 只由 options
//     主键唯一约束串行,ON CONFLICT DO NOTHING 静默吸收"其他节点已完成"
//     的并发写入(MySQL 生成无副作用的 ON DUPLICATE KEY UPDATE,PG/SQLite
//     生成 ON CONFLICT DO NOTHING)。
//   - VideoPrice 值不是合法 JSON 或结构不符时返回错误、中止启动:这是有意
//     fail-stop —— 该 option 由管理端定价编辑器写库,坏值说明被外部改坏/数据
//     损坏,静默跳过会让存量价格继续按新语义计费而错算;合法 JSON 中个别表
//     条目为 null(如 {"viduq3-pro":null})则跳过该表并记 SysLog,不影响其余表
//     折算,也不 panic。
//
// SQLite 无 FOR UPDATE 语法,lockForUpdate 自动降级为普通读(SQLite 单写者)。
//
// 必须在 AutoMigrate(&Option{}) 之后、loadOptionsFromDatabase 之前运行。
func migrateVideoPriceUsdToUSD(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	if !db.Migrator().HasTable(&Option{}) {
		return nil
	}

	// 快路径:标记已存在则无需任何处理(无需起事务)
	var marker Option
	if err := db.Where(&Option{Key: videoPriceUsdMigratedKey}).First(&marker).Error; err == nil {
		return nil
	}

	err := db.Transaction(func(tx *gorm.DB) error {
		var opt Option
		if err := lockForUpdate(tx).Where(&Option{Key: "VideoPrice"}).First(&opt).Error; err != nil {
			return err
		}
		// 已持有存量行锁(串行点):复查标记,锁等待期间可能已有其他节点完成
		var lockedMarker Option
		if err := lockForUpdate(tx).Where(&Option{Key: videoPriceUsdMigratedKey}).First(&lockedMarker).Error; err == nil {
			return nil
		}

		m, err := ratio_setting.ParseVideoPriceMap(opt.Value)
		if err != nil {
			return fmt.Errorf("migrate video price: parse VideoPrice option: %w", err)
		}
		if len(m) > 0 {
			for tableKey, table := range m {
				// 合法 JSON 允许顶层条目为 null(如 {"viduq3-pro":null}):nil 表没有
				// 行可 ÷7.3,跳过即可,不能让迁移在启动路径上 panic。
				if table == nil {
					common.SysLog("migrate video price: skip nil price table entry for model " + tableKey)
					continue
				}
				for i := range table.Rows {
					table.Rows[i].NormalPrice /= legacyVideoUsdRate
					table.Rows[i].OffPeakPrice /= legacyVideoUsdRate
				}
			}
			raw, err := common.Marshal(m)
			if err != nil {
				return fmt.Errorf("migrate video price: marshal: %w", err)
			}
			if err := tx.Model(&Option{}).Where(&Option{Key: "VideoPrice"}).Update("value", string(raw)).Error; err != nil {
				return err
			}
			common.SysLog("migrated VideoPrice option from ¥/s to USD/s")
		}
		// 并发兜底:正常不可能冲突(持有存量行锁期间其他节点进不来),冲突即其他
		// 节点已完成,静默跳过即可。
		return tx.Clauses(clause.OnConflict{DoNothing: true}).
			Create(&Option{Key: videoPriceUsdMigratedKey, Value: "1"}).Error
	})
	if err == nil {
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	// 无存量 VideoPrice 行(全新库或从未配置):上面的探针事务已回滚,不能在同一
	// 事务里直接写标记 —— InnoDB REPEATABLE READ 下对不存在行的 FOR UPDATE 会
	// 遗留覆盖标记位置的 gap lock,两节点并发双插互相等待即死锁;独立事务里
	// 不加锁的 INSERT 只撞主键唯一约束,由 ON CONFLICT DO NOTHING 干净吸收。
	return db.Transaction(func(tx *gorm.DB) error {
		return tx.Clauses(clause.OnConflict{DoNothing: true}).
			Create(&Option{Key: videoPriceUsdMigratedKey, Value: "1"}).Error
	})
}
