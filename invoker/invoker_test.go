package invoker_test

import (
	"bytes"
	"context"
	"fmt"

	"code.cloudfoundry.org/goshims/execshim/exec_fake"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"code.cloudfoundry.org/voldriver"
	"code.cloudfoundry.org/voldriver/driverhttp"

	"code.cloudfoundry.org/voldriver/invoker"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("RealInvoker", func() {
	var (
		subject    invoker.Invoker
		fakeCmd    *exec_fake.FakeCmd
		fakeExec   *exec_fake.FakeExec
		testLogger lager.Logger
		testCtx    context.Context
		testEnv    voldriver.Env
		cmd        = "some-fake-command"
		args       = []string{"fake-args-1"}
	)
	Context("when invoking an executable", func() {
		BeforeEach(func() {
			testLogger = lagertest.NewTestLogger("InvokerTest")
			testCtx = context.TODO()
			testEnv = driverhttp.NewHttpDriverEnv(testLogger, testCtx)
			fakeExec = new(exec_fake.FakeExec)
			fakeCmd = new(exec_fake.FakeCmd)
			fakeExec.CommandContextReturns(fakeCmd)
			subject = invoker.NewRealInvokerWithExec(fakeExec)
		})

		It("should report an error when unable to attach to stdout", func() {
			fakeCmd.StdoutPipeReturns(errCloser{bytes.NewBufferString("")}, fmt.Errorf("unable to attach to stdout"))
			err := subject.Invoke(testEnv, cmd, args)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("unable to attach to stdout"))
		})

		It("should report an error when unable to attach to stderr", func() {
			fakeCmd.StderrPipeReturns(errCloser{bytes.NewBufferString("")}, fmt.Errorf("unable to attach to stderr"))
			err := subject.Invoke(testEnv, cmd, args)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("unable to attach to stderr"))
		})

		It("should report an error when unable to start binary", func() {
			fakeCmd.StdoutPipeReturns(errCloser{bytes.NewBufferString("cmdfails")}, nil)
			fakeCmd.StartReturns(fmt.Errorf("unable to start binary"))
			err := subject.Invoke(testEnv, cmd, args)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("unable to start binary"))
		})
		It("should report an error when executing the driver binary fails", func() {
			fakeCmd.WaitReturns(fmt.Errorf("executing driver binary fails"))

			err := subject.Invoke(testEnv, cmd, args)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("executing driver binary fails"))
		})
		It("should successfully invoke cli", func() {
			err := subject.Invoke(testEnv, cmd, args)
			Expect(err).ToNot(HaveOccurred())
		})
	})
})
