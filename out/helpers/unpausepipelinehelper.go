package helpers

type PipelineUnpauser struct {
	client Client
}

func NewPipelineUnpauser(client Client) *PipelineUnpauser {
	return &PipelineUnpauser{
		client: client,
	}
}

func (p PipelineUnpauser) UnpausePipeline(
	teamName string,
	pipelineName string,
) error {
	err := p.client.UnpausePipeline(teamName, pipelineName)
	if err != nil {
		return err
	}

	return nil
}
