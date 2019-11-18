package out_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/concourse/concourse-pipeline-resource/concourse"
	"github.com/concourse/concourse-pipeline-resource/fly/flyfakes"
	"github.com/concourse/concourse-pipeline-resource/logger"
	"github.com/concourse/concourse-pipeline-resource/out"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/robdimsdale/sanitizer"
)

var _ = Describe("Out", func() {
	var (
		sourcesDir string

		ginkgoLogger logger.Logger

		target        string
		username      string
		otherUsername string
		password      string
		otherPassword string
		teamName      string
		otherTeamName string
		pipelines     []concourse.Pipeline

		apiPipelines    []string
		setPipelinesErr error

		pipelineContents []string

		outRequest    concourse.OutRequest
		badOutRequest concourse.OutRequest
		command       *out.Command

		fakeFlyCommand *flyfakes.FakeCommand
	)

	BeforeEach(func() {
		fakeFlyCommand = &flyfakes.FakeCommand{}

		var err error
		sourcesDir, err = ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())

		target = "some target"
		username = "some user"
		otherUsername = "some other user"
		password = "some password"
		otherPassword = "some other password"
		teamName = "main"
		otherTeamName = "some-other-team"

		apiPipelines = []string{"pipeline-1", "pipeline-2", "pipeline-3", "pipeline-4"}
		setPipelinesErr = nil

		pipelineContents = make([]string, 4)

		pipelineContents[0] = `---
pipeline1: foo
`

		pipelineContents[1] = `---
pipeline2: foo
`

		pipelineContents[2] = `---
pipeline3: foo
`

		pipelineContents[3] = `---
pipeline4: foo
`

		pipelines = []concourse.Pipeline{
			{
				Name:       apiPipelines[0],
				ConfigFile: "pipeline_1.yml",
				VarsFiles: []string{
					"vars_1.yml",
					"vars_2.yml",
				},
				TeamName: teamName,
			},
			{
				Name:       apiPipelines[1],
				ConfigFile: "pipeline_2.yml",
				TeamName:   teamName,
				Unpaused:   true,
				Exposed:    true,
			},
			{
				Name:       apiPipelines[2],
				ConfigFile: "pipeline_3.yml",
				TeamName:   otherTeamName,
				Vars: map[string]interface{}{
					"launch-missiles": true,
				},
			},
			{
				Name:       apiPipelines[3],
				ConfigFile: "pipeline43.yml",
				TeamName:   otherTeamName,
				HideStdout: true,
			},
		}

		fakeFlyCommand.GetPipelineStub = func(name string) ([]byte, error) {
			defer GinkgoRecover()
			ginkgoLogger.Debugf("GetPipelineStub for: %s\n", name)

			switch name {
			case apiPipelines[0]:
				return []byte(pipelineContents[0]), nil
			case apiPipelines[1]:
				return []byte(pipelineContents[1]), nil
			case apiPipelines[2]:
				return []byte(pipelineContents[2]), nil
			case apiPipelines[3]:
				return []byte(pipelineContents[3]), nil
			default:
				Fail("Unexpected invocation of flyCommand.GetPipeline")
				return nil, nil
			}
		}

		outRequest = concourse.OutRequest{
			Source: concourse.Source{
				Target: target,
				Teams: []concourse.Team{
					{
						Name:     teamName,
						Username: username,
						Password: password,
					},
					{
						Name:     otherTeamName,
						Username: otherUsername,
						Password: otherPassword,
					},
				},
			},
			Params: concourse.OutParams{
				Pipelines: pipelines,
			},
		}

		badOutRequest = concourse.OutRequest{
			Source: concourse.Source{
				Target: target,
				Teams: []concourse.Team{
					{
						Name:     "the-wrong-team",
						Username: "the wrong username",
						Password: "the wrong password",
					},
				},
			},
			Params: concourse.OutParams{
				Pipelines: pipelines,
			},
		}
	})

	JustBeforeEach(func() {
		fakeFlyCommand.SetPipelineReturns(nil, setPipelinesErr)

		sanitized := concourse.SanitizedSource(outRequest.Source)
		sanitizer := sanitizer.NewSanitizer(sanitized, GinkgoWriter)

		ginkgoLogger = logger.NewLogger(sanitizer)

		command = out.NewCommand(ginkgoLogger, fakeFlyCommand, sourcesDir)
	})

	AfterEach(func() {
		err := os.RemoveAll(sourcesDir)
		Expect(err).NotTo(HaveOccurred())
	})

	It("invokes fly set-pipeline for each pipeline", func() {
		_, err := command.Run(outRequest)
		Expect(err).NotTo(HaveOccurred())

		Expect(fakeFlyCommand.SetPipelineCallCount()).To(Equal(len(pipelines)))

		for i, p := range pipelines {
			name, configFilepath, varsFilepaths, vars := fakeFlyCommand.SetPipelineArgsForCall(i)
			_, tname, _, _, _ := fakeFlyCommand.LoginArgsForCall(i)
			Expect(name).To(Equal(p.Name))
			Expect(tname).To(Equal(p.TeamName))
			Expect(configFilepath).To(Equal(filepath.Join(sourcesDir, p.ConfigFile)))

			// the first pipeline has vars files
			if i == 0 {
				Expect(varsFilepaths[0]).To(Equal(filepath.Join(sourcesDir, p.VarsFiles[0])))
				Expect(varsFilepaths[1]).To(Equal(filepath.Join(sourcesDir, p.VarsFiles[1])))
			}

			// the second pipeline has Unpaused and Exposed set to true
			if i == 1 {
				name := fakeFlyCommand.UnpausePipelineArgsForCall(0)
				Expect(name).To(Equal(p.Name))
				Expect(fakeFlyCommand.UnpausePipelineCallCount()).To(Equal(1))
				Expect(fakeFlyCommand.ExposePipelineCallCount()).To(Equal(1))
			}

			// the third pipeline has vars
			if i == 2 {
				Expect(vars).ToNot(BeNil())
				Expect(vars["launch-missiles"]).To(BeTrue())
				Expect(p.HideStdout).To(BeFalse())
			}

			// the fourth pipeline as hide_stdout: true
			if i == 3 {
				Expect(p.HideStdout).To(BeTrue())
			}
		}
	})

	It("returns provided version", func() {
		response, err := command.Run(outRequest)

		Expect(err).NotTo(HaveOccurred())

		Expect(response.Version[apiPipelines[0]]).To(Equal("4f4bd60b18bf697cc68dac9cb95537c2"))
	})

	It("returns metadata", func() {
		response, err := command.Run(outRequest)

		Expect(err).NotTo(HaveOccurred())

		Expect(response.Metadata).NotTo(BeNil())
	})

	Context("when insecure parses as true", func() {
		BeforeEach(func() {
			outRequest.Source.Insecure = "true"
		})

		It("invokes the login with insecure: true, without error", func() {
			_, err := command.Run(outRequest)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeFlyCommand.LoginCallCount()).To(Equal(6))
			_, _, _, _, insecure := fakeFlyCommand.LoginArgsForCall(0)

			Expect(insecure).To(BeTrue())
		})
	})

	Context("when insecure fails to parse into a boolean", func() {
		BeforeEach(func() {
			outRequest.Source.Insecure = "unparsable"
		})

		It("returns an error", func() {
			_, err := command.Run(outRequest)
			Expect(err).To(HaveOccurred())
		})
	})

	Context("when setting a pipeline that belongs to another team", func() {
		It("returns an error", func() {
			_, err := command.Run(badOutRequest)
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
			_, err := command.Run(outRequest)
			Expect(err).To(HaveOccurred())

			Expect(err).To(Equal(expectedErr))
		})
	})

	Context("when setting pipelines returns an error", func() {
		BeforeEach(func() {
			setPipelinesErr = fmt.Errorf("some error")
		})

		It("returns an error", func() {
			_, err := command.Run(outRequest)
			Expect(err).To(HaveOccurred())

			Expect(err).To(Equal(setPipelinesErr))
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
			_, err := command.Run(outRequest)
			Expect(err).To(HaveOccurred())

			Expect(err).To(Equal(expectedErr))
		})
	})
})
