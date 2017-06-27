package helpers

import (
    "strings"
)

type PipelineDeleter struct {
    client Client
}

func NewPipelineDeleter(client Client) *PipelineDeleter {
    return &PipelineDeleter{
        client: client,
    }
}

func (p PipelineDeleter) DeletePipeline(
    teamName string,
    pipelineName string,
) error {
    err := p.client.DeletePipeline(teamName, pipelineName)

    if err != nil && !strings.Contains(err.Error(), "Pipeline not found") {
        return err
    }

    return nil
}
