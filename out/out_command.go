package out

import (
	"crypto/md5"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/concourse/fly/template"
	"github.com/robdimsdale/concourse-pipeline-resource/concourse"
	"github.com/robdimsdale/concourse-pipeline-resource/concourse/api"
	"github.com/robdimsdale/concourse-pipeline-resource/logger"
	"github.com/robdimsdale/concourse-pipeline-resource/out/helpers"
	"github.com/robdimsdale/concourse-pipeline-resource/pipelinerunner"
)

const (
	apiPrefix = "/api/v1"
)

type OutCommand struct {
	logger         logger.Logger
	binaryVersion  string
	apiClient      api.Client
	sourcesDir     string
	pipelineSetter helpers.PipelineSetter
}

func NewOutCommand(
	binaryVersion string,
	logger logger.Logger,
	pipelineSetter helpers.PipelineSetter,
	apiClient api.Client,
	sourcesDir string,
) *OutCommand {
	return &OutCommand{
		logger:         logger,
		binaryVersion:  binaryVersion,
		pipelineSetter: pipelineSetter,
		apiClient:      apiClient,
		sourcesDir:     sourcesDir,
	}
}

func (c *OutCommand) Run(input concourse.OutRequest) (concourse.OutResponse, error) {
	c.logger.Debugf("Received input: %+v\n", input)

	pipelines := input.Params.Pipelines

	c.logger.Debugf("Input pipelines: %+v\n", pipelines)

	c.logger.Debugf("Setting pipelines\n")
	for _, p := range pipelines {
		configFilepath := filepath.Join(c.sourcesDir, p.ConfigFile)

		var varsFilepaths []string
		for _, v := range p.VarsFiles {
			varFilepath := filepath.Join(c.sourcesDir, v)
			varsFilepaths = append(varsFilepaths, varFilepath)
		}

		var templateVariables template.Variables
		err := c.pipelineSetter.SetPipeline(
			p.Name,
			configFilepath,
			templateVariables,
			varsFilepaths,
		)
		if err != nil {
			return concourse.OutResponse{}, err
		}
	}
	c.logger.Debugf("Setting pipelines complete\n")

	c.logger.Debugf("Getting pipelines\n")
	apiPipelines, err := c.apiClient.Pipelines()
	if err != nil {
		return concourse.OutResponse{}, err
	}
	c.logger.Debugf("Getting pipelines complete\n")

	c.logger.Debugf("Found pipelines: %+v\n", apiPipelines)

	gpFunc := func(index int, pipeline api.Pipeline) (string, error) {
		c.logger.Debugf("Getting pipeline config: %s\n", pipeline.Name)
		_, config, _, err := c.apiClient.PipelineConfig(pipeline.Name)

		if err != nil {
			return "", err
		}

		return config, nil
	}

	pipelinesContents, err := pipelinerunner.RunForAllPipelines(gpFunc, apiPipelines, c.logger)
	if err != nil {
		return concourse.OutResponse{}, err
	}

	allContent := strings.Join(pipelinesContents, "")

	pipelinesChecksumString := fmt.Sprintf(
		"%x",
		md5.Sum([]byte(allContent)),
	)
	c.logger.Debugf("pipeline content checksum: %s\n", pipelinesChecksumString)

	metadata := []concourse.Metadata{}

	response := concourse.OutResponse{
		Version: concourse.Version{
			PipelinesChecksum: pipelinesChecksumString,
		},
		Metadata: metadata,
	}

	return response, nil
}
