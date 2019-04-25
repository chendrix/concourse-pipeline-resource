package in_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/concourse/concourse-pipeline-resource/concourse"
	"github.com/concourse/concourse-pipeline-resource/fly/flyfakes"
	"github.com/concourse/concourse-pipeline-resource/in"
	"github.com/concourse/concourse-pipeline-resource/logger"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/robdimsdale/sanitizer"
)

var _ = Describe("In", func() {
	var (
		downloadDir string

		ginkgoLogger logger.Logger

		target string
		teams  []concourse.Team

		inRequest concourse.InRequest
		command   *in.Command

		fakeFlyCommand *flyfakes.FakeCommand

		pipelines        []string
		pipelineVersions []string

		pipelinesErr error

		pipelineContents []string
	)

	BeforeEach(func() {
		fakeFlyCommand = &flyfakes.FakeCommand{}

		var err error
		downloadDir, err = ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())

		target = "some target"
		teams = []concourse.Team{
			{
				Name:     "main",
				Username: "some user",
				Password: "some password",
			},
			{
				Name:      "another team",
				Username:  "another user",
				Password:  "another password",
				Pipelines: []string{"pipeline-2"},
			},
		}

		pipelinesErr = nil
		pipelines = []string{"pipeline-1", "pipeline-2"}
		pipelineVersions = []string{"1234", "2345"}
		pipelineContents = make([]string, 2)

		pipelineContents[0] = `---
pipeline1: foo
`

		pipelineContents[1] = `---
pipeline2: foo
`

		inRequest = concourse.InRequest{
			Source: concourse.Source{
				Target: target,
				Teams:  teams,
			},
			Version: concourse.Version{
				pipelines[0]: pipelineVersions[0],
			},
		}

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
	})

	JustBeforeEach(func() {
		fakeFlyCommand.PipelinesReturns(pipelines, pipelinesErr)

		sanitized := concourse.SanitizedSource(inRequest.Source)
		sanitizer := sanitizer.NewSanitizer(sanitized, GinkgoWriter)

		ginkgoLogger = logger.NewLogger(sanitizer)

		command = in.NewCommand(ginkgoLogger, fakeFlyCommand, downloadDir)
	})

	AfterEach(func() {
		err := os.RemoveAll(downloadDir)
		Expect(err).NotTo(HaveOccurred())
	})

	It("downloads all pipeline configs to the target directory", func() {
		_, err := command.Run(inRequest)

		Expect(err).NotTo(HaveOccurred())

		files, err := ioutil.ReadDir(downloadDir)
		Expect(err).NotTo(HaveOccurred())

		Expect(files).To(HaveLen(len(pipelines) + len(teams[1].Pipelines)))

		Expect(files[0].Name()).To(MatchRegexp("%s-%s.yml", teams[1].Name, pipelines[1]))

		contents, err := ioutil.ReadFile(filepath.Join(downloadDir, files[0].Name()))
		Expect(err).NotTo(HaveOccurred())
		Expect(string(contents)).To(Equal(pipelineContents[1]))

		Expect(files[1].Name()).To(MatchRegexp("%s.yml", pipelines[0]))

		contents, err = ioutil.ReadFile(filepath.Join(downloadDir, files[1].Name()))
		Expect(err).NotTo(HaveOccurred())
		Expect(string(contents)).To(Equal(pipelineContents[0]))

		Expect(files[2].Name()).To(MatchRegexp("%s.yml", pipelines[1]))

		contents, err = ioutil.ReadFile(filepath.Join(downloadDir, files[2].Name()))
		Expect(err).NotTo(HaveOccurred())
		Expect(string(contents)).To(Equal(pipelineContents[1]))
	})

	It("returns provided version", func() {
		response, err := command.Run(inRequest)

		Expect(err).NotTo(HaveOccurred())

		Expect(response.Version[pipelines[0]]).To(Equal(pipelineVersions[0]))
	})

	It("returns metadata", func() {
		response, err := command.Run(inRequest)

		Expect(err).NotTo(HaveOccurred())

		Expect(response.Metadata).NotTo(BeNil())
	})

	Context("when insecure parses as true", func() {
		BeforeEach(func() {
			inRequest.Source.Insecure = "true"
		})

		It("invokes the login with insecure: true, without error", func() {
			_, err := command.Run(inRequest)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeFlyCommand.LoginCallCount()).To(Equal(2))
			_, _, _, _, insecure := fakeFlyCommand.LoginArgsForCall(0)

			Expect(insecure).To(BeTrue())
		})
	})

	Context("when insecure fails to parse into a boolean", func() {
		BeforeEach(func() {
			inRequest.Source.Insecure = "unparsable"
		})

		It("returns an error", func() {
			_, err := command.Run(inRequest)
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
			_, err := command.Run(inRequest)
			Expect(err).To(HaveOccurred())

			Expect(err).To(Equal(expectedErr))
		})
	})

	Context("when getting pipelines returns an error", func() {
		BeforeEach(func() {
			pipelinesErr = fmt.Errorf("some error")
		})

		It("returns an error", func() {
			_, err := command.Run(inRequest)
			Expect(err).To(HaveOccurred())

			Expect(err).To(Equal(pipelinesErr))
		})
	})

	Context("when getting a non existing pipeline", func() {
		BeforeEach(func() {
			fakeFlyCommand.GetPipelineReturns(nil, fmt.Errorf("error: pipeline not found"))
		})

		It("catches the error", func() {
			_, err := command.Run(inRequest)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("when getting pipeline returns an error", func() {
		var (
			expectedErr error
		)

		BeforeEach(func() {
			expectedErr = fmt.Errorf("some error")
			fakeFlyCommand.GetPipelineReturns(nil, expectedErr)
		})

		It("returns an error", func() {
			_, err := command.Run(inRequest)
			Expect(err).To(Equal(expectedErr))
		})
	})
})
