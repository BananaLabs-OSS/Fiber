//go:build wasip1

package docker

//go:wasmimport pulp docker_list
func hostList(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32

//go:wasmimport pulp docker_get
func hostGet(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32

//go:wasmimport pulp docker_create
func hostCreate(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32

//go:wasmimport pulp docker_destroy
func hostDestroy(reqPtr, reqLen uint32) uint32

//go:wasmimport pulp docker_restart
func hostRestart(reqPtr, reqLen uint32) uint32

//go:wasmimport pulp docker_exec
func hostExec(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32

//go:wasmimport pulp docker_logs
func hostLogs(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32

//go:wasmimport pulp docker_stats
func hostStats(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32

//go:wasmimport pulp docker_files_read
func hostFilesRead(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32

//go:wasmimport pulp docker_files_write
func hostFilesWrite(reqPtr, reqLen uint32) uint32

//go:wasmimport pulp docker_files_delete
func hostFilesDelete(reqPtr, reqLen uint32) uint32

//go:wasmimport pulp docker_events_poll
func hostEventsPoll(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32

//go:wasmimport pulp docker_stats_all
func hostStatsAll(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32

//go:wasmimport pulp docker_build
func hostBuild(reqPtr, reqLen uint32) uint32

//go:wasmimport pulp docker_build_status
func hostBuildStatus(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32
