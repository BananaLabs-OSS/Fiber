//go:build wasip1

package s3

//go:wasmimport pulp s3_put
func hostS3Put(reqPtr, reqLen uint32) uint32

//go:wasmimport pulp s3_put_sized
func hostS3PutSized(reqPtr, reqLen uint32) uint32

//go:wasmimport pulp s3_put_multipart_init
func hostS3PutMultipartInit(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32

//go:wasmimport pulp s3_put_multipart_part
func hostS3PutMultipartPart(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32

//go:wasmimport pulp s3_put_multipart_complete
func hostS3PutMultipartComplete(reqPtr, reqLen uint32) uint32

//go:wasmimport pulp s3_put_multipart_abort
func hostS3PutMultipartAbort(reqPtr, reqLen uint32) uint32

//go:wasmimport pulp s3_presign
func hostS3Presign(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32

//go:wasmimport pulp s3_presign_put
func hostS3PresignPut(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32

//go:wasmimport pulp s3_head
func hostS3Head(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32

//go:wasmimport pulp s3_get
func hostS3Get(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32

//go:wasmimport pulp s3_copy
func hostS3Copy(reqPtr, reqLen uint32) uint32

//go:wasmimport pulp s3_delete
func hostS3Delete(reqPtr, reqLen uint32) uint32

//go:wasmimport pulp s3_list
func hostS3List(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32
