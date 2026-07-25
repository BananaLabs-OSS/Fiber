//go:build wasip1

package pulp

//go:wasmimport pulp fs_read
func hostFSRead(pathPtr, pathLen, dataPtrOut, dataLenOut uint32) uint32

//go:wasmimport pulp fs_write
func hostFSWrite(pathPtr, pathLen, dataPtr, dataLen, reqPtr, reqLen uint32) uint32

//go:wasmimport pulp fs_delete
func hostFSDelete(pathPtr, pathLen uint32) uint32

//go:wasmimport pulp fs_list
func hostFSList(pathPtr, pathLen, dataPtrOut, dataLenOut uint32) uint32

//go:wasmimport pulp fs_stat
func hostFSStat(reqPtr, reqLen, dataPtrOut, dataLenOut uint32) uint32

//go:wasmimport pulp fs_rename
func hostFSRename(reqPtr, reqLen uint32) uint32

//go:wasmimport pulp fs_remove_all
func hostFSRemoveAll(pathPtr, pathLen uint32) uint32

//go:wasmimport pulp fs_mkdir_all
func hostFSMkdirAll(reqPtr, reqLen uint32) uint32

//go:wasmimport pulp fs_chmod
func hostFSChmod(reqPtr, reqLen uint32) uint32

//go:wasmimport pulp fs_create_temp
func hostFSCreateTemp(reqPtr, reqLen, dataPtrOut, dataLenOut uint32) uint32

//go:wasmimport pulp fs_mkdir_temp
func hostFSMkdirTemp(reqPtr, reqLen, dataPtrOut, dataLenOut uint32) uint32
