package invoker

import (
	"code.cloudfoundry.org/goshims/execshim"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/voldriver"
	"fmt"
)

//go:generate counterfeiter -o ./voldriverfakes/fake_invoker.go . Invoker

type Invoker interface {
	Invoke(env voldriver.Env, executable string, args []string) ([]byte, error)
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

func (r *realInvoker) Invoke(env voldriver.Env, executable string, cmdArgs []string) ([]byte, error) {
	logger := env.Logger().Session("invoking-command", lager.Data{"executable": executable, "args": cmdArgs})
	logger.Info("start")
	defer logger.Info("end")

	cmdHandle := r.useExec.CommandContext(env.Context(), executable, cmdArgs...)

	output, err := cmdHandle.CombinedOutput()
	if err != nil {
		logger.Error(fmt.Sprintf("%s invocation failed", executable), err, lager.Data{"err": output})
		return output, fmt.Errorf("%s - details:\n%s", err.Error(), output)
	}

	return output, nil
}
