package check

import (
	"crypto/md5"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"

	"github.com/concourse/concourse-pipeline-resource/concourse"
	"github.com/concourse/concourse-pipeline-resource/fly"
	"github.com/concourse/concourse-pipeline-resource/logger"
)

type Command struct {
	logger      logger.Logger
	logFilePath string
	flyCommand  fly.Command
}

func NewCommand(
	logger logger.Logger,
	logFilePath string,
	flyCommand fly.Command,
) *Command {
	return &Command{
		logger:      logger,
		logFilePath: logFilePath,
		flyCommand:  flyCommand,
	}
}

func (c *Command) Run(input concourse.CheckRequest) (concourse.CheckResponse, error) {
	logDir := filepath.Dir(c.logFilePath)
	existingLogFiles, err := filepath.Glob(filepath.Join(logDir, "concourse-pipeline-resource-check.log*"))
	if err != nil {
		// This is untested because the only error returned by filepath.Glob is a
		// malformed glob, and this glob is hard-coded to be correct.
		return nil, err
	}

	for _, f := range existingLogFiles {
		if filepath.Base(f) != filepath.Base(c.logFilePath) {
			c.logger.Debugf("Removing existing log file: %s\n", f)
			err := os.Remove(f)
			if err != nil {
				// This is untested because it is too hard to force os.Remove to return
				// an error.
				return nil, err
			}
		}
	}

	c.logger.Debugf("Received input: %+v\n", input)

	insecure := false
	if input.Source.Insecure != "" {
		var err error
		insecure, err = strconv.ParseBool(input.Source.Insecure)
		if err != nil {
			return concourse.CheckResponse{}, err
		}
	}

	teams := make(map[string]concourse.Team)

	for _, team := range input.Source.Teams {
		teams[team.Name] = team
	}

	pipelineVersions := make(map[string]string)

	for teamName, team := range teams {
		c.logger.Debugf("Performing login\n")
		_, err := c.flyCommand.Login(
			input.Source.Target,
			teamName,
			team.Username,
			team.Password,
			insecure,
		)
		if err != nil {
			return concourse.CheckResponse{}, err
		}

		c.logger.Debugf("Login successful\n")

		var pipelines []string
		if team.Pipelines == nil {
			pipelines, err = c.flyCommand.Pipelines()
			if err != nil {
				return concourse.CheckResponse{}, err
			}
			c.logger.Debugf("Found pipelines (%s): %+v\n", teamName, pipelines)
		} else {
			pipelines = team.Pipelines
			c.logger.Debugf("Using configured pipelines (%s): %+v\n", teamName, pipelines)
		}

		for _, pipelineName := range pipelines {
			c.logger.Debugf("Getting pipeline: %s\n", pipelineName)
			outBytes, err := c.flyCommand.GetPipeline(pipelineName)
			if err != nil {
				match, _ := regexp.MatchString("pipeline not found", err.Error())
				if match {
					c.logger.Debugf("Pipeline %s not already present, skipping.\n", pipelineName)
					continue
				} else {
					return concourse.CheckResponse{}, err
				}
			}

			version := fmt.Sprintf(
				"%x",
				md5.Sum(outBytes),
			)
			pipelineVersions[fmt.Sprintf("%s:%s", team.Username, pipelineName)] = version
		}
	}

	out := concourse.CheckResponse{
		pipelineVersions,
	}

	c.logger.Debugf("Returning output: %+v\n", out)

	return out, nil
}
