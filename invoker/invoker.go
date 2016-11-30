package invoker

import (
	"code.cloudfoundry.org/goshims/execshim"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/voldriver"
)

//go:generate counterfeiter -o ./cephfakes/fake_invoker.go . Invoker

type Invoker interface {
	Invoke(env voldriver.Env, executable string, args []string) error
}

type realInvoker struct {
	useExec execshim.Exec
}

func NewRealInvoker() Invoker {
	return NewRealInvokerWithExec(&execshim.ExecShim{})
}

func NewRealInvokerWithExec(useExec execshim.Exec) Invoker {
	return &realInvoker{useExec}
}

func (r *realInvoker) Invoke(env voldriver.Env, executable string, cmdArgs []string) error {
	logger := env.Logger().Session("invoking-command", lager.Data{"executable": executable, "args": cmdArgs})
	logger.Info("start")
	defer logger.Info("end")

	cmdHandle := r.useExec.CommandContext(env.Context(), executable, cmdArgs...)

	_, err := cmdHandle.StdoutPipe()
	if err != nil {
		logger.Error("unable to get stdout", err)
		return err
	}

	_, err = cmdHandle.StderrPipe()
	if err != nil {
		logger.Error("unable to get stderr", err)
		return err
	}

	if err = cmdHandle.Start(); err != nil {
		logger.Error("starting command", err)
		return err
	}

	if err = cmdHandle.Wait(); err != nil {
		logger.Error("command-exited", err)
		return err
	}

	// could validate stdout, but defer until actually need it
	return nil
}
