package invoker_test

import (
	"context"
	"errors"
	"fmt"
	"syscall"
	"time"

	"code.cloudfoundry.org/dockerdriver"
	"code.cloudfoundry.org/dockerdriver/driverhttp"
	"code.cloudfoundry.org/goshims/execshim/exec_fake"
	"code.cloudfoundry.org/goshims/syscallshim/syscall_fake"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"

	"code.cloudfoundry.org/dockerdriver/invoker"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ProcessGroupInvoker", func() {
	var (
		subject     invoker.Invoker
		fakeCmd     *exec_fake.FakeCmd
		fakeExec    *exec_fake.FakeExec
		fakeSyscall *syscall_fake.FakeSyscall
		testLogger  lager.Logger
		testCtx     context.Context
		cancel      context.CancelFunc
		testEnv     dockerdriver.Env
		cmd         = "some-fake-command"
		args        = []string{"fake-args-1", "fake-args-2"}
		attrs       *syscall.SysProcAttr
	)

	Context("when invoking an executable", func() {

		BeforeEach(func() {
			testLogger = lagertest.NewTestLogger("InvokerTest")
			testCtx, cancel = context.WithCancel(context.TODO())
			testEnv = driverhttp.NewHttpDriverEnv(testLogger, testCtx)

			fakeExec = new(exec_fake.FakeExec)
			fakeCmd = new(exec_fake.FakeCmd)
			fakeExec.CommandContextReturns(fakeCmd)
			attrs = &syscall.SysProcAttr{}
			fakeCmd.SysProcAttrReturns(attrs)
			fakeSyscall = new(syscall_fake.FakeSyscall)

			subject = invoker.NewProcessGroupInvokerWithExec(fakeExec, fakeSyscall)
		})

		It("should set the stdout and stderr", func() {
			_, err := subject.Invoke(testEnv, cmd, args)
			Expect(err).ToNot(HaveOccurred())

			Expect(fakeCmd.SetStdoutCallCount()).To(Equal(1))
			Expect(fakeCmd.SetStderrCallCount()).To(Equal(1))
		})

		It("should run the command in its own process group", func() {
			_, err := subject.Invoke(testEnv, cmd, args)
			Expect(err).ToNot(HaveOccurred())
			Expect(attrs.Setpgid).To(BeTrue())
		})

		It("should successfully invoke cli", func() {
			_, err := subject.Invoke(testEnv, cmd, args)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should not signal the process", func() {
			_, err := subject.Invoke(testEnv, cmd, args)
			Expect(err).ToNot(HaveOccurred())

			Expect(fakeSyscall.KillCallCount()).To(BeZero())
		})

		Context("when the command start fails", func() {

			BeforeEach(func() {
				fakeCmd.StartReturns(errors.New("start badness"))
			})

			It("should report an error", func() {
				_, err := subject.Invoke(testEnv, cmd, args)
				Expect(err).To(HaveOccurred())

				Expect(err.Error()).To(ContainSubstring("start badness"))
			})

			It("should not signal the process", func() {
				_, err := subject.Invoke(testEnv, cmd, args)
				Expect(err).To(HaveOccurred())

				Expect(fakeSyscall.KillCallCount()).To(BeZero())
			})
		})

		Context("when command fails", func() {

			BeforeEach(func() {
				fakeCmd.WaitReturns(fmt.Errorf("executing binary fails"))
			})

			It("should report an error", func() {
				_, err := subject.Invoke(testEnv, cmd, args)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("executing binary fails"))
			})

			//It("should return command output", func() {
			//	output, _ := subject.Invoke(testEnv, cmd, args)
			//	Expect(string(output)).To(Equal("an error occured"))
			//})

			It("should not signal the process", func() {
				_, err := subject.Invoke(testEnv, cmd, args)
				Expect(err).To(HaveOccurred())

				Expect(fakeSyscall.KillCallCount()).To(BeZero())
			})
		})

		Context("when the context is cancelled", func() {
			BeforeEach(func() {
				fakeCmd.PidReturns(9999)

				fakeCmd.WaitStub = func() error {
					cancel()
					time.Sleep(time.Second)
					return context.Canceled
				}
			})

			It("should SIGKILL the process group", func() {
				_, err := subject.Invoke(testEnv, cmd, args)
				Expect(err).To(HaveOccurred())

				Expect(fakeSyscall.KillCallCount()).To(Equal(1))
				pid, signal := fakeSyscall.KillArgsForCall(0)
				Expect(pid).To(Equal(-9999)) // process group
				Expect(signal).To(Equal(syscall.SIGKILL))
			})
		})
	})
})
