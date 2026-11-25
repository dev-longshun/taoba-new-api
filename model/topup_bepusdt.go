package model

import (
	"errors"
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

const PaymentProviderBEpusdt = "bepusdt"

var ErrPaymentAmountMismatch = errors.New("payment amount mismatch")

// RechargeBEpusdt completes a BEpusdt order in one transaction: it locks the
// order row, checks its state and the paid fiat amount, marks it successful and
// credits the wallet. Duplicate or concurrent callbacks credit at most once;
// alreadyDone=true means the order had already been completed.
func RechargeBEpusdt(tradeNo string, paidMoney decimal.Decimal, callerIp string) (alreadyDone bool, err error) {
	if tradeNo == "" {
		return false, errors.New("payment order number not provided")
	}

	refCol := "`trade_no`"
	if common.UsingMainDatabase(common.DatabaseTypePostgreSQL) {
		refCol = `"trade_no"`
	}

	var quotaToAdd int
	topUp := &TopUp{}
	err = DB.Transaction(func(tx *gorm.DB) error {
		if err := lockForUpdate(tx).Where(refCol+" = ?", tradeNo).First(topUp).Error; err != nil {
			return ErrTopUpNotFound
		}
		if topUp.PaymentProvider != PaymentProviderBEpusdt {
			return ErrPaymentMethodMismatch
		}
		if topUp.Status == common.TopUpStatusSuccess {
			alreadyDone = true
			return nil
		}
		// An order can expire on our side while the payment is still being
		// confirmed on chain; a confirmed payment still completes it.
		if topUp.Status != common.TopUpStatusPending && topUp.Status != common.TopUpStatusExpired {
			return ErrTopUpStatusInvalid
		}
		// BEpusdt echoes the fiat amount of the order it created.
		if paidMoney.Sub(decimal.NewFromFloat(topUp.Money)).Abs().GreaterThan(decimal.NewFromFloat(0.01)) {
			return ErrPaymentAmountMismatch
		}
		var quotaErr error
		quotaToAdd, quotaErr = common.WalletQuotaFromDecimalStrict(
			decimal.NewFromInt(topUp.Amount).Mul(decimal.NewFromFloat(common.QuotaPerUnit)),
		)
		if quotaErr != nil || quotaToAdd <= 0 {
			return ErrInvalidTopUpQuota
		}
		topUp.CompleteTime = common.GetTimestamp()
		topUp.Status = common.TopUpStatusSuccess
		if err := tx.Save(topUp).Error; err != nil {
			return err
		}
		return creditTopUpQuota(tx, topUp.UserId, quotaToAdd, nil)
	})
	if err != nil {
		if !errors.Is(err, ErrTopUpNotFound) && !errors.Is(err, ErrPaymentMethodMismatch) &&
			!errors.Is(err, ErrTopUpStatusInvalid) && !errors.Is(err, ErrPaymentAmountMismatch) {
			common.SysError(common.LogText("bepusdt topup failed: %s", err.Error()))
		}
		return false, err
	}
	if alreadyDone {
		return true, nil
	}
	syncCreditUserQuotaCache(topUp.UserId, quotaToAdd, common.LogText("bepusdt topup"))

	common.SysLog(common.LogText("BEpusdt top-up succeeded trade_no=%s user_id=%d quota_to_add=%d money=%.2f", topUp.TradeNo, topUp.UserId, quotaToAdd, topUp.Money))
	RecordTopupLog(topUp.UserId, common.NewMessage("Online top-up succeeded, amount added: {{quota}}, payment amount: {{amount}}", map[string]any{"quota": logger.FormatQuota(quotaToAdd), "amount": fmt.Sprintf("%f", topUp.Money)}), callerIp, topUp.PaymentMethod, PaymentProviderBEpusdt)
	return false, nil
}
