package out

import (
	"log"
	"path/filepath"
	"strconv"

	"github.com/concourse/atc"
	"github.com/concourse/fly/template"
	"github.com/robdimsdale/concourse-pipeline-resource/concourse"
	"github.com/robdimsdale/concourse-pipeline-resource/concourse/api"
)

const (
	apiPrefix = "/api/v1"
)

//go:generate counterfeiter . Client
type Client interface {
	Pipelines(teamName string) ([]api.Pipeline, error)
	PipelineConfig(teamName string, pipelineName string) (config atc.Config, rawConfig string, version string, err error)
}

//go:generate counterfeiter . Logger
type Logger interface {
	Debugf(format string, a ...interface{}) (n int, err error)
}

//go:generate counterfeiter . PipelineSetter
type PipelineSetter interface {
	SetPipeline(
		teamName string,
		pipelineName string,
		configPath string,
		templateVariables template.Variables,
		templateVariablesFiles []string,
	) error
}

//go:generate counterfeiter . PipelineUnpauser
type PipelineUnpauser interface {
	UnpausePipeline(
		teamName string,
		pipelineName string,
	) error
}

//go:generate counterfeiter . PipelineDeleter
type PipelineDeleter interface {
	DeletePipeline(
		teamName string,
		pipelineName string,
	) error
}

type OutCommand struct {
	logger            Logger
	binaryVersion     string
	apiClient         Client
	sourcesDir        string
	pipelineSetter    PipelineSetter
	pipelineUnpauser  PipelineUnpauser
	pipelineDeleter   PipelineDeleter
}

func NewOutCommand(
	binaryVersion string,
	logger Logger,
	pipelineSetter PipelineSetter,
	pipelineUnpauser PipelineUnpauser,
	pipelineDeleter PipelineDeleter,
	apiClient Client,
	sourcesDir string,
) *OutCommand {
	return &OutCommand{
		logger:            logger,
		binaryVersion:     binaryVersion,
		pipelineSetter:    pipelineSetter,
		pipelineUnpauser:  pipelineUnpauser,
		pipelineDeleter:   pipelineDeleter,
		apiClient:         apiClient,
		sourcesDir:        sourcesDir,
	}
}

func (c *OutCommand) Run(input concourse.OutRequest) (concourse.OutResponse, error) {
	c.logger.Debugf("Received input: %+v\n", input)

	pipelines := input.Params.Pipelines

	c.logger.Debugf("Input pipelines: %+v\n", pipelines)

	c.logger.Debugf("Setting pipelines\n")
	for _, p := range pipelines {
		present := true

		if p.Present != "" {
			var err error
			present, err = strconv.ParseBool(p.Present)
			if err != nil {
				log.Fatalln("Invalid value for present: %v", p.Present)
			}
		}

		if !present {
			c.logger.Debugf("Deleting pipeline: %v\n", p.TeamName)

			err := c.pipelineDeleter.DeletePipeline(p.TeamName, p.Name)
			if err != nil {
				return concourse.OutResponse{}, err
			}

			continue
		}

		configFilepath := filepath.Join(c.sourcesDir, p.ConfigFile)

		var varsFilepaths []string
		for _, v := range p.VarsFiles {
			varFilepath := filepath.Join(c.sourcesDir, v)
			varsFilepaths = append(varsFilepaths, varFilepath)
		}

		var templateVariables template.Variables
		err := c.pipelineSetter.SetPipeline(
			p.TeamName,
			p.Name,
			configFilepath,
			templateVariables,
			varsFilepaths,
		)
		if err != nil {
			return concourse.OutResponse{}, err
		}

		unpause := false

		if input.Params.Unpause != "" {
			unpause, err = strconv.ParseBool(input.Params.Unpause)
			if err != nil {
				log.Fatalln("Invalid value for unpause: %v", input.Params.Unpause)
			}
		}

		if unpause {
			err := c.pipelineUnpauser.UnpausePipeline(
				p.TeamName,
				p.Name,
			)
			if err != nil {
				return concourse.OutResponse{}, err
			}
		}
	}
	c.logger.Debugf("Setting pipelines complete\n")

	c.logger.Debugf("Getting pipelines\n")

	teamName := input.Source.Teams[0].Name
	apiPipelines, err := c.apiClient.Pipelines(teamName)
	if err != nil {
		return concourse.OutResponse{}, err
	}

	c.logger.Debugf("Found pipelines: %+v\n", pipelines)

	pipelineVersions := make(map[string]string, len(pipelines))

	for _, pipeline := range apiPipelines {
		c.logger.Debugf("Getting pipeline: %s\n", pipeline.Name)
		_, _, version, err := c.apiClient.PipelineConfig(teamName, pipeline.Name)

		if err != nil {
			return concourse.OutResponse{}, err
		}

		pipelineVersions[pipeline.Name] = version
	}

	response := concourse.OutResponse{
		Version:  pipelineVersions,
		Metadata: []concourse.Metadata{},
	}

	return response, nil
}
