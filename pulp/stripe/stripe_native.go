//go:build !wasip1

package stripe

// Native builds do not have a Pulp host. Every guest import fails closed.
func hostCheckoutSessionCreate(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32 { return 99 }

func hostCheckoutSessionGet(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32 { return 99 }

func hostWebhookVerify(reqPtr, reqLen uint32) uint32 { return 99 }

func hostPaymentIntentCreate(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32 { return 99 }

func hostPaymentIntentGet(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32 { return 99 }

func hostPaymentIntentCapture(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32 { return 99 }

func hostPaymentIntentCancel(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32 { return 99 }

func hostSetupIntentCreate(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32 { return 99 }

func hostSetupIntentGet(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32 { return 99 }

func hostRefundCreate(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32 { return 99 }

func hostCustomerCreate(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32 { return 99 }

func hostInvoiceCreate(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32 { return 99 }

func hostInvoiceFinalize(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32 { return 99 }

func hostInvoiceMarkPaidOutOfBand(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32 { return 99 }

func hostInvoiceItemCreate(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32 { return 99 }

func hostBalanceGet(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32 { return 99 }

func hostCouponCreate(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32 { return 99 }

func hostPromotionCodeCreate(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32 { return 99 }

func hostPromotionCodeLookup(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32 { return 99 }

func hostPromotionCodeUpdate(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32 { return 99 }
