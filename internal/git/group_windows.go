//go:build windows

// The Windows half of ADR-0044 clause 4.
//
// Windows has no process group a signal can reach, and TerminateProcess ends
// one process. A job object is the construct that owns a process tree:
// everything a job member starts joins the job, so terminating the job
// terminates the descendants too. JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE adds the
// same guarantee for the case where this process dies without cleaning up.
//
// The job functions are not in the standard library's syscall package, so
// they are resolved from kernel32 the way internal/server/resolve_windows.go
// already resolves GetFinalPathNameByHandleW: kernel32 is loaded in every
// Windows process, so this costs a lookup rather than a load, and it adds no
// dependency (ADR-0049 clause 2).

package git

import (
	"os/exec"
	"syscall"
	"unsafe"
)

const (
	// JobObjectExtendedLimitInformation.
	jobObjectExtendedLimitInformation = 9
	// JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE.
	jobObjectLimitKillOnJobClose = 0x00002000
	// The access the job needs on the child: enough to place it in the job
	// and to end it.
	processSetQuota  = 0x0100
	processTerminate = 0x0001
)

// jobObjectBasicLimitInformation is JOBOBJECT_BASIC_LIMIT_INFORMATION.
type jobObjectBasicLimitInformation struct {
	PerProcessUserTimeLimit int64
	PerJobUserTimeLimit     int64
	LimitFlags              uint32
	MinimumWorkingSetSize   uintptr
	MaximumWorkingSetSize   uintptr
	ActiveProcessLimit      uint32
	Affinity                uintptr
	PriorityClass           uint32
	SchedulingClass         uint32
}

// ioCounters is IO_COUNTERS.
type ioCounters struct {
	ReadOperationCount  uint64
	WriteOperationCount uint64
	OtherOperationCount uint64
	ReadTransferCount   uint64
	WriteTransferCount  uint64
	OtherTransferCount  uint64
}

// jobObjectExtendedLimitInformationStruct is
// JOBOBJECT_EXTENDED_LIMIT_INFORMATION.
type jobObjectExtendedLimitInformationStruct struct {
	BasicLimitInformation jobObjectBasicLimitInformation
	IoInfo                ioCounters
	ProcessMemoryLimit    uintptr
	JobMemoryLimit        uintptr
	PeakProcessMemoryUsed uintptr
	PeakJobMemoryUsed     uintptr
}

// processGroup holds the job object one invocation's tree lives in.
type processGroup struct {
	job *syscall.Handle
}

func newGroup() group { return processGroup{job: new(syscall.Handle)} }

// prepare puts the child in a console process group of its own, so a console
// control event aimed at this process does not reach it and the other way
// round.
func (processGroup) prepare(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.CreationFlags |= syscall.CREATE_NEW_PROCESS_GROUP
}

// adopt places the running child into a fresh job object. os/exec cannot
// start a process suspended, so this happens immediately after the child
// exists; a descendant started in the interval before the assignment is the
// one case this does not cover, and git starts none that early.
func (g processGroup) adopt(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	createJobObject := kernel32.NewProc("CreateJobObjectW")
	setInformationJobObject := kernel32.NewProc("SetInformationJobObject")
	assignProcessToJobObject := kernel32.NewProc("AssignProcessToJobObject")

	handle, _, _ := createJobObject.Call(0, 0)
	if handle == 0 {
		return
	}
	limits := jobObjectExtendedLimitInformationStruct{}
	limits.BasicLimitInformation.LimitFlags = jobObjectLimitKillOnJobClose
	if ok, _, _ := setInformationJobObject.Call(handle, jobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&limits)), unsafe.Sizeof(limits)); ok == 0 {
		_ = syscall.CloseHandle(syscall.Handle(handle))
		return
	}
	process, err := syscall.OpenProcess(processSetQuota|processTerminate, false, uint32(cmd.Process.Pid))
	if err != nil {
		_ = syscall.CloseHandle(syscall.Handle(handle))
		return
	}
	defer syscall.CloseHandle(process)
	if ok, _, _ := assignProcessToJobObject.Call(handle, uintptr(process)); ok == 0 {
		_ = syscall.CloseHandle(syscall.Handle(handle))
		return
	}
	*g.job = syscall.Handle(handle)
}

// terminate ends every process in the job, then closes it. Closing alone
// would end them too, through the kill-on-close limit, but ending them
// explicitly is what makes the moment observable.
func (g processGroup) terminate(cmd *exec.Cmd) error {
	if *g.job == 0 {
		if cmd.Process == nil {
			return nil
		}
		return cmd.Process.Kill()
	}
	terminateJobObject := syscall.NewLazyDLL("kernel32.dll").NewProc("TerminateJobObject")
	_, _, _ = terminateJobObject.Call(uintptr(*g.job), 1)
	err := syscall.CloseHandle(*g.job)
	*g.job = 0
	return err
}
