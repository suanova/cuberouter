package model

import (
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/common"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// legacyVideoPriceOptionJSON 是迁移前(¥/s 语义)的存量 VideoPrice option 样例。
const legacyVideoPriceOptionJSON = `{"viduq3-pro":{"rows":[{"resolution":"1080p","normal_price":0.75,"off_peak_price":0.375},{"resolution":"720p","normal_price":0.625,"off_peak_price":0.3125}]}}`

func seedVideoPriceOption(t *testing.T, db *gorm.DB, value string) {
	t.Helper()
	require.NoError(t, db.Create(&Option{Key: "VideoPrice", Value: value}).Error)
}

func readOptionValue(t *testing.T, db *gorm.DB, key string) (string, bool) {
	t.Helper()
	var opt Option
	err := db.Where(&Option{Key: key}).First(&opt).Error
	if err != nil {
		return "", false
	}
	return opt.Value, true
}

// deleteVideoPriceMigrationOptions 清掉本测试写入的种子行。SQLite :memory: 上
// 是无害空操作;MySQL/PostgreSQL 变体共用真实 options 表,先清后清保证幂等可测。
func deleteVideoPriceMigrationOptions(t *testing.T, db *gorm.DB) {
	t.Helper()
	for _, key := range []string{"VideoPrice", videoPriceUsdMigratedKey} {
		require.NoError(t, db.Where(&Option{Key: key}).Delete(&Option{}).Error)
	}
}

// requireMigratedVideoPriceValues 校验 VideoPrice 行已被 ÷7.3 且标记已写入。
func requireMigratedVideoPriceValues(t *testing.T, db *gorm.DB) {
	t.Helper()
	var m map[string]struct {
		Rows []struct {
			Resolution   string  `json:"resolution"`
			NormalPrice  float64 `json:"normal_price"`
			OffPeakPrice float64 `json:"off_peak_price"`
		} `json:"rows"`
	}
	value, ok := readOptionValue(t, db, "VideoPrice")
	require.True(t, ok)
	require.NoError(t, common.Unmarshal([]byte(value), &m))
	rows := m["viduq3-pro"].Rows
	require.Len(t, rows, 2)
	assert.InDelta(t, 0.75/7.3, rows[0].NormalPrice, 1e-12)
	assert.InDelta(t, 0.375/7.3, rows[0].OffPeakPrice, 1e-12)
	assert.InDelta(t, 0.625/7.3, rows[1].NormalPrice, 1e-12)
	assert.InDelta(t, 0.3125/7.3, rows[1].OffPeakPrice, 1e-12)
	_, marked := readOptionValue(t, db, videoPriceUsdMigratedKey)
	assert.True(t, marked)
}

// videoPriceUsdMigrationNilTableScenario 存量 VideoPrice 含 nil 表条目(合法 JSON,
// 如 {"viduq3-pro":null}):不得 panic,nil 表被跳过,标记照常写入,无价格行被改写。
func videoPriceUsdMigrationNilTableScenario(t *testing.T, db *gorm.DB) {
	t.Helper()
	deleteVideoPriceMigrationOptions(t, db)
	t.Cleanup(func() { deleteVideoPriceMigrationOptions(t, db) })

	seedVideoPriceOption(t, db, `{"viduq3-pro":null}`)

	// 两遍:验证幂等,且 nil 表条目不 panic、不产生任何 ÷7.3 改写
	for range 2 {
		require.NotPanics(t, func() { require.NoError(t, migrateVideoPriceUsdToUSD(db)) })
	}
	_, marked := readOptionValue(t, db, videoPriceUsdMigratedKey)
	assert.True(t, marked)
	// 无数据行被改写:值保持原 JSON 语义(null 表,不含价格行)
	value, ok := readOptionValue(t, db, "VideoPrice")
	require.True(t, ok)
	require.JSONEq(t, `{"viduq3-pro":null}`, value)
}

func videoPriceUsdMigrationWithDataScenario(t *testing.T, db *gorm.DB) {
	t.Helper()
	deleteVideoPriceMigrationOptions(t, db)
	t.Cleanup(func() { deleteVideoPriceMigrationOptions(t, db) })

	seedVideoPriceOption(t, db, legacyVideoPriceOptionJSON)

	// 两遍:验证幂等(二次不得再 ÷7.3)
	for range 2 {
		require.NoError(t, migrateVideoPriceUsdToUSD(db))
	}
	requireMigratedVideoPriceValues(t, db)
}

func videoPriceUsdMigrationNoDataScenario(t *testing.T, db *gorm.DB) {
	t.Helper()
	deleteVideoPriceMigrationOptions(t, db)
	t.Cleanup(func() { deleteVideoPriceMigrationOptions(t, db) })

	for range 2 {
		require.NoError(t, migrateVideoPriceUsdToUSD(db))
	}
	_, marked := readOptionValue(t, db, videoPriceUsdMigratedKey)
	assert.True(t, marked)
	// 不引入 VideoPrice 行
	_, hasVideo := readOptionValue(t, db, "VideoPrice")
	assert.False(t, hasVideo)
}

// runMigrateVideoPriceWorkers 并行执行 N 次迁移并收集各自错误。并发"多主节点
// 同时启动"时败者必须干净跳过(成功返回),而不是撞唯一键/死锁报错中止启动。
func runMigrateVideoPriceWorkers(t *testing.T, db *gorm.DB, workers int) {
	t.Helper()
	errs := make(chan error, workers)
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- migrateVideoPriceUsdToUSD(db)
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
}

// videoPriceUsdMigrationConcurrentScenario 存量行竞态:所有节点成功、价格只 ÷7.3
// 一次(败者在数据行锁上阻塞后复查标记跳过)。
func videoPriceUsdMigrationConcurrentScenario(t *testing.T, db *gorm.DB) {
	t.Helper()
	deleteVideoPriceMigrationOptions(t, db)
	t.Cleanup(func() { deleteVideoPriceMigrationOptions(t, db) })

	seedVideoPriceOption(t, db, legacyVideoPriceOptionJSON)
	runMigrateVideoPriceWorkers(t, db, 4)
	requireMigratedVideoPriceValues(t, db)
}

// videoPriceUsdMigrationConcurrentNoDataScenario 全新库竞态(无存量行,无数据行
// 锁可串行):标记写入由唯一键 + ON CONFLICT DO NOTHING 兜底,所有节点成功。
func videoPriceUsdMigrationConcurrentNoDataScenario(t *testing.T, db *gorm.DB) {
	t.Helper()
	deleteVideoPriceMigrationOptions(t, db)
	t.Cleanup(func() { deleteVideoPriceMigrationOptions(t, db) })

	runMigrateVideoPriceWorkers(t, db, 4)
	_, marked := readOptionValue(t, db, videoPriceUsdMigratedKey)
	assert.True(t, marked)
	_, hasVideo := readOptionValue(t, db, "VideoPrice")
	assert.False(t, hasVideo)
}

func TestMigrateVideoPriceUsdToUSDSQLite(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Option{}))
	videoPriceUsdMigrationWithDataScenario(t, db)
	videoPriceUsdMigrationNilTableScenario(t, db)
}

func TestMigrateVideoPriceUsdToUSDNoDataSQLite(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Option{}))
	videoPriceUsdMigrationNoDataScenario(t, db)
}

func TestMigrateVideoPriceUsdToUSDMySQL(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("TEST_MYSQL_DSN"))
	if dsn == "" {
		t.Skip("TEST_MYSQL_DSN is not configured")
	}

	common.SetDatabaseTypes(common.DatabaseTypeMySQL, common.DatabaseTypeSQLite)
	t.Cleanup(func() { common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite) })

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })

	require.NoError(t, db.AutoMigrate(&Option{}))
	videoPriceUsdMigrationWithDataScenario(t, db)
	videoPriceUsdMigrationNoDataScenario(t, db)
	videoPriceUsdMigrationNilTableScenario(t, db)
	videoPriceUsdMigrationConcurrentScenario(t, db)
	videoPriceUsdMigrationConcurrentNoDataScenario(t, db)
}

func TestMigrateVideoPriceUsdToUSDPostgreSQL(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("TEST_POSTGRES_DSN"))
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN is not configured")
	}

	common.SetDatabaseTypes(common.DatabaseTypePostgreSQL, common.DatabaseTypeSQLite)
	t.Cleanup(func() { common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite) })

	db, err := gorm.Open(postgres.New(postgres.Config{
		DSN:                  dsn,
		PreferSimpleProtocol: true,
	}), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })

	require.NoError(t, db.AutoMigrate(&Option{}))
	videoPriceUsdMigrationWithDataScenario(t, db)
	videoPriceUsdMigrationNoDataScenario(t, db)
	videoPriceUsdMigrationNilTableScenario(t, db)
	videoPriceUsdMigrationConcurrentScenario(t, db)
	videoPriceUsdMigrationConcurrentNoDataScenario(t, db)
}
