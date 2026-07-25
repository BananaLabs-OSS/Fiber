//go:build !wasip1

package s3

// Native builds do not have a Pulp host. Every guest import fails closed.
func hostS3Put(reqPtr, reqLen uint32) uint32 { return 99 }

func hostS3PutSized(reqPtr, reqLen uint32) uint32 { return 99 }

func hostS3PutMultipartInit(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32 { return 99 }

func hostS3PutMultipartPart(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32 { return 99 }

func hostS3PutMultipartComplete(reqPtr, reqLen uint32) uint32 { return 99 }

func hostS3PutMultipartAbort(reqPtr, reqLen uint32) uint32 { return 99 }

func hostS3Presign(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32 { return 99 }

func hostS3PresignPut(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32 { return 99 }

func hostS3Head(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32 { return 99 }

func hostS3Get(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32 { return 99 }

func hostS3Copy(reqPtr, reqLen uint32) uint32 { return 99 }

func hostS3Delete(reqPtr, reqLen uint32) uint32 { return 99 }

func hostS3List(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32 { return 99 }
