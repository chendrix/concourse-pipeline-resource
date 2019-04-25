package check_test

import (
	"crypto/md5"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/concourse/concourse-pipeline-resource/check"
	"github.com/concourse/concourse-pipeline-resource/concourse"
	"github.com/concourse/concourse-pipeline-resource/fly/flyfakes"
	"github.com/concourse/concourse-pipeline-resource/logger"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/robdimsdale/sanitizer"
)

var _ = Describe("Check", func() {
	var (
		tempDir     string
		logFilePath string

		ginkgoLogger logger.Logger

		target string

		expectedResponse concourse.CheckResponse
		pipelineContents []string

		checkRequest concourse.CheckRequest
		command      *check.Command

		pipelinesErr   error
		pipelines      []string
		fakeFlyCommand *flyfakes.FakeCommand
	)

	BeforeEach(func() {
		fakeFlyCommand = &flyfakes.FakeCommand{}

		pipelinesErr = nil
		pipelines = []string{"pipeline 1", "pipeline 2"}

		pipelineContents = make([]string, 2)

		pipelineContents[0] = `---
pipeline1: foo
`

		pipelineContents[1] = `---
pipeline2: foo
`

		fakeFlyCommand.GetPipelineStub = func(name string) ([]byte, error) {
			ginkgoLogger.Debugf("GetPipelineStub for: %s\n", name)

			switch name {
			case pipelines[0]:
				return []byte(pipelineContents[0]), nil
			case pipelines[1]:
				return []byte(pipelineContents[1]), nil
			default:
				Fail("Unexpected invocation of flyCommand.GetPipeline")
				return nil, nil
			}
		}

		var err error
		tempDir, err = ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())

		logFilePath = filepath.Join(tempDir, "concourse-pipeline-resource-check.log1234")
		err = ioutil.WriteFile(logFilePath, []byte("initial log content"), os.ModePerm)
		Expect(err).NotTo(HaveOccurred())

		target = "some target"
		teams := []concourse.Team{
			{
				Name:     "main",
				Username: "some user",
				Password: "some password",
			},
			{
				Name:      "another team",
				Username:  "another user",
				Password:  "another password",
				Pipelines: []string{"pipeline 2"},
			},
		}

		checkRequest = concourse.CheckRequest{
			Source: concourse.Source{
				Target: target,
				Teams:  teams,
			},
		}

		sanitized := concourse.SanitizedSource(checkRequest.Source)
		sanitizer := sanitizer.NewSanitizer(sanitized, GinkgoWriter)

		ginkgoLogger = logger.NewLogger(sanitizer)

		expectedResponse = []concourse.Version{
			{
				fmt.Sprintf("%s:%s", teams[0].Username, pipelines[0]): fmt.Sprintf("%x", md5.Sum([]byte(pipelineContents[0]))),
				fmt.Sprintf("%s:%s", teams[0].Username, pipelines[1]): fmt.Sprintf("%x", md5.Sum([]byte(pipelineContents[1]))),
				fmt.Sprintf("%s:%s", teams[1].Username, pipelines[1]): fmt.Sprintf("%x", md5.Sum([]byte(pipelineContents[1]))),
			},
		}

		command = check.NewCommand(
			ginkgoLogger,
			logFilePath,
			fakeFlyCommand,
		)
	})

	AfterEach(func() {
		err := os.RemoveAll(tempDir)
		Expect(err).NotTo(HaveOccurred())
	})

	JustBeforeEach(func() {
		fakeFlyCommand.PipelinesReturns(pipelines, pipelinesErr)
	})

	It("returns pipelines checksum without error", func() {
		response, err := command.Run(checkRequest)
		Expect(err).NotTo(HaveOccurred())

		Expect(response).To(Equal(expectedResponse))
	})

	Context("when the most recent version is provided", func() {
		BeforeEach(func() {
			checkRequest.Version = concourse.Version{
				pipelines[0]: fmt.Sprintf("%x", md5.Sum([]byte(pipelineContents[0]))),
				pipelines[1]: fmt.Sprintf("%x", md5.Sum([]byte(pipelineContents[1]))),
			}
		})

		It("returns the most recent version", func() {
			response, err := command.Run(checkRequest)
			Expect(err).NotTo(HaveOccurred())

			Expect(response).To(Equal(expectedResponse))
		})
	})

	Context("when some other version is provided", func() {
		BeforeEach(func() {
			checkRequest.Version = concourse.Version{
				"foo": "bar",
			}
		})

		It("returns the most recent version", func() {

		})
	})

	Context("when log files already exist", func() {
		var (
			otherFilePath1 string
			otherFilePath2 string
		)

		BeforeEach(func() {
			otherFilePath1 = filepath.Join(tempDir, "concourse-pipeline-resource-check.log1")
			err := ioutil.WriteFile(otherFilePath1, []byte("initial log content"), os.ModePerm)
			Expect(err).NotTo(HaveOccurred())

			otherFilePath2 = filepath.Join(tempDir, "concourse-pipeline-resource-check.log2")
			err = ioutil.WriteFile(otherFilePath2, []byte("initial log content"), os.ModePerm)
			Expect(err).NotTo(HaveOccurred())
		})

		It("removes the other log files", func() {
			_, err := command.Run(checkRequest)
			Expect(err).NotTo(HaveOccurred())

			_, err = os.Stat(otherFilePath1)
			Expect(err).To(HaveOccurred())
			Expect(os.IsNotExist(err)).To(BeTrue())

			_, err = os.Stat(otherFilePath2)
			Expect(err).To(HaveOccurred())
			Expect(os.IsNotExist(err)).To(BeTrue())

			_, err = os.Stat(logFilePath)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("when insecure parses as true", func() {
		BeforeEach(func() {
			checkRequest.Source.Insecure = "true"
		})

		It("invokes the login with insecure: true, without error", func() {
			_, err := command.Run(checkRequest)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeFlyCommand.LoginCallCount()).To(Equal(2))
			_, _, _, _, insecure := fakeFlyCommand.LoginArgsForCall(0)

			Expect(insecure).To(BeTrue())
		})
	})

	Context("when insecure fails to parse into a boolean", func() {
		BeforeEach(func() {
			checkRequest.Source.Insecure = "unparsable"
		})

		It("returns an error", func() {
			_, err := command.Run(checkRequest)
			Expect(err).To(HaveOccurred())
		})
	})

	Context("when login returns an error", func() {
		var (
			expectedErr error
		)

		BeforeEach(func() {
			expectedErr = fmt.Errorf("login failed")
			fakeFlyCommand.LoginReturns(nil, expectedErr)
		})

		It("returns an error", func() {
			_, err := command.Run(checkRequest)
			Expect(err).To(HaveOccurred())

			Expect(err).To(Equal(expectedErr))
		})
	})

	Context("when getting pipelines returns an error", func() {
		BeforeEach(func() {
			pipelinesErr = fmt.Errorf("some error")
		})

		It("forwards the error", func() {
			_, err := command.Run(checkRequest)
			Expect(err).To(HaveOccurred())

			Expect(err).To(Equal(pipelinesErr))
		})
	})

	Context("when getting a non existing pipeline", func() {
		BeforeEach(func() {
			fakeFlyCommand.GetPipelineReturns(nil, fmt.Errorf("error: pipeline not found"))
		})

		It("catches the error", func() {
			_, err := command.Run(checkRequest)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("when calling fly to get pipeline config returns an error", func() {
		var (
			expectedErr error
		)

		BeforeEach(func() {
			expectedErr = fmt.Errorf("error executing fly")

			fakeFlyCommand.GetPipelineReturns(nil, expectedErr)
		})

		It("returns an error", func() {
			_, err := command.Run(checkRequest)
			Expect(err).To(HaveOccurred())

			Expect(err).To(Equal(expectedErr))
		})
	})
})
