package controller

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const bepusdtTestToken = "test-token"

// bepusdtPaidCallback was produced by BEpusdt's own notify code
// (notify.EpNotify + utils.EpusdtSign) with the token "test-token".
const bepusdtPaidCallback = `{"trade_id":"b1a2c3","order_id":"USR1NOabc1231700000000","amount":73,"actual_amount":"10.27","token":"TUFmR4PLRikaRoeZKS3F879fw9uQf186hK","block_transaction_id":"0xhash","signature":"109d2e4e0108083e1aabd3c732de6c6c","status":2}`

// setupBEpusdtTest configures BEpusdt against gatewayURL and a database
// holding user 1 (quota 0) in the default group. The database is a fresh
// SQLite file unless TEST_BEPUSDT_SQL_DSN points at MySQL or PostgreSQL.
func setupBEpusdtTest(t *testing.T, gatewayURL string) {
	t.Helper()
	confirmPaymentComplianceForTest(t)
	originalAddress := setting.BEpusdtAddress
	originalToken := setting.BEpusdtToken
	originalMethods := setting.BEpusdtPayMethods
	originalServerAddress := system_setting.ServerAddress
	originalQuotaPerUnit := common.QuotaPerUnit
	originalPrice := operation_setting.Price
	originalDisplayType := operation_setting.GetGeneralSetting().QuotaDisplayType
	originalDB, originalLogDB := model.DB, model.LOG_DB
	originalType, originalLogType := common.MainDatabaseType(), common.LogDatabaseType()
	originalMaster, originalCache, originalRedis, originalSQLite := common.IsMasterNode, common.MemoryCacheEnabled, common.RedisEnabled, common.SQLitePath
	t.Cleanup(func() {
		setting.BEpusdtAddress = originalAddress
		setting.BEpusdtToken = originalToken
		setting.BEpusdtPayMethods = originalMethods
		system_setting.ServerAddress = originalServerAddress
		common.QuotaPerUnit = originalQuotaPerUnit
		operation_setting.Price = originalPrice
		operation_setting.GetGeneralSetting().QuotaDisplayType = originalDisplayType
		model.DB, model.LOG_DB = originalDB, originalLogDB
		common.SetDatabaseTypes(originalType, originalLogType)
		common.IsMasterNode, common.MemoryCacheEnabled, common.RedisEnabled, common.SQLitePath = originalMaster, originalCache, originalRedis, originalSQLite
	})

	t.Setenv("SQL_DSN", os.Getenv("TEST_BEPUSDT_SQL_DSN"))
	t.Setenv("LOG_SQL_DSN", "")
	common.IsMasterNode, common.MemoryCacheEnabled, common.RedisEnabled = false, false, false
	common.SQLitePath = filepath.Join(t.TempDir(), "bepusdt.db")
	require.NoError(t, model.InitDB())
	db := model.DB
	sqlDB, err := db.DB()
	require.NoError(t, err)
	model.LOG_DB = db
	common.SetLogDatabaseType(common.MainDatabaseType())
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.TopUp{}, &model.Log{}))
	clearRows := func() {
		require.NoError(t, db.Where("user_id = ?", 1).Delete(&model.TopUp{}).Error)
		require.NoError(t, db.Unscoped().Where("id = ?", 1).Delete(&model.User{}).Error)
	}
	clearRows()
	t.Cleanup(func() {
		clearRows()
		require.NoError(t, sqlDB.Close())
	})

	setting.BEpusdtAddress = gatewayURL
	setting.BEpusdtToken = bepusdtTestToken
	setting.BEpusdtPayMethods = `[{"name":"USDT-TRC20","type":"usdt.trc20","min_topup":"10"}]`
	system_setting.ServerAddress = "https://dashboard.example.com"
	common.QuotaPerUnit = 500000
	operation_setting.Price = 7.3
	operation_setting.GetGeneralSetting().QuotaDisplayType = operation_setting.QuotaDisplayTypeUSD
	require.NoError(t, db.Create(&model.User{Id: 1, Username: "usdt_user", Group: "default", Status: common.UserStatusEnabled}).Error)
}

func sendBEpusdtCallback(t *testing.T, body string) *httptest.ResponseRecorder {
	t.Helper()
	router := gin.New()
	router.POST("/api/user/bepusdt/notify", BEpusdtNotify)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/user/bepusdt/notify", bytes.NewBufferString(body)))
	return recorder
}

func userQuota(t *testing.T) int {
	t.Helper()
	var user model.User
	require.NoError(t, model.DB.First(&user, 1).Error)
	return user.Quota
}

func TestBEpusdtNotifyCreditsVerifiedPaymentOnce(t *testing.T) {
	setupBEpusdtTest(t, "https://pay.example.com")
	require.NoError(t, model.DB.Create(&model.TopUp{
		UserId: 1, Amount: 10, Money: 73, TradeNo: "USR1NOabc1231700000000",
		PaymentMethod: "usdt.trc20", PaymentProvider: model.PaymentProviderBEpusdt, Status: common.TopUpStatusPending,
	}).Error)

	tampered := bytes.Replace([]byte(bepusdtPaidCallback), []byte(`"amount":73`), []byte(`"amount":730`), 1)
	rejected := sendBEpusdtCallback(t, string(tampered))
	assert.Equal(t, http.StatusUnauthorized, rejected.Code)
	assert.Equal(t, 0, userQuota(t))

	first := sendBEpusdtCallback(t, bepusdtPaidCallback)
	assert.Equal(t, http.StatusOK, first.Code)
	assert.Equal(t, 10*500000, userQuota(t))

	duplicate := sendBEpusdtCallback(t, bepusdtPaidCallback)
	assert.Equal(t, http.StatusOK, duplicate.Code)
	assert.Equal(t, 10*500000, userQuota(t))
	assert.Equal(t, common.TopUpStatusSuccess, model.GetTopUpByTradeNo("USR1NOabc1231700000000").Status)
}

func TestBEpusdtNotifyRejectsPaymentForDifferentAmount(t *testing.T) {
	setupBEpusdtTest(t, "https://pay.example.com")
	require.NoError(t, model.DB.Create(&model.TopUp{
		UserId: 1, Amount: 100, Money: 730, TradeNo: "USR1NOabc1231700000000",
		PaymentMethod: "usdt.trc20", PaymentProvider: model.PaymentProviderBEpusdt, Status: common.TopUpStatusPending,
	}).Error)

	recorder := sendBEpusdtCallback(t, bepusdtPaidCallback)

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.Equal(t, 0, userQuota(t))
	assert.Equal(t, common.TopUpStatusPending, model.GetTopUpByTradeNo("USR1NOabc1231700000000").Status)
}

func TestBEpusdtNotifyMarksExpiredOrder(t *testing.T) {
	setupBEpusdtTest(t, "https://pay.example.com")
	require.NoError(t, model.DB.Create(&model.TopUp{
		UserId: 1, Amount: 10, Money: 73, TradeNo: "USR1NOexpired",
		PaymentMethod: "usdt.trc20", PaymentProvider: model.PaymentProviderBEpusdt, Status: common.TopUpStatusPending,
	}).Error)
	payload := map[string]any{"trade_id": "t1", "order_id": "USR1NOexpired", "amount": 73, "actual_amount": "10.27", "token": "T", "status": 3}
	payload["signature"] = bepusdtSign(payload, bepusdtTestToken)
	body, err := common.Marshal(payload)
	require.NoError(t, err)

	recorder := sendBEpusdtCallback(t, string(body))

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, common.TopUpStatusExpired, model.GetTopUpByTradeNo("USR1NOexpired").Status)
	assert.Equal(t, 0, userQuota(t))
}

func TestRequestBEpusdtPay(t *testing.T) {
	testCases := []struct {
		name          string
		gatewayReply  string
		amount        int64
		wantUSDT      bool
		wantStatus    string
		wantNoGateway bool
	}{
		{
			name:         "gateway accepts the order",
			gatewayReply: `{"status_code":200,"message":"success","data":{"trade_id":"tid1","trade_type":"usdt.trc20","actual_amount":"10.27","token":"TUFmR4PLRikaRoeZKS3F879fw9uQf186hK","expiration_time":1799}}`,
			amount:       10,
			wantUSDT:     true,
			wantStatus:   common.TopUpStatusPending,
		},
		{
			name:         "gateway rejection fails the order",
			gatewayReply: `{"status_code":400,"message":"no wallet"}`,
			amount:       10,
			wantStatus:   common.TopUpStatusFailed,
		},
		{
			name:          "amount below the method minimum never reaches the gateway",
			amount:        5,
			wantNoGateway: true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var gatewayBody map[string]any
			gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				raw, err := io.ReadAll(r.Body)
				require.NoError(t, err)
				require.NoError(t, common.Unmarshal(raw, &gatewayBody))
				_, _ = w.Write([]byte(tc.gatewayReply))
			}))
			t.Cleanup(gateway.Close)
			setupBEpusdtTest(t, gateway.URL)

			router := gin.New()
			router.POST("/pay", func(c *gin.Context) { c.Set("id", 1) }, RequestBEpusdtPay)
			recorder := httptest.NewRecorder()
			body := `{"amount":` + common.Interface2String(tc.amount) + `,"payment_method":"usdt.trc20"}`
			router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/pay", bytes.NewBufferString(body)))
			require.Equal(t, http.StatusOK, recorder.Code)

			var response struct {
				Message string          `json:"message"`
				Data    json.RawMessage `json:"data"`
			}
			require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))

			if tc.wantNoGateway {
				assert.Nil(t, gatewayBody)
				assert.Equal(t, "error", response.Message)
				return
			}
			require.NotNil(t, gatewayBody)
			assert.Equal(t, bepusdtSign(gatewayBody, bepusdtTestToken), gatewayBody["signature"])
			assert.Equal(t, 73.0, gatewayBody["amount"])
			assert.Equal(t, "CNY", gatewayBody["fiat"])
			assert.Equal(t, "https://dashboard.example.com/api/user/bepusdt/notify", gatewayBody["notify_url"])

			var topUp model.TopUp
			require.NoError(t, model.DB.Where("trade_no = ?", gatewayBody["order_id"]).First(&topUp).Error)
			assert.Equal(t, tc.wantStatus, topUp.Status)
			assert.Equal(t, model.PaymentProviderBEpusdt, topUp.PaymentProvider)
			assert.Equal(t, 73.0, topUp.Money)

			if !tc.wantUSDT {
				assert.Equal(t, "error", response.Message)
				return
			}
			assert.Equal(t, "success", response.Message)
			var data struct {
				USDT map[string]any `json:"usdt"`
			}
			require.NoError(t, common.Unmarshal(response.Data, &data))
			assert.Equal(t, topUp.TradeNo, data.USDT["trade_no"])
			assert.Equal(t, "TUFmR4PLRikaRoeZKS3F879fw9uQf186hK", data.USDT["address"])
			assert.Equal(t, "10.27", data.USDT["amount"])
			assert.Equal(t, "73.00", data.USDT["money"])
		})
	}
}
