package router

import (
	"github.com/abubakvr/payup-backend/services/payment/internal/controller"
	"github.com/gin-gonic/gin"
)

// Setup returns the HTTP router for the payment service. Add your payment routes here.
func Setup(ctrl *controller.PaymentController) *gin.Engine {
	r := gin.Default()

	r.GET("/health", ctrl.Health)
	r.GET("/ready", ctrl.Health)

	// 9PSB inbound webhook (no auth). Event=wallet-upgrade: finalize upgrade request and update wallet tier on APPROVED.
	r.POST("/webhooks/9psb", ctrl.Handle9PSBWebhook)

	r.POST("/wallets", ctrl.OpenWallet)
	// User-authenticated (JWT). Returns wallet details from DB (account number, account name, status).
	r.GET("/wallet", ctrl.GetWallet)
	// User-authenticated (JWT). Returns live balance from 9PSB wallet_enquiry.
	r.GET("/wallet/balance", ctrl.GetBalance)
	// User-authenticated (JWT). 9PSB WaaS transaction history. Query: from_date, to_date (YYYY-MM-DD, max 31 days), limit (default 20).
	r.GET("/wallet/waas/transactions", ctrl.GetWaasTransactions)
	// User-authenticated (JWT). 9PSB WaaS wallet status.
	r.GET("/wallet/waas/status", ctrl.GetWaasWalletStatus)
	// User-authenticated (JWT). Returns wallet upgrade status (latest upgrade request if any).
	r.GET("/wallet/upgrade-status", ctrl.GetWalletUpgradeStatus)
	// User-authenticated (JWT). Returns wallet transaction history (newest first). Query: limit (default 20, max 100), offset.
	r.GET("/wallet/transactions", ctrl.GetWalletTransactions)
	// User-authenticated (JWT). Returns a single transaction by transaction_ref (path param). 404 if not found or not owned.
	r.GET("/wallet/transactions/:transaction_ref", ctrl.GetTransactionDetail)
	// Resolve beneficiary name: 9PSB (120001) = wallet_enquiry, other banks = other_banks_enquiry. Body: bank_code, account_number.
	r.POST("/wallet/beneficiary-enquiry", ctrl.BeneficiaryEnquiry)
	// User-authenticated (JWT); optional X-Idempotency-Key. Gateway should use auth_request for /v1/payment/* or /v1/transfers.
	r.POST("/transfers", ctrl.TransferToOtherBank)

	return r
}
