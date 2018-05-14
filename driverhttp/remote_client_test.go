package driverhttp_test

import (
	"net/http"
	"time"

	"code.cloudfoundry.org/clock/fakeclock"

	"bytes"
	"fmt"

	"io/ioutil"

	"os"
	"os/exec"
	"path"

	"context"

	"code.cloudfoundry.org/goshims/http_wrap/http_fake"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"code.cloudfoundry.org/voldriver"
	"code.cloudfoundry.org/voldriver/driverhttp"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"
)

var _ = Describe("RemoteClient", func() {

	var (
		testLogger                lager.Logger
		ctx                       context.Context
		env                       voldriver.Env
		httpClient                *http_fake.FakeClient
		driver                    voldriver.Driver
		validHttpMountResponse    *http.Response
		validHttpPathResponse     *http.Response
		validHttpActivateResponse *http.Response
		fakeClock                 *fakeclock.FakeClock
	)

	BeforeEach(func() {
		httpClient = new(http_fake.FakeClient)
		fakeClock = fakeclock.NewFakeClock(time.Now())
		driver = driverhttp.NewRemoteClientWithClient("http://127.0.0.1:8080", httpClient, fakeClock)

		validHttpMountResponse = &http.Response{
			StatusCode: driverhttp.StatusOK,
			Body:       stringCloser{bytes.NewBufferString("{\"Mountpoint\":\"somePath\"}")},
		}

		validHttpPathResponse = &http.Response{
			StatusCode: driverhttp.StatusOK,
			Body:       stringCloser{bytes.NewBufferString("{\"Mountpoint\":\"somePath\"}")},
		}

		validHttpActivateResponse = &http.Response{
			StatusCode: driverhttp.StatusOK,
			Body:       stringCloser{bytes.NewBufferString("{\"Implements\":[\"VolumeDriver\"]}")},
		}
		testLogger = lagertest.NewTestLogger("LocalDriver Server Test")
		ctx = context.TODO()
		env = driverhttp.NewHttpDriverEnv(testLogger, ctx)
	})

	Context("when the driver returns as error and the transport is TCP", func() {

		BeforeEach(func() {
			fakeClock = fakeclock.NewFakeClock(time.Now())
			httpClient = new(http_fake.FakeClient)
			driver = driverhttp.NewRemoteClientWithClient("http://127.0.0.1:8080", httpClient, fakeClock)
			httpClient.DoStub = func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: driverhttp.StatusInternalServerError,
					Body:       stringCloser{bytes.NewBufferString("{\"Err\":\"some error string\"}")},
				}, nil
			}

			go fastForward(fakeClock, 40)
		})

		It("should not be able to mount", func() {

			volumeId := "fake-volume"
			mountResponse := driver.Mount(env, voldriver.MountRequest{Name: volumeId})

			By("signaling an error")
			Expect(mountResponse.Err).To(Equal("some error string"))
			Expect(mountResponse.Mountpoint).To(Equal(""))
		})

		It("should not be able to unmount", func() {

			volumeId := "fake-volume"
			unmountResponse := driver.Unmount(env, voldriver.UnmountRequest{Name: volumeId})

			By("signaling an error")
			Expect(unmountResponse.Err).To(Equal("some error string"))
		})

	})

	Context("when the driver returns successful and the transport is TCP", func() {
		var volumeId string

		Context("when match is called to check for driver reuse", func() {
			var (
				ret       bool
				matchable voldriver.MatchableDriver
			)

			BeforeEach(func() {
				matchable, ret = driver.(voldriver.MatchableDriver)
				Expect(ret).To(BeTrue())
			})

			Context("when stuff matches", func() {
				BeforeEach(func() {
					ret = matchable.Matches(testLogger, "", nil)
				})
				It("should match", func() {
					Expect(ret).To(BeTrue())
				})
			})

			Context("when stuff doesn't match", func() {
				BeforeEach(func() {
					ret = matchable.Matches(testLogger, "", &voldriver.TLSConfig{InsecureSkipVerify: true, CAFile: "foo", CertFile: "foo", KeyFile: "foo"})
				})
				It("should not match", func() {
					Expect(ret).To(BeFalse())
				})
			})
		})

		It("should be able to mount", func() {
			httpClient.DoReturns(validHttpMountResponse, nil)

			mountResponse := driver.Mount(env, voldriver.MountRequest{Name: volumeId})

			By("giving back a path with no error")
			Expect(mountResponse.Err).To(Equal(""))
			Expect(mountResponse.Mountpoint).To(Equal("somePath"))
		})

		It("should return mount point", func() {
			httpClient.DoReturns(validHttpPathResponse, nil)

			volumeId := "fake-volume"
			pathResponse := driver.Path(env, voldriver.PathRequest{Name: volumeId})

			Expect(pathResponse.Err).To(Equal(""))
			Expect(pathResponse.Mountpoint).To(Equal("somePath"))
		})

		It("should be able to unmount", func() {

			validHttpUnmountResponse := &http.Response{
				StatusCode: driverhttp.StatusOK,
				Body:       stringCloser{bytes.NewBufferString("{\"Err\":\"\"}")},
			}

			httpClient.DoReturns(validHttpUnmountResponse, nil)

			volumeId := "fake-volume"
			unmountResponse := driver.Unmount(env, voldriver.UnmountRequest{Name: volumeId})

			Expect(unmountResponse.Err).To(Equal(""))
		})

		It("should be able to activate", func() {
			httpClient.DoReturns(validHttpActivateResponse, nil)

			activateResponse := driver.Activate(env)

			By("giving back a path with no error")
			Expect(activateResponse.Err).To(Equal(""))
			Expect(activateResponse.Implements).To(Equal([]string{"VolumeDriver"}))
		})
	})

	Context("when the driver is malicious and the transport is TCP", func() {

		BeforeEach(func() {
			httpClient.DoStub = func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: driverhttp.StatusOK,
					Body:       stringCloser{bytes.NewBufferString("i am trying to pown your system")},
				}, nil
			}

			go fastForward(fakeClock, 40)
		})

		It("should not be able to mount", func() {
			volumeId := "fake-volume"
			mountResponse := driver.Mount(env, voldriver.MountRequest{Name: volumeId})

			By("signaling an error")
			Expect(mountResponse.Err).NotTo(Equal(""))
			Expect(mountResponse.Mountpoint).To(Equal(""))
		})

		It("should still not be able to mount", func() {

			invalidHttpResponse := &http.Response{
				StatusCode: driverhttp.StatusInternalServerError,
				Body:       stringCloser{bytes.NewBufferString("i am trying to pown your system")},
			}

			httpClient.DoReturns(invalidHttpResponse, nil)

			volumeId := "fake-volume"
			mountResponse := driver.Mount(env, voldriver.MountRequest{Name: volumeId})

			By("signaling an error")
			Expect(mountResponse.Err).NotTo(Equal(""))
			Expect(mountResponse.Mountpoint).To(Equal(""))
		})

		It("should not be able to unmount", func() {

			volumeId := "fake-volume"
			unmountResponse := driver.Unmount(env, voldriver.UnmountRequest{Name: volumeId})

			Expect(unmountResponse.Err).NotTo(Equal(""))
		})

	})

	Context("when the http transport fails and the transport is TCP", func() {

		BeforeEach(func() {
			// all of the tests in this context will perform retry logic over 30 seconds, so we need to
			// simulate time passing.
			go fastForward(fakeClock, 40)
		})

		It("should fail to mount", func() {

			httpClient.DoReturns(nil, fmt.Errorf("connection failed"))

			volumeId := "fake-volume"
			mountResponse := driver.Mount(env, voldriver.MountRequest{Name: volumeId})

			By("signaling an error")
			Expect(mountResponse.Err).To(Equal("connection failed"))
		})

		It("should fail to unmount", func() {

			httpClient.DoReturns(nil, fmt.Errorf("connection failed"))

			volumeId := "fake-volume"
			unmountResponse := driver.Unmount(env, voldriver.UnmountRequest{Name: volumeId})

			By("signaling an error")
			Expect(unmountResponse.Err).NotTo(Equal(""))
		})

		It("should still fail to unmount", func() {

			invalidHttpResponse := &http.Response{
				StatusCode: driverhttp.StatusInternalServerError,
				Body:       errCloser{bytes.NewBufferString("")},
			}

			httpClient.DoReturns(invalidHttpResponse, nil)

			volumeId := "fake-volume"
			unmountResponse := driver.Unmount(env, voldriver.UnmountRequest{Name: volumeId})

			Expect(unmountResponse.Err).NotTo(Equal(""))
		})

		It("should fail to activate", func() {
			httpClient.DoReturns(nil, fmt.Errorf("connection failed"))

			activateResponse := driver.Activate(env)

			By("signaling an error")
			Expect(activateResponse.Err).NotTo(Equal(""))
		})

	})

	Context("when the transport is unix", func() {
		var (
			volumeId                     string
			unixRunner                   *ginkgomon.Runner
			localDriverUnixServerProcess ifrit.Process
			socketPath                   string
		)

		BeforeEach(func() {
			tmpdir, err := ioutil.TempDir(os.TempDir(), "fake-driver-test")
			Expect(err).ShouldNot(HaveOccurred())

			socketPath = path.Join(tmpdir, "localdriver.sock")

			unixRunner = ginkgomon.New(ginkgomon.Config{
				Name: "local-driver",
				Command: exec.Command(
					localDriverPath,
					"-listenAddr", socketPath,
					"-transport", "unix",
				),
				StartCheck: "local-driver-server.started",
			})

			httpClient = new(http_fake.FakeClient)
			volumeId = "fake-volume"
			localDriverUnixServerProcess = ginkgomon.Invoke(unixRunner)

			time.Sleep(time.Millisecond * 1000)

			fakeClock = fakeclock.NewFakeClock(time.Now())
			driver = driverhttp.NewRemoteClientWithClient(socketPath, httpClient, fakeClock)
			validHttpMountResponse = &http.Response{
				StatusCode: driverhttp.StatusOK,
				Body:       stringCloser{bytes.NewBufferString("{\"Mountpoint\":\"somePath\"}")},
			}
		})

		AfterEach(func() {
			ginkgomon.Kill(localDriverUnixServerProcess)
		})

		It("should be able to mount", func() {
			httpClient.DoReturns(validHttpMountResponse, nil)
			mountResponse := driver.Mount(env, voldriver.MountRequest{Name: volumeId})

			By("returning a mountpoint without errors")
			Expect(mountResponse.Err).To(Equal(""))
			Expect(mountResponse.Mountpoint).To(Equal("somePath"))
		})

		It("should be able to unmount", func() {

			validHttpUnmountResponse := &http.Response{
				StatusCode: driverhttp.StatusOK,
				Body:       stringCloser{bytes.NewBufferString("{\"Err\":\"\"}")},
			}

			httpClient.DoReturns(validHttpUnmountResponse, nil)

			volumeId := "fake-volume"
			unmountResponse := driver.Unmount(env, voldriver.UnmountRequest{Name: volumeId})

			Expect(unmountResponse.Err).To(Equal(""))
		})

		It("should be able to activate", func() {
			httpClient.DoReturns(validHttpActivateResponse, nil)

			activateResponse := driver.Activate(env)

			By("giving back a activation response with no error")
			Expect(activateResponse.Err).To(Equal(""))
			Expect(activateResponse.Implements).To(Equal([]string{"VolumeDriver"}))
		})

	})

})

func fastForward(fakeClock *fakeclock.FakeClock, seconds int) {
	for i := 0; i < seconds; i++ {
		time.Sleep(time.Millisecond * 3)
		fakeClock.IncrementBySeconds(1)
	}
}
