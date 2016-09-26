package driverhttp_test

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"

	"fmt"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"code.cloudfoundry.org/voldriver"
	"code.cloudfoundry.org/voldriver/driverhttp"
	"code.cloudfoundry.org/voldriver/voldriverfakes"
	"context"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"sync"
	"time"
)

type RecordingCloseNotifier struct {
	*httptest.ResponseRecorder
	cn chan bool
}

func (rcn *RecordingCloseNotifier) CloseNotify() <-chan bool {
	return rcn.cn
}

func (rcn *RecordingCloseNotifier) SimulateClientCancel() {
	rcn.cn <- true
}

var _ = Describe("Volman Driver Handlers", func() {

	Context("when generating http handlers", func() {
		var testLogger = lagertest.NewTestLogger("HandlersTest")

		It("should produce a handler with an activate route", func() {
			By("faking out the driver")
			driver := &voldriverfakes.FakeDriver{}
			driver.ActivateReturns(voldriver.ActivateResponse{Implements: []string{"VolumeDriver"}})
			handler, err := driverhttp.NewHandler(testLogger, driver)
			Expect(err).NotTo(HaveOccurred())

			By("then fake serving the response using the handler")
			route, found := voldriver.Routes.FindRouteByName(voldriver.ActivateRoute)
			Expect(found).To(BeTrue())

			path := fmt.Sprintf("http://0.0.0.0%s", route.Path)
			httpRequest, err := http.NewRequest("POST", path, bytes.NewReader([]byte{}))
			Expect(err).NotTo(HaveOccurred())

			httpResponseRecorder := httptest.NewRecorder()
			handler.ServeHTTP(httpResponseRecorder, httpRequest)

			By("then deserialing the HTTP response")
			activateResponse := voldriver.ActivateResponse{}
			body, err := ioutil.ReadAll(httpResponseRecorder.Body)
			err = json.Unmarshal(body, &activateResponse)

			By("then expecting correct JSON conversion")
			Expect(err).ToNot(HaveOccurred())
			Expect(activateResponse.Implements).Should(Equal([]string{"VolumeDriver"}))
		})

		It("should produce a handler with a list route", func() {
			By("faking out the driver")
			driver := &voldriverfakes.FakeDriver{}
			volume := voldriver.VolumeInfo{
				Name:       "fake-volume",
				Mountpoint: "fake-mountpoint",
			}
			listResponse := voldriver.ListResponse{
				Volumes: []voldriver.VolumeInfo{volume},
				Err:     "",
			}

			driver.ListReturns(listResponse)
			handler, err := driverhttp.NewHandler(testLogger, driver)
			Expect(err).NotTo(HaveOccurred())

			By("then fake serving the response using the handler")
			route, found := voldriver.Routes.FindRouteByName(voldriver.ListRoute)
			Expect(found).To(BeTrue())

			path := fmt.Sprintf("http://0.0.0.0%s", route.Path)
			httpRequest, err := http.NewRequest("POST", path, bytes.NewReader([]byte{}))
			Expect(err).NotTo(HaveOccurred())

			httpResponseRecorder := httptest.NewRecorder()
			handler.ServeHTTP(httpResponseRecorder, httpRequest)

			By("then deserialing the HTTP response")
			listResponse = voldriver.ListResponse{}
			body, err := ioutil.ReadAll(httpResponseRecorder.Body)
			err = json.Unmarshal(body, &listResponse)

			By("then expecting correct JSON conversion")
			Expect(err).ToNot(HaveOccurred())
			Expect(listResponse.Volumes[0].Name).Should(Equal("fake-volume"))
		})

		Context("Mount", func() {

			var (
				err    error
				req    *http.Request
				res    *RecordingCloseNotifier
				driver *voldriverfakes.FakeDriver
				wg     sync.WaitGroup

				subject http.Handler
			)

			var ExpectMountPointToEqual = func(value string) voldriver.MountResponse {
				mountResponse := voldriver.MountResponse{}
				body, err := ioutil.ReadAll(res.Body)

				err = json.Unmarshal(body, &mountResponse)
				Expect(err).ToNot(HaveOccurred())

				Expect(mountResponse.Mountpoint).Should(Equal(value))
				return mountResponse
			}

			BeforeEach(func() {
				driver = &voldriverfakes.FakeDriver{}

				subject, err = driverhttp.NewHandler(testLogger, driver)
				Expect(err).NotTo(HaveOccurred())

				volumeId := "something"
				MountRequest := voldriver.MountRequest{
					Name: "some-volume",
					Opts: map[string]interface{}{"volume_id": volumeId},
				}
				mountJSONRequest, err := json.Marshal(MountRequest)
				Expect(err).NotTo(HaveOccurred())

				res = &RecordingCloseNotifier{
					ResponseRecorder: httptest.NewRecorder(),
					cn:               make(chan bool, 1),
				}

				route, found := voldriver.Routes.FindRouteByName(voldriver.MountRoute)
				Expect(found).To(BeTrue())

				path := fmt.Sprintf("http://0.0.0.0%s", route.Path)
				req, err = http.NewRequest("POST", path, bytes.NewReader(mountJSONRequest))
				Expect(err).NotTo(HaveOccurred())
			})

			Context("when mount is successful", func() {

				JustBeforeEach(func() {
					driver.MountReturns(voldriver.MountResponse{Mountpoint: "dummy_path"})

					wg.Add(1)
					testLogger.Info(fmt.Sprintf("%#v", res.Body))

					go func() {
						subject.ServeHTTP(res, req)
						wg.Done()
					}()

				})

				It("should produce a handler with a mount route", func() {
					wg.Wait()
					ExpectMountPointToEqual("dummy_path")
				})
			})

			Context("when the mount hangs and the client closes the connection", func() {
				JustBeforeEach(func() {
					driver.MountStub = func(logger lager.Logger, ctx context.Context, mountRequest voldriver.MountRequest) voldriver.MountResponse {
						for true {
							time.Sleep(time.Second * 1)

							select {
							case <-ctx.Done():
								logger.Error("from ctx", ctx.Err())
								return voldriver.MountResponse{Err: ctx.Err().Error()}
							}
						}
						return voldriver.MountResponse{}
					}
					wg.Add(2)

					go func() {
						subject.ServeHTTP(res, req)
						wg.Done()
					}()

					go func() {
						res.SimulateClientCancel()
						wg.Done()
					}()
				})

				It("should respond with context canceled", func() {
					wg.Wait()
					mountResponse := ExpectMountPointToEqual("")
					Expect(mountResponse.Err).Should(ContainSubstring("context canceled"))
				})
			})
		})

		It("should produce a handler with an unmount route", func() {
			By("faking out the driver")
			driver := &voldriverfakes.FakeDriver{}
			driver.UnmountReturns(voldriver.ErrorResponse{})

			handler, err := driverhttp.NewHandler(testLogger, driver)
			Expect(err).NotTo(HaveOccurred())

			httpResponseRecorder := httptest.NewRecorder()
			unmountRequest := voldriver.UnmountRequest{}
			unmountJSONRequest, err := json.Marshal(unmountRequest)
			Expect(err).NotTo(HaveOccurred())

			By("then fake serving the response using the handler")
			route, found := voldriver.Routes.FindRouteByName(voldriver.UnmountRoute)
			Expect(found).To(BeTrue())

			path := fmt.Sprintf("http://0.0.0.0%s", route.Path)
			httpRequest, err := http.NewRequest("POST", path, bytes.NewReader(unmountJSONRequest))
			Expect(err).NotTo(HaveOccurred())
			handler.ServeHTTP(httpResponseRecorder, httpRequest)

			By("then expecting correct HTTP status code")
			Expect(httpResponseRecorder.Code).To(Equal(200))
		})

		It("should produce a handler with a get route", func() {
			By("faking out the driver")
			driver := &voldriverfakes.FakeDriver{}
			driver.GetReturns(voldriver.GetResponse{Volume: voldriver.VolumeInfo{Name: "some-volume", Mountpoint: "dummy_path"}})
			handler, err := driverhttp.NewHandler(testLogger, driver)
			Expect(err).NotTo(HaveOccurred())

			httpResponseRecorder := httptest.NewRecorder()
			getRequest := voldriver.GetRequest{}
			getJSONRequest, err := json.Marshal(getRequest)
			Expect(err).NotTo(HaveOccurred())

			By("then fake serving the response using the handler")
			route, found := voldriver.Routes.FindRouteByName(voldriver.GetRoute)
			Expect(found).To(BeTrue())

			path := fmt.Sprintf("http://0.0.0.0%s", route.Path)
			httpRequest, err := http.NewRequest("POST", path, bytes.NewReader(getJSONRequest))
			Expect(err).NotTo(HaveOccurred())
			handler.ServeHTTP(httpResponseRecorder, httpRequest)

			By("then expecting correct HTTP status code")
			Expect(httpResponseRecorder.Code).To(Equal(200))
		})

		It("should produce a handler with a path route", func() {
			By("faking out the driver")
			driver := &voldriverfakes.FakeDriver{}
			driver.PathReturns(voldriver.PathResponse{})
			handler, err := driverhttp.NewHandler(testLogger, driver)
			Expect(err).NotTo(HaveOccurred())

			httpResponseRecorder := httptest.NewRecorder()
			pathRequest := voldriver.PathRequest{Name: "some-volume"}
			pathJSONRequest, err := json.Marshal(pathRequest)
			Expect(err).NotTo(HaveOccurred())

			By("then fake serving the response using the handler")
			route, found := voldriver.Routes.FindRouteByName(voldriver.PathRoute)
			Expect(found).To(BeTrue())

			path := fmt.Sprintf("http://0.0.0.0%s", route.Path)
			httpRequest, err := http.NewRequest("POST", path, bytes.NewReader(pathJSONRequest))
			Expect(err).NotTo(HaveOccurred())
			handler.ServeHTTP(httpResponseRecorder, httpRequest)

			By("then expecting correct HTTP status code")
			Expect(httpResponseRecorder.Code).To(Equal(200))
		})

		It("should produce a handler with a create route", func() {
			By("faking out the driver")
			driver := &voldriverfakes.FakeDriver{}
			driver.CreateReturns(voldriver.ErrorResponse{})
			handler, err := driverhttp.NewHandler(testLogger, driver)
			Expect(err).NotTo(HaveOccurred())

			httpResponseRecorder := httptest.NewRecorder()
			createRequest := voldriver.CreateRequest{Name: "some-volume"}
			createJSONRequest, err := json.Marshal(createRequest)
			Expect(err).NotTo(HaveOccurred())

			By("then fake serving the response using the handler")
			route, found := voldriver.Routes.FindRouteByName(voldriver.CreateRoute)
			Expect(found).To(BeTrue())

			path := fmt.Sprintf("http://0.0.0.0%s", route.Path)
			httpRequest, err := http.NewRequest("POST", path, bytes.NewReader(createJSONRequest))
			Expect(err).NotTo(HaveOccurred())
			handler.ServeHTTP(httpResponseRecorder, httpRequest)

			By("then expecting correct HTTP status code")
			Expect(httpResponseRecorder.Code).To(Equal(200))
		})

		It("should produce a handler with a remove route", func() {
			By("faking out the driver")
			driver := &voldriverfakes.FakeDriver{}
			driver.RemoveReturns(voldriver.ErrorResponse{})
			handler, err := driverhttp.NewHandler(testLogger, driver)
			Expect(err).NotTo(HaveOccurred())

			httpResponseRecorder := httptest.NewRecorder()
			removeRequest := voldriver.RemoveRequest{Name: "some-volume"}
			removeJSONRequest, err := json.Marshal(removeRequest)
			Expect(err).NotTo(HaveOccurred())

			By("then fake serving the response using the handler")
			route, found := voldriver.Routes.FindRouteByName(voldriver.RemoveRoute)
			Expect(found).To(BeTrue())

			path := fmt.Sprintf("http://0.0.0.0%s", route.Path)
			httpRequest, err := http.NewRequest("POST", path, bytes.NewReader(removeJSONRequest))
			Expect(err).NotTo(HaveOccurred())
			handler.ServeHTTP(httpResponseRecorder, httpRequest)

			By("then expecting correct HTTP status code")
			Expect(httpResponseRecorder.Code).To(Equal(200))
		})

		It("should produce a handler with a capabilities route", func() {
			By("faking out the driver")
			driver := &voldriverfakes.FakeDriver{}
			driver.CapabilitiesReturns(voldriver.CapabilitiesResponse{Capabilities: voldriver.CapabilityInfo{Scope: "global"}})
			handler, err := driverhttp.NewHandler(testLogger, driver)
			Expect(err).NotTo(HaveOccurred())

			By("then fake serving the response using the handler")
			route, found := voldriver.Routes.FindRouteByName(voldriver.CapabilitiesRoute)
			Expect(found).To(BeTrue())

			path := fmt.Sprintf("http://0.0.0.0%s", route.Path)
			httpRequest, err := http.NewRequest("POST", path, bytes.NewReader([]byte{}))
			Expect(err).NotTo(HaveOccurred())

			httpResponseRecorder := httptest.NewRecorder()
			handler.ServeHTTP(httpResponseRecorder, httpRequest)

			By("then deserialing the HTTP response")
			capabilitiesResponse := voldriver.CapabilitiesResponse{}
			body, err := ioutil.ReadAll(httpResponseRecorder.Body)
			err = json.Unmarshal(body, &capabilitiesResponse)

			By("then expecting correct JSON conversion")
			Expect(err).ToNot(HaveOccurred())
			Expect(capabilitiesResponse.Capabilities).Should(Equal(voldriver.CapabilityInfo{Scope: "global"}))
		})
	})
})
