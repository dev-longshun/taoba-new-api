package controller

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"github.com/shopspring/decimal"
)

// BEpusdt order status values sent in callbacks.
const (
	bepusdtStatusPaid    = 2
	bepusdtStatusExpired = 3
)

// bepusdtMinTimeout is the shortest order lifetime BEpusdt accepts.
const bepusdtMinTimeout = 180

func isBEpusdtTopUpEnabled() bool {
	if !isPaymentComplianceConfirmed() {
		return false
	}
	return strings.TrimSpace(setting.BEpusdtAddress) != "" &&
		strings.TrimSpace(setting.BEpusdtToken) != "" &&
		len(setting.GetBEpusdtPayMethods()) > 0
}

// bepusdtSign implements the BEpusdt / Epusdt signature: non-empty values
// sorted by key, joined as k=v with &, followed by the API token, MD5 in hex.
// Values are formatted from their decoded JSON form, as BEpusdt does.
func bepusdtSign(params map[string]any, token string) string {
	keys := make([]string, 0, len(params))
	for key, value := range params {
		if key == "signature" || value == nil || value == "" {
			continue
		}
		keys = append(keys, key)
	}
	slices.Sort(keys)
	var builder strings.Builder
	for i, key := range keys {
		if i > 0 {
			builder.WriteString("&")
		}
		builder.WriteString(key)
		builder.WriteString("=")
		builder.WriteString(fmt.Sprintf("%v", params[key]))
	}
	builder.WriteString(token)
	sum := md5.Sum([]byte(builder.String()))
	return hex.EncodeToString(sum[:])
}

type bepusdtTransaction struct {
	TradeId        string `json:"trade_id"`
	TradeType      string `json:"trade_type"`
	ActualAmount   string `json:"actual_amount"`
	Token          string `json:"token"`
	ExpirationTime int64  `json:"expiration_time"`
}

// createBEpusdtTransaction calls BEpusdt's create-transaction API. The request
// is signed over its JSON round trip so number formatting matches the gateway.
func createBEpusdtTransaction(ctx context.Context, params map[string]any) (*bepusdtTransaction, error) {
	encoded, err := common.Marshal(params)
	if err != nil {
		return nil, err
	}
	var decoded map[string]any
	if err := common.Unmarshal(encoded, &decoded); err != nil {
		return nil, err
	}
	params["signature"] = bepusdtSign(decoded, setting.BEpusdtToken)
	body, err := common.Marshal(params)
	if err != nil {
		return nil, err
	}

	endpoint := strings.TrimRight(setting.BEpusdtAddress, "/") + "/api/v1/order/create-transaction"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bepusdt http status %d", resp.StatusCode)
	}

	var result struct {
		StatusCode int                `json:"status_code"`
		Message    string             `json:"message"`
		Data       bepusdtTransaction `json:"data"`
	}
	if err := common.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("bepusdt response decode failed: %w", err)
	}
	if result.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bepusdt rejected the order: status_code=%d message=%q", result.StatusCode, result.Message)
	}
	if result.Data.TradeId == "" || result.Data.Token == "" || result.Data.ActualAmount == "" {
		return nil, errors.New("bepusdt response misses trade_id, token or actual_amount")
	}
	return &result.Data, nil
}

func RequestBEpusdtPay(c *gin.Context) {
	if !isBEpusdtTopUpEnabled() {
		paymentError(c, common.NewMessage("Payment information has not been configured by administrator"))
		return
	}
	var req EpayRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		paymentError(c, common.NewMessage("Invalid parameters"))
		return
	}
	method, found := lo.Find(setting.GetBEpusdtPayMethods(), func(m map[string]string) bool {
		return m["type"] == req.PaymentMethod
	})
	if !found {
		paymentError(c, common.NewMessage("Payment method does not exist"))
		return
	}
	minTopup := getMinTopup()
	if methodMin, err := strconv.ParseInt(method["min_topup"], 10, 64); err == nil {
		minTopup = max(minTopup, methodMin)
	}
	if req.Amount < minTopup {
		paymentError(c, common.NewMessage("Top-up amount cannot be less than {{min}}", map[string]any{"min": minTopup}))
		return
	}
	id := c.GetInt("id")
	if rejectInvalidTopUpQuota(c, id, req.Amount) {
		return
	}
	group, err := model.GetUserGroup(id, true)
	if err != nil {
		paymentError(c, common.NewMessage("Failed to get the user group"))
		return
	}
	payMoney := decimal.NewFromFloat(getPayMoney(req.Amount, group)).Round(2)
	if payMoney.LessThan(decimal.NewFromFloat(0.01)) {
		paymentError(c, common.NewMessage("Payment amount is too low"))
		return
	}

	tradeNo := fmt.Sprintf("USR%dNO%s%d", id, common.GetRandomString(6), time.Now().Unix())
	amount := req.Amount
	if operation_setting.GetQuotaDisplayType() == operation_setting.QuotaDisplayTypeTokens {
		amount = decimal.NewFromInt(amount).Div(decimal.NewFromFloat(common.QuotaPerUnit)).IntPart()
	}
	// The order is stored before BEpusdt sees it, so every callback finds it.
	topUp := &model.TopUp{
		UserId:          id,
		Amount:          amount,
		Money:           payMoney.InexactFloat64(),
		TradeNo:         tradeNo,
		PaymentMethod:   req.PaymentMethod,
		PaymentProvider: model.PaymentProviderBEpusdt,
		CreateTime:      time.Now().Unix(),
		Status:          common.TopUpStatusPending,
	}
	if err := topUp.Insert(); err != nil {
		logger.LogError(c.Request.Context(), common.LogText("BEpusdt failed to create top-up order user_id=%d trade_no=%s payment_method=%s amount=%d error=%q", id, tradeNo, req.PaymentMethod, req.Amount, err.Error()))
		paymentError(c, common.NewMessage("Failed to create order"))
		return
	}

	transaction, err := createBEpusdtTransaction(c.Request.Context(), map[string]any{
		"order_id":     tradeNo,
		"amount":       payMoney.InexactFloat64(),
		"fiat":         "CNY",
		"trade_type":   req.PaymentMethod,
		"name":         fmt.Sprintf("TUC%d", req.Amount),
		"timeout":      max(setting.BEpusdtTimeout, bepusdtMinTimeout),
		"notify_url":   service.GetCallbackAddress() + "/api/user/bepusdt/notify",
		"redirect_url": paymentReturnPath("/wallet"),
	})
	if err != nil {
		logger.LogError(c.Request.Context(), common.LogText("BEpusdt failed to start payment user_id=%d trade_no=%s payment_method=%s amount=%d error=%q", id, tradeNo, req.PaymentMethod, req.Amount, err.Error()))
		if updateErr := model.UpdatePendingTopUpStatus(tradeNo, model.PaymentProviderBEpusdt, common.TopUpStatusFailed); updateErr != nil {
			logger.LogError(c.Request.Context(), common.LogText("BEpusdt failed to mark order failed trade_no=%s error=%q", tradeNo, updateErr.Error()))
		}
		paymentError(c, common.NewMessage("Failed to start payment"))
		return
	}

	logger.LogInfo(c.Request.Context(), common.LogText("BEpusdt top-up order created user_id=%d trade_no=%s trade_id=%s payment_method=%s amount=%d money=%s usdt=%s", id, tradeNo, transaction.TradeId, req.PaymentMethod, req.Amount, payMoney.StringFixed(2), transaction.ActualAmount))
	c.JSON(http.StatusOK, gin.H{
		"message": "success",
		"data": gin.H{
			"usdt": gin.H{
				"trade_no":   tradeNo,
				"trade_id":   transaction.TradeId,
				"trade_type": transaction.TradeType,
				"address":    transaction.Token,
				"amount":     transaction.ActualAmount,
				"money":      payMoney.StringFixed(2),
				"expire_in":  transaction.ExpirationTime,
			},
		},
	})
}

// BEpusdtNotify handles BEpusdt status callbacks. BEpusdt treats any HTTP 200
// as delivered, so rejected callbacks answer with an error status to be retried.
func BEpusdtNotify(c *gin.Context) {
	ctx := c.Request.Context()
	if !isBEpusdtTopUpEnabled() {
		logger.LogWarn(ctx, common.LogText("BEpusdt webhook rejected reason=webhook_disabled client_ip=%s", c.ClientIP()))
		c.String(http.StatusServiceUnavailable, "fail")
		return
	}
	rawBody, err := c.GetRawData()
	if err != nil {
		c.String(http.StatusBadRequest, "fail")
		return
	}
	var payload map[string]any
	if err := common.Unmarshal(rawBody, &payload); err != nil {
		logger.LogWarn(ctx, common.LogText("BEpusdt webhook body is not JSON client_ip=%s", c.ClientIP()))
		c.String(http.StatusBadRequest, "fail")
		return
	}
	signature, _ := payload["signature"].(string)
	expected := bepusdtSign(payload, setting.BEpusdtToken)
	if signature == "" || subtle.ConstantTimeCompare([]byte(signature), []byte(expected)) != 1 {
		logger.LogWarn(ctx, common.LogText("BEpusdt webhook signature verification failed client_ip=%s", c.ClientIP()))
		c.String(http.StatusUnauthorized, "fail")
		return
	}

	tradeNo, _ := payload["order_id"].(string)
	status, _ := payload["status"].(float64)
	logger.LogInfo(ctx, common.LogText("BEpusdt webhook signature verified trade_no=%s status=%v trade_id=%v actual_amount=%v tx=%v client_ip=%s", tradeNo, status, payload["trade_id"], payload["actual_amount"], payload["block_transaction_id"], c.ClientIP()))

	switch int(status) {
	case bepusdtStatusPaid:
		paidMoney, _ := payload["amount"].(float64)
		LockOrder(tradeNo)
		defer UnlockOrder(tradeNo)
		alreadyDone, err := model.RechargeBEpusdt(tradeNo, decimal.NewFromFloat(paidMoney), c.ClientIP())
		if err != nil {
			logger.LogWarn(ctx, common.LogText("BEpusdt top-up rejected trade_no=%s client_ip=%s error=%q", tradeNo, c.ClientIP(), err.Error()))
			c.String(http.StatusBadRequest, "fail")
			return
		}
		if alreadyDone {
			logger.LogInfo(ctx, common.LogText("BEpusdt duplicate callback ignored trade_no=%s", tradeNo))
		}
	case bepusdtStatusExpired:
		err := model.UpdatePendingTopUpStatus(tradeNo, model.PaymentProviderBEpusdt, common.TopUpStatusExpired)
		if err != nil && !errors.Is(err, model.ErrTopUpStatusInvalid) {
			logger.LogWarn(ctx, common.LogText("BEpusdt failed to expire order trade_no=%s error=%q", tradeNo, err.Error()))
		}
	}
	c.String(http.StatusOK, "ok")
}

// GetBEpusdtOrderStatus lets the wallet page poll its own order.
func GetBEpusdtOrderStatus(c *gin.Context) {
	topUp := model.GetTopUpByTradeNo(c.Query("trade_no"))
	if topUp == nil || topUp.UserId != c.GetInt("id") || topUp.PaymentProvider != model.PaymentProviderBEpusdt {
		paymentError(c, common.NewMessage("Order not found"))
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "success", "data": gin.H{"status": topUp.Status}})
}
