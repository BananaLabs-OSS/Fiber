//go:build wasip1

package stripe

//go:wasmimport pulp stripe_checkout_session_create
func hostCheckoutSessionCreate(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32

//go:wasmimport pulp stripe_checkout_session_get
func hostCheckoutSessionGet(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32

//go:wasmimport pulp stripe_webhook_verify
func hostWebhookVerify(reqPtr, reqLen uint32) uint32

//go:wasmimport pulp stripe_payment_intent_create
func hostPaymentIntentCreate(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32

//go:wasmimport pulp stripe_payment_intent_get
func hostPaymentIntentGet(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32

//go:wasmimport pulp stripe_payment_intent_capture
func hostPaymentIntentCapture(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32

//go:wasmimport pulp stripe_payment_intent_cancel
func hostPaymentIntentCancel(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32

//go:wasmimport pulp stripe_setup_intent_create
func hostSetupIntentCreate(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32

//go:wasmimport pulp stripe_setup_intent_get
func hostSetupIntentGet(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32

//go:wasmimport pulp stripe_refund_create
func hostRefundCreate(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32

//go:wasmimport pulp stripe_customer_create
func hostCustomerCreate(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32

//go:wasmimport pulp stripe_invoice_create
func hostInvoiceCreate(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32

//go:wasmimport pulp stripe_invoice_finalize
func hostInvoiceFinalize(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32

//go:wasmimport pulp stripe_invoice_mark_paid_out_of_band
func hostInvoiceMarkPaidOutOfBand(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32

//go:wasmimport pulp stripe_invoice_item_create
func hostInvoiceItemCreate(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32

//go:wasmimport pulp stripe_balance_get
func hostBalanceGet(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32

//go:wasmimport pulp stripe_coupon_create
func hostCouponCreate(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32

//go:wasmimport pulp stripe_promotion_code_create
func hostPromotionCodeCreate(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32

//go:wasmimport pulp stripe_promotion_code_lookup
func hostPromotionCodeLookup(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32

//go:wasmimport pulp stripe_promotion_code_update
func hostPromotionCodeUpdate(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32
