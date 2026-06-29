package worker

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/document/internal/service"
	"github.com/hibiken/asynq"
)

type Client struct {
	client *asynq.Client
}

func NewClient(redisAddr string) *Client {
	return &Client{
		client: asynq.NewClient(asynq.RedisClientOpt{Addr: redisAddr}),
	}
}

func (c *Client) Close() error {
	return c.client.Close()
}

// EnqueueReportJob implements service.TaskEnqueuer.
func (c *Client) EnqueueReportJob(ctx context.Context, jobType service.JobType, jobID, attemptID, requestID, userID string) (string, error) {
	taskType, err := TaskTypeForJobType(jobType)
	if err != nil {
		return "", err
	}
	data, err := json.Marshal(ReportJobPayload{
		RequestID: requestID,
		JobType:   string(jobType),
		JobID:     jobID,
		AttemptID: attemptID,
		UserID:    userID,
	})
	if err != nil {
		return "", fmt.Errorf("marshal report job payload: %w", err)
	}
	task := asynq.NewTask(taskType, data, asynq.Queue("document"))
	info, err := c.client.EnqueueContext(ctx, task)
	if err != nil {
		return "", fmt.Errorf("enqueue report job: %w", err)
	}
	return info.ID, nil
}
