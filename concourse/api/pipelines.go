package api

import (
    "fmt"

    "github.com/concourse/atc"
)

func (c Client) clientForTeam(teamName string) (ConcourseClient, error) {
    client := c.gcClients[teamName]
    if client == nil {
        return nil, fmt.Errorf("no client found for team: '%s'", teamName)
    }
    return client, nil
}

func (c Client) Pipelines(teamName string) ([]Pipeline, error) {
    gcClient, err := c.clientForTeam(teamName)
    if err != nil {
        return nil, c.wrapErr(err)
    }

    atcPipelines, err := gcClient.ListPipelines()
    if err != nil {
        return nil, c.wrapErr(err)
    }

    return pipelinesFromATCPipelines(atcPipelines), nil
}

func (c Client) PipelineConfig(teamName string, pipelineName string) (atc.Config, string, string, error) {
    gcClient, err := c.clientForTeam(teamName)
    if err != nil {
        return atc.Config{}, "", "", c.wrapErr(err)
    }

    atcConfig, atcRawConfig, configVersion, exists, err :=
        gcClient.PipelineConfig(pipelineName)
    if err != nil {
        return atc.Config{}, "", "", c.wrapErr(err)
    }

    if !exists {
        err := fmt.Errorf("Pipeline not found: %s", pipelineName)
        return atc.Config{}, "", "", c.wrapErr(err)
    }

    return atcConfig, atcRawConfig.String(), configVersion, nil
}

func (c Client) SetPipelineConfig(
    teamName string,
    pipelineName string,
    configVersion string,
    passedConfig atc.Config,
) error {
    gcClient, err := c.clientForTeam(teamName)
    if err != nil {
        return c.wrapErr(err)
    }

    created, updated, _, err := gcClient.CreateOrUpdatePipelineConfig(
        pipelineName,
        configVersion,
        passedConfig,
    )
    if err != nil {
        return c.wrapErr(err)
    }

    if !created && !updated {
        err := fmt.Errorf("Pipeline neither created nor updated: %s", pipelineName)
        return c.wrapErr(err)
    }

    return nil
}

func (c Client) UnpausePipeline(
    teamName string,
    pipelineName string,
) error {
    gcClient, err := c.clientForTeam(teamName)
    if err != nil {
        return c.wrapErr(err)
    }

    unpaused, err := gcClient.UnpausePipeline(pipelineName)
    if err != nil {
        return c.wrapErr(err)
    }

    if !unpaused {
        err := fmt.Errorf("Pipeline not unpaused: %s", pipelineName)
        return c.wrapErr(err)
    }

    return nil
}

func (c Client) DeletePipeline(teamName string, pipelineName string) error {
    exists, err := c.gcClients[teamName].DeletePipeline(pipelineName)
    if err != nil {
        return c.wrapErr(err)
    }

    if !exists {
        err := fmt.Errorf("Pipeline not found: %s", pipelineName)
        return c.wrapErr(err)
    }

    return nil
}
