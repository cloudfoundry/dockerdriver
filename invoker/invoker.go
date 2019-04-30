package invoker

import (
	"bytes"
	"code.cloudfoundry.org/dockerdriver"
	"code.cloudfoundry.org/goshims/execshim"
	"code.cloudfoundry.org/goshims/syscallshim"
	"code.cloudfoundry.org/lager"
	"context"
	"fmt"
	"syscall"
)

//go:generate counterfeiter -o ../dockerdriverfakes/fake_invoker.go . Invoker

type Invoker interface {
	Invoke(env dockerdriver.Env, executable string, args []string) ([]byte, error)
}

type realInvoker struct {
	useExec execshim.Exec
}

type pgroupInvoker struct {
	useExec execshim.Exec
	syscallShim syscallshim.Syscall
}

func NewRealInvoker() Invoker {
	return NewRealInvokerWithExec(&execshim.ExecShim{})
}

func NewRealInvokerWithExec(useExec execshim.Exec) Invoker {
	return &realInvoker{useExec}
}

func NewProcessGroupInvoker() Invoker {
	return NewProcessGroupInvokerWithExec(&execshim.ExecShim{}, &syscallshim.SyscallShim{})
}

func NewProcessGroupInvokerWithExec(useExec execshim.Exec, syscallShim syscallshim.Syscall) Invoker {
	return &pgroupInvoker{useExec, syscallShim}
}

func (r *realInvoker) Invoke(env dockerdriver.Env, executable string, cmdArgs []string) ([]byte, error) {
	logger := env.Logger().Session("invoking-command", lager.Data{"executable": executable, "args": cmdArgs})
	logger.Info("start")
	defer logger.Info("end")

	cmdHandle := r.useExec.CommandContext(env.Context(), executable, cmdArgs...)

	output, err := cmdHandle.CombinedOutput()
	if err != nil {
		logger.Error("invocation-failed", err, lager.Data{"output": output, "exe": executable})
		return output, fmt.Errorf("%s - details:\n%s", err.Error(), output)
	}

	return output, nil
}

func (r *pgroupInvoker) Invoke(env dockerdriver.Env, executable string, cmdArgs []string) ([]byte, error) {
	logger := env.Logger().Session("invoking-command-pgroup", lager.Data{"executable": executable, "args": cmdArgs})
	logger.Info("start")
	defer logger.Info("end")

	cmdHandle := r.useExec.CommandContext(context.Background(), executable, cmdArgs...)
	cmdHandle.SysProcAttr().Setpgid = true

	var outb bytes.Buffer
	cmdHandle.SetStdout(&outb)
	cmdHandle.SetStderr(&outb)
	err := cmdHandle.Start()
	if err != nil {
		logger.Error("command-start-failed", err, lager.Data{"exe": executable, "output": outb.Bytes()})
		return nil, err
	}

	complete := make(chan bool)

	go func() {
		select {
		case <-complete:
			// noop
		case <-env.Context().Done():
			logger.Info("command-sigkill", lager.Data{"exe": executable, "pid": -cmdHandle.Pid()})
			r.syscallShim.Kill(-cmdHandle.Pid(), syscall.SIGKILL)
		}
	}()

	err = cmdHandle.Wait()
	if err != nil {
		logger.Error("command-failed", err, lager.Data{"exe": executable, "output": outb.Bytes()})
		return outb.Bytes(), err
	}

	close(complete)

	return outb.Bytes(), nil
}
