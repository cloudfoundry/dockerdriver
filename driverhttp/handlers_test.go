package driverhttp_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"time"

	"code.cloudfoundry.org/dockerdriver"
	"code.cloudfoundry.org/dockerdriver/dockerdriverfakes"
	"code.cloudfoundry.org/dockerdriver/driverhttp"
	"code.cloudfoundry.org/lager/v3/lagertest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
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

var _ = Describe("Docker Driver Handlers", func() {

	var testLogger = lagertest.NewTestLogger("HandlersTest")

	var ErrorResponse = func(res *RecordingCloseNotifier) dockerdriver.ErrorResponse {
		response := dockerdriver.ErrorResponse{}

		body, err := io.ReadAll(res.Body)
		Expect(err).ToNot(HaveOccurred())

		err = json.Unmarshal(body, &response)
		Expect(err).ToNot(HaveOccurred())

		return response
	}

	Context("Activate", func() {
		var (
			err    error
			req    *http.Request
			res    *RecordingCloseNotifier
			driver *dockerdriverfakes.FakeDriver
			wg     sync.WaitGroup

			subject http.Handler
		)

		BeforeEach(func() {
			driver = &dockerdriverfakes.FakeDriver{}

			subject, err = driverhttp.NewHandler(testLogger, driver)
			Expect(err).NotTo(HaveOccurred())

			res = &RecordingCloseNotifier{
				ResponseRecorder: httptest.NewRecorder(),
				cn:               make(chan bool, 1),
			}

			route, found := dockerdriver.Routes.FindRouteByName(dockerdriver.ActivateRoute)
			Expect(found).To(BeTrue())

			path := fmt.Sprintf("http://0.0.0.0%s", route.Path)
			req, err = http.NewRequest("POST", path, bytes.NewReader([]byte{}))
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when activate is successful", func() {
			JustBeforeEach(func() {
				driver.ActivateReturns(dockerdriver.ActivateResponse{Implements: []string{"VolumeDriver"}})

				wg.Add(1)

				go func() {
					subject.ServeHTTP(res, req)
					wg.Done()
				}()

			})

			It("should respond 200 OK with VolumeDriver info", func() {
				wg.Wait()

				Expect(res.Code).To(Equal(200))

				activateResponse := dockerdriver.ActivateResponse{}

				body, err := io.ReadAll(res.Body)
				Expect(err).ToNot(HaveOccurred())

				err = json.Unmarshal(body, &activateResponse)
				Expect(err).ToNot(HaveOccurred())

				Expect(activateResponse.Implements).Should(Equal([]string{"VolumeDriver"}))
			})
		})

		Context("when activate hangs and the client closes the connection", func() {
			JustBeforeEach(func() {
				driver.ActivateStub = func(env dockerdriver.Env) dockerdriver.ActivateResponse {
					ctx := env.Context()
					logger := env.Logger()
					for true {
						time.Sleep(time.Second * 1)

						select {
						case <-ctx.Done():
							logger.Error("from-ctx", ctx.Err())
							return dockerdriver.ActivateResponse{Err: ctx.Err().Error()}
						}
					}
					return dockerdriver.ActivateResponse{}
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

				Expect(res.Code).To(Equal(200))

				activateResponse := dockerdriver.ActivateResponse{}

				body, err := io.ReadAll(res.Body)
				Expect(err).ToNot(HaveOccurred())

				err = json.Unmarshal(body, &activateResponse)
				Expect(err).ToNot(HaveOccurred())

				Expect(activateResponse.Err).Should(ContainSubstring("context canceled"))
			})
		})
	})

	Context("List", func() {
		var (
			err    error
			req    *http.Request
			res    *RecordingCloseNotifier
			driver *dockerdriverfakes.FakeDriver
			wg     sync.WaitGroup

			subject http.Handler
		)

		BeforeEach(func() {
			driver = &dockerdriverfakes.FakeDriver{}

			subject, err = driverhttp.NewHandler(testLogger, driver)
			Expect(err).NotTo(HaveOccurred())

			res = &RecordingCloseNotifier{
				ResponseRecorder: httptest.NewRecorder(),
				cn:               make(chan bool, 1),
			}

			route, found := dockerdriver.Routes.FindRouteByName(dockerdriver.ListRoute)
			Expect(found).To(BeTrue())

			path := fmt.Sprintf("http://0.0.0.0%s", route.Path)
			req, err = http.NewRequest("POST", path, bytes.NewReader([]byte{}))
			Expect(err).NotTo(HaveOccurred())

		})

		Context("when list is successful", func() {
			JustBeforeEach(func() {
				volume := dockerdriver.VolumeInfo{
					Name:       "fake-volume",
					Mountpoint: "fake-mountpoint",
				}
				listResponse := dockerdriver.ListResponse{
					Volumes: []dockerdriver.VolumeInfo{volume},
					Err:     "",
				}

				driver.ListReturns(listResponse)

				wg.Add(1)

				go func() {
					subject.ServeHTTP(res, req)
					wg.Done()
				}()
			})

			It("should respond 200 OK with the volume info", func() {
				wg.Wait()

				Expect(res.Code).To(Equal(200))

				listResponse := dockerdriver.ListResponse{}

				body, err := io.ReadAll(res.Body)
				Expect(err).ToNot(HaveOccurred())

				err = json.Unmarshal(body, &listResponse)
				Expect(err).ToNot(HaveOccurred())

				Expect(listResponse.Err).Should(BeEmpty())
				Expect(listResponse.Volumes[0].Name).Should(Equal("fake-volume"))
			})
		})

		Context("when the list hangs and the client closes the connection", func() {
			JustBeforeEach(func() {
				driver.ListStub = func(env dockerdriver.Env) dockerdriver.ListResponse {
					ctx := env.Context()
					logger := env.Logger()
					for true {
						time.Sleep(time.Second * 1)

						select {
						case <-ctx.Done():
							logger.Error("from-ctx", ctx.Err())
							return dockerdriver.ListResponse{Err: ctx.Err().Error()}
						}
					}
					return dockerdriver.ListResponse{}
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

				Expect(res.Code).To(Equal(200))

				listResponse := dockerdriver.ListResponse{}

				body, err := io.ReadAll(res.Body)
				Expect(err).ToNot(HaveOccurred())

				err = json.Unmarshal(body, &listResponse)
				Expect(err).ToNot(HaveOccurred())

				Expect(listResponse.Err).Should(ContainSubstring("context canceled"))
			})
		})
	})

	Context("Mount", func() {
		var (
			err    error
			req    *http.Request
			res    *RecordingCloseNotifier
			driver *dockerdriverfakes.FakeDriver
			wg     sync.WaitGroup

			subject http.Handler
		)

		var ExpectMountPointToEqual = func(value string) dockerdriver.MountResponse {
			mountResponse := dockerdriver.MountResponse{}
			body, err := io.ReadAll(res.Body)

			err = json.Unmarshal(body, &mountResponse)
			Expect(err).ToNot(HaveOccurred())

			Expect(mountResponse.Mountpoint).Should(Equal(value))
			return mountResponse
		}

		BeforeEach(func() {
			driver = &dockerdriverfakes.FakeDriver{}

			subject, err = driverhttp.NewHandler(testLogger, driver)
			Expect(err).NotTo(HaveOccurred())

			res = &RecordingCloseNotifier{
				ResponseRecorder: httptest.NewRecorder(),
				cn:               make(chan bool, 1),
			}

			route, found := dockerdriver.Routes.FindRouteByName(dockerdriver.MountRoute)
			Expect(found).To(BeTrue())

			path := fmt.Sprintf("http://0.0.0.0%s", route.Path)

			MountRequest := dockerdriver.MountRequest{
				Name: "some-volume",
			}
			mountJSONRequest, err := json.Marshal(MountRequest)
			Expect(err).NotTo(HaveOccurred())

			req, err = http.NewRequest("POST", path, bytes.NewReader(mountJSONRequest))
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when mount is successful", func() {

			JustBeforeEach(func() {
				driver.MountReturns(dockerdriver.MountResponse{Mountpoint: "dummy_path"})

				wg.Add(1)

				go func() {
					subject.ServeHTTP(res, req)
					wg.Done()
				}()

			})

			It("should respond 200 OK with the mountpoint", func() {
				wg.Wait()

				Expect(res.Code).To(Equal(200))

				ExpectMountPointToEqual("dummy_path")
			})
		})

		Context("when the mount hangs and the client closes the connection", func() {
			JustBeforeEach(func() {
				driver.MountStub = func(env dockerdriver.Env, mountRequest dockerdriver.MountRequest) dockerdriver.MountResponse {
					ctx := env.Context()
					logger := env.Logger()
					for true {
						time.Sleep(time.Second * 1)

						select {
						case <-ctx.Done():
							logger.Error("from-ctx", ctx.Err())
							return dockerdriver.MountResponse{Err: ctx.Err().Error()}
						}
					}
					return dockerdriver.MountResponse{}
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

				Expect(res.Code).To(Equal(200))

				mountResponse := ExpectMountPointToEqual("")
				Expect(mountResponse.Err).Should(ContainSubstring("context canceled"))
			})
		})
	})

	Context("Unmount", func() {
		var (
			err    error
			req    *http.Request
			res    *RecordingCloseNotifier
			driver *dockerdriverfakes.FakeDriver
			wg     sync.WaitGroup

			subject http.Handler
		)

		BeforeEach(func() {
			driver = &dockerdriverfakes.FakeDriver{}

			subject, err = driverhttp.NewHandler(testLogger, driver)
			Expect(err).NotTo(HaveOccurred())

			unmountRequest := dockerdriver.UnmountRequest{}
			unmountJSONRequest, err := json.Marshal(unmountRequest)
			Expect(err).NotTo(HaveOccurred())

			res = &RecordingCloseNotifier{
				ResponseRecorder: httptest.NewRecorder(),
				cn:               make(chan bool, 1),
			}

			route, found := dockerdriver.Routes.FindRouteByName(dockerdriver.UnmountRoute)
			Expect(found).To(BeTrue())

			path := fmt.Sprintf("http://0.0.0.0%s", route.Path)
			req, err = http.NewRequest("POST", path, bytes.NewReader(unmountJSONRequest))
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when unmount is successful", func() {
			JustBeforeEach(func() {
				driver.UnmountReturns(dockerdriver.ErrorResponse{})

				wg.Add(1)

				go func() {
					subject.ServeHTTP(res, req)
					wg.Done()
				}()

			})

			It("should respond 200 OK", func() {
				wg.Wait()
				Expect(res.Code).To(Equal(200))
			})
		})

		Context("when the unmount hangs and the client closes the connection", func() {
			JustBeforeEach(func() {
				driver.UnmountStub = func(env dockerdriver.Env, unmountRequest dockerdriver.UnmountRequest) dockerdriver.ErrorResponse {
					ctx := env.Context()
					logger := env.Logger()
					for true {
						time.Sleep(time.Second * 1)

						select {
						case <-ctx.Done():
							logger.Error("from-ctx", ctx.Err())
							return dockerdriver.ErrorResponse{Err: ctx.Err().Error()}
						}
					}
					return dockerdriver.ErrorResponse{}
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

				Expect(res.Code).To(Equal(200))

				response := ErrorResponse(res)
				Expect(response.Err).Should(ContainSubstring("context canceled"))
			})
		})
	})

	Context("Get", func() {
		var (
			err    error
			req    *http.Request
			res    *RecordingCloseNotifier
			driver *dockerdriverfakes.FakeDriver
			wg     sync.WaitGroup

			subject http.Handler
		)

		BeforeEach(func() {
			driver = &dockerdriverfakes.FakeDriver{}

			subject, err = driverhttp.NewHandler(testLogger, driver)
			Expect(err).NotTo(HaveOccurred())

			res = &RecordingCloseNotifier{
				ResponseRecorder: httptest.NewRecorder(),
				cn:               make(chan bool, 1),
			}

			getRequest := dockerdriver.GetRequest{}
			getJSONRequest, err := json.Marshal(getRequest)
			Expect(err).NotTo(HaveOccurred())

			By("then fake serving the response using the handler")
			route, found := dockerdriver.Routes.FindRouteByName(dockerdriver.GetRoute)
			Expect(found).To(BeTrue())

			path := fmt.Sprintf("http://0.0.0.0%s", route.Path)
			req, err = http.NewRequest("POST", path, bytes.NewReader(getJSONRequest))
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when get is successful", func() {
			JustBeforeEach(func() {
				driver.GetReturns(dockerdriver.GetResponse{Volume: dockerdriver.VolumeInfo{Name: "some-volume", Mountpoint: "dummy_path"}})

				wg.Add(1)

				go func() {
					subject.ServeHTTP(res, req)
					wg.Done()
				}()

			})

			It("should return 200 OK", func() {
				wg.Wait()

				Expect(res.Code).To(Equal(200))

				getResponse := dockerdriver.GetResponse{}
				body, err := io.ReadAll(res.Body)
				err = json.Unmarshal(body, &getResponse)
				Expect(err).ToNot(HaveOccurred())

				Expect(getResponse.Volume.Name).Should(Equal("some-volume"))
				Expect(getResponse.Volume.Mountpoint).Should(Equal("dummy_path"))
			})
		})

		Context("when the get hangs and the client closes the connection", func() {
			JustBeforeEach(func() {
				driver.GetStub = func(env dockerdriver.Env, getRequest dockerdriver.GetRequest) dockerdriver.GetResponse {
					ctx := env.Context()
					logger := env.Logger()
					for true {
						time.Sleep(time.Second * 1)

						select {
						case <-ctx.Done():
							logger.Error("from-ctx", ctx.Err())
							return dockerdriver.GetResponse{Err: ctx.Err().Error()}
						}
					}
					return dockerdriver.GetResponse{}
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

				Expect(res.Code).To(Equal(200))

				response := ErrorResponse(res)
				Expect(response.Err).Should(ContainSubstring("context canceled"))
			})
		})
	})

	Context("Path", func() {
		var (
			err    error
			req    *http.Request
			res    *RecordingCloseNotifier
			driver *dockerdriverfakes.FakeDriver
			wg     sync.WaitGroup

			subject http.Handler
		)

		BeforeEach(func() {
			driver = &dockerdriverfakes.FakeDriver{}

			subject, err = driverhttp.NewHandler(testLogger, driver)
			Expect(err).NotTo(HaveOccurred())

			res = &RecordingCloseNotifier{
				ResponseRecorder: httptest.NewRecorder(),
				cn:               make(chan bool, 1),
			}

			pathRequest := dockerdriver.PathRequest{Name: "some-volume"}
			pathJSONRequest, err := json.Marshal(pathRequest)
			Expect(err).NotTo(HaveOccurred())

			route, found := dockerdriver.Routes.FindRouteByName(dockerdriver.PathRoute)
			Expect(found).To(BeTrue())

			path := fmt.Sprintf("http://0.0.0.0%s", route.Path)
			req, err = http.NewRequest("POST", path, bytes.NewReader(pathJSONRequest))
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when path is successful", func() {
			JustBeforeEach(func() {
				driver.PathReturns(dockerdriver.PathResponse{
					Mountpoint: "/some/mountpoint",
				})

				wg.Add(1)

				go func() {
					subject.ServeHTTP(res, req)
					wg.Done()
				}()

			})

			It("should respond 200 OK", func() {
				wg.Wait()

				Expect(res.Code).To(Equal(200))

				pathResponse := dockerdriver.PathResponse{}
				body, err := io.ReadAll(res.Body)
				err = json.Unmarshal(body, &pathResponse)
				Expect(err).ToNot(HaveOccurred())

				Expect(pathResponse.Mountpoint).Should(Equal("/some/mountpoint"))
			})
		})

		Context("when the path hangs and the client closes the connection", func() {
			JustBeforeEach(func() {
				driver.PathStub = func(env dockerdriver.Env, pathRequest dockerdriver.PathRequest) dockerdriver.PathResponse {
					ctx := env.Context()
					logger := env.Logger()
					for true {
						time.Sleep(time.Second * 1)

						select {
						case <-ctx.Done():
							logger.Error("from-ctx", ctx.Err())
							return dockerdriver.PathResponse{Err: ctx.Err().Error()}
						}
					}
					return dockerdriver.PathResponse{}
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

				Expect(res.Code).To(Equal(200))

				response := ErrorResponse(res)
				Expect(response.Err).Should(ContainSubstring("context canceled"))
			})
		})
	})

	Context("Create", func() {
		var (
			err    error
			req    *http.Request
			res    *RecordingCloseNotifier
			driver *dockerdriverfakes.FakeDriver
			wg     sync.WaitGroup

			subject http.Handler
		)

		BeforeEach(func() {
			driver = &dockerdriverfakes.FakeDriver{}

			subject, err = driverhttp.NewHandler(testLogger, driver)
			Expect(err).NotTo(HaveOccurred())

			res = &RecordingCloseNotifier{
				ResponseRecorder: httptest.NewRecorder(),
				cn:               make(chan bool, 1),
			}

			createRequest := dockerdriver.CreateRequest{Name: "some-volume"}
			createJSONRequest, err := json.Marshal(createRequest)
			Expect(err).NotTo(HaveOccurred())

			route, found := dockerdriver.Routes.FindRouteByName(dockerdriver.CreateRoute)
			Expect(found).To(BeTrue())

			path := fmt.Sprintf("http://0.0.0.0%s", route.Path)
			req, err = http.NewRequest("POST", path, bytes.NewReader(createJSONRequest))
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when create is successful", func() {
			JustBeforeEach(func() {
				driver.CreateReturns(dockerdriver.ErrorResponse{})

				wg.Add(1)

				go func() {
					subject.ServeHTTP(res, req)
					wg.Done()
				}()

			})

			It("should respond 200 OK", func() {
				wg.Wait()

				Expect(res.Code).To(Equal(200))

				response := ErrorResponse(res)
				Expect(response.Err).Should(BeEmpty())
			})
		})

		Context("when the create hangs and the client closes the connection", func() {
			JustBeforeEach(func() {
				driver.CreateStub = func(env dockerdriver.Env, createRequest dockerdriver.CreateRequest) dockerdriver.ErrorResponse {
					ctx := env.Context()
					logger := env.Logger()
					for true {
						time.Sleep(time.Second * 1)

						select {
						case <-ctx.Done():
							logger.Error("from-ctx", ctx.Err())
							return dockerdriver.ErrorResponse{Err: ctx.Err().Error()}
						}
					}
					return dockerdriver.ErrorResponse{}
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

				Expect(res.Code).To(Equal(200))

				response := ErrorResponse(res)
				Expect(response.Err).Should(ContainSubstring("context canceled"))
			})
		})
	})

	Context("Remove", func() {
		var (
			err    error
			req    *http.Request
			res    *RecordingCloseNotifier
			driver *dockerdriverfakes.FakeDriver
			wg     sync.WaitGroup

			subject http.Handler
		)

		BeforeEach(func() {
			driver = &dockerdriverfakes.FakeDriver{}

			subject, err = driverhttp.NewHandler(testLogger, driver)
			Expect(err).NotTo(HaveOccurred())

			res = &RecordingCloseNotifier{
				ResponseRecorder: httptest.NewRecorder(),
				cn:               make(chan bool, 1),
			}

			removeRequest := dockerdriver.RemoveRequest{Name: "some-volume"}
			removeJSONRequest, err := json.Marshal(removeRequest)
			Expect(err).NotTo(HaveOccurred())

			route, found := dockerdriver.Routes.FindRouteByName(dockerdriver.RemoveRoute)
			Expect(found).To(BeTrue())

			path := fmt.Sprintf("http://0.0.0.0%s", route.Path)
			req, err = http.NewRequest("POST", path, bytes.NewReader(removeJSONRequest))
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when remove is successful", func() {
			JustBeforeEach(func() {
				driver.RemoveReturns(dockerdriver.ErrorResponse{})

				wg.Add(1)

				go func() {
					subject.ServeHTTP(res, req)
					wg.Done()
				}()

			})

			It("should respond 200 OK", func() {
				wg.Wait()

				Expect(res.Code).To(Equal(200))

				response := ErrorResponse(res)
				Expect(response.Err).Should(BeEmpty())
			})
		})

		Context("when the remove hangs and the client closes the connection", func() {
			JustBeforeEach(func() {
				driver.RemoveStub = func(env dockerdriver.Env, removeRequest dockerdriver.RemoveRequest) dockerdriver.ErrorResponse {
					ctx := env.Context()
					logger := env.Logger()
					for true {
						time.Sleep(time.Second * 1)

						select {
						case <-ctx.Done():
							logger.Error("from-ctx", ctx.Err())
							return dockerdriver.ErrorResponse{Err: ctx.Err().Error()}
						}
					}
					return dockerdriver.ErrorResponse{}
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

				Expect(res.Code).To(Equal(200))

				response := ErrorResponse(res)
				Expect(response.Err).Should(ContainSubstring("context canceled"))
			})
		})
	})

	Context("Capabilities", func() {
		var (
			err    error
			req    *http.Request
			res    *RecordingCloseNotifier
			driver *dockerdriverfakes.FakeDriver
			wg     sync.WaitGroup

			subject http.Handler
		)

		BeforeEach(func() {
			driver = &dockerdriverfakes.FakeDriver{}

			subject, err = driverhttp.NewHandler(testLogger, driver)
			Expect(err).NotTo(HaveOccurred())

			res = &RecordingCloseNotifier{
				ResponseRecorder: httptest.NewRecorder(),
				cn:               make(chan bool, 1),
			}

			route, found := dockerdriver.Routes.FindRouteByName(dockerdriver.CapabilitiesRoute)
			Expect(found).To(BeTrue())

			path := fmt.Sprintf("http://0.0.0.0%s", route.Path)
			req, err = http.NewRequest("POST", path, bytes.NewReader([]byte{}))
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when capabilities is successful", func() {
			JustBeforeEach(func() {
				driver.CapabilitiesReturns(dockerdriver.CapabilitiesResponse{Capabilities: dockerdriver.CapabilityInfo{Scope: "global"}})

				wg.Add(1)

				go func() {
					subject.ServeHTTP(res, req)
					wg.Done()
				}()

			})

			It("should respond 200 OK", func() {
				wg.Wait()

				Expect(res.Code).To(Equal(200))

				capabilitiesResponse := dockerdriver.CapabilitiesResponse{}
				body, err := io.ReadAll(res.Body)
				err = json.Unmarshal(body, &capabilitiesResponse)
				Expect(err).ToNot(HaveOccurred())

				Expect(capabilitiesResponse.Capabilities).Should(Equal(dockerdriver.CapabilityInfo{Scope: "global"}))
			})
		})
	})
})
