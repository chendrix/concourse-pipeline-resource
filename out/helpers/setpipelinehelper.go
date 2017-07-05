package helpers

import (
	"io/ioutil"

	"github.com/concourse/atc"
	"github.com/concourse/fly/template"
	"github.com/mitchellh/mapstructure"
	"gopkg.in/yaml.v2"
)

//go:generate counterfeiter . Client
type Client interface {
	DeletePipeline(teamName string, pipelineName string) error
	PipelineConfig(teamName string, pipelineName string) (config atc.Config, rawConfig string, version string, err error)
	SetPipelineConfig(teamName string, pipelineName string, configVersion string, passedConfig atc.Config) error
	UnpausePipeline(teamName string, pipelineName string) error
}

//go:generate counterfeiter . ConfigDiffer
type ConfigDiffer interface {
	Diff(existingConfig atc.Config, newConfig atc.Config) error
}

type PipelineSetter struct {
	client       Client
	configDiffer ConfigDiffer
}

func NewPipelineSetter(client Client, configDiffer ConfigDiffer) *PipelineSetter {
	return &PipelineSetter{
		client:       client,
		configDiffer: configDiffer,
	}
}

func (p PipelineSetter) SetPipeline(
	teamName string,
	pipelineName string,
	configPath string,
	templateVariables template.Variables,
	templateVariablesFiles []string,
) error {
	newConfig, err := p.newConfig(
		configPath,
		templateVariablesFiles,
		templateVariables,
	)
	if err != nil {
		return err
	}

	existingConfig, _, existingConfigVersion, err :=
		p.client.PipelineConfig(teamName, pipelineName)
	if err != nil {
		return err
	}

	p.configDiffer.Diff(existingConfig, newConfig)

	err = p.client.SetPipelineConfig(
		teamName,
		pipelineName,
		existingConfigVersion,
		newConfig,
	)
	if err != nil {
		return err
	}

	return nil
}

func (p PipelineSetter) newConfig(
	configPath string,
	templateVariablesFiles []string,
	templateVariables template.Variables,
) (atc.Config, error) {
	configFile, err := ioutil.ReadFile(configPath)
	if err != nil {
		return atc.Config{}, err
	}

	var resultVars template.Variables

	for _, path := range templateVariablesFiles {
		fileVars, templateErr := template.LoadVariablesFromFile(string(path))
		if templateErr != nil {
			return atc.Config{}, templateErr
		}

		resultVars = resultVars.Merge(fileVars)
	}

	resultVars = resultVars.Merge(templateVariables)

	configFile, err = template.Evaluate(configFile, resultVars)
	if err != nil {
		return atc.Config{}, err
	}

	var configStructure interface{}
	err = yaml.Unmarshal(configFile, &configStructure)
	if err != nil {
		return atc.Config{}, err
	}

	var newConfig atc.Config
	msConfig := &mapstructure.DecoderConfig{
		Result:           &newConfig,
		WeaklyTypedInput: true,
		DecodeHook: mapstructure.ComposeDecodeHookFunc(
			atc.SanitizeDecodeHook,
			atc.VersionConfigDecodeHook,
		),
	}

	decoder, err := mapstructure.NewDecoder(msConfig)
	if err != nil {
		return atc.Config{}, err
	}

	if err := decoder.Decode(configStructure); err != nil {
		return atc.Config{}, err
	}

	return newConfig, nil
}
