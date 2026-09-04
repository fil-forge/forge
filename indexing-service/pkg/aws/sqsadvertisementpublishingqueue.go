package aws

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/fil-forge/go-ipni-tools/pkg/queue"
	"github.com/google/uuid"
	"github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/codec/dagjson"
	"github.com/ipld/go-ipld-prime/node/bindnode"
	ipldschema "github.com/ipld/go-ipld-prime/schema"
	"github.com/ipni/go-libipni/ingest/schema"
)

type advMarshaller schema.Advertisement

func (a advMarshaller) MarshalJSON() ([]byte, error) {
	return encodeJSON((*schema.Advertisement)(&a), schema.AdvertisementPrototype.Type())
}

func (a *advMarshaller) UnmarshalJSON(data []byte) error {
	var adv schema.Advertisement
	if err := decodeJSON(data, &adv, schema.AdvertisementPrototype.Type()); err != nil {
		return err
	}
	*a = advMarshaller(adv)
	return nil
}

func encodeJSON(val any, typ ipldschema.Type, opts ...bindnode.Option) (bytes []byte, err error) {
	defer func() {
		if r := recover(); r != nil {
			if asStr, ok := r.(string); ok {
				err = errors.New(asStr)
			} else if asErr, ok := r.(error); ok {
				err = asErr
			} else {
				err = fmt.Errorf("unknown panic encoding JSON: %+v", r)
			}
		}
	}()
	bytes, err = ipld.Marshal(dagjson.Encode, val, typ, opts...)
	return
}

func decodeJSON(b []byte, bind any, typ ipldschema.Type, opts ...bindnode.Option) error {
	_, err := ipld.Unmarshal(b, dagjson.Decode, bind, typ, opts...)
	return err
}

// queueMessage is the struct that is serialized onto an SQS message queue in JSON
type queueMessage struct {
	JobID         uuid.UUID     `json:"JobID,omitempty"`
	Advertisement advMarshaller `json:"Message,omitempty"`
}

// SQSAdvertisementPublishingQueue implements a queue for publishing advertisements using SQS
type SQSAdvertisementPublishingQueue struct {
	queueID   string
	sqsClient *sqs.Client
	decoder   *SQSAdvertisementPublishingDecoder
}

// NewSQSAdvertisementPublishingQueue returns a new SQSAdvertisementPublishingQueue for the given aws config
func NewSQSAdvertisementPublishingQueue(cfg aws.Config, queueID string) *SQSAdvertisementPublishingQueue {
	return &SQSAdvertisementPublishingQueue{
		queueID:   queueID,
		sqsClient: sqs.NewFromConfig(cfg),
		decoder:   NewSQSAdvertisementPublishingDecoder(),
	}
}

// Queue sends a new advertisement publishing job to the SQS queue.
func (s *SQSAdvertisementPublishingQueue) Queue(ctx context.Context, adv schema.Advertisement) error {
	uuid := uuid.New()
	messageJSON, err := json.Marshal(queueMessage{
		JobID:         uuid,
		Advertisement: advMarshaller(adv),
	})
	if err != nil {
		return fmt.Errorf("serializing message json: %w", err)
	}
	message := &sqs.SendMessageInput{
		QueueUrl:       aws.String(s.queueID),
		MessageBody:    aws.String(string(messageJSON)),
		MessageGroupId: aws.String("default"),
	}
	_, err = s.sqsClient.SendMessage(ctx, message)
	if err != nil {
		return fmt.Errorf("enqueueing message: %w", err)
	}
	return nil
}

// Read reads up to maxJobs advertisements from the SQS queue.
func (s *SQSAdvertisementPublishingQueue) Read(ctx context.Context, maxJobs int) ([]queue.WithID[schema.Advertisement], error) {
	receiveOutput, err := s.sqsClient.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
		QueueUrl:            aws.String(s.queueID),
		MaxNumberOfMessages: int32(maxJobs),
		WaitTimeSeconds:     20, // enable long-polling
	})
	if err != nil {
		return nil, fmt.Errorf("failed to receive messages from SQS: %w", err)
	}

	if len(receiveOutput.Messages) == 0 {
		return []queue.WithID[schema.Advertisement]{}, nil
	}

	ads := make([]queue.WithID[schema.Advertisement], 0, len(receiveOutput.Messages))
	for _, msg := range receiveOutput.Messages {
		ad, err := s.decoder.DecodeMessage(ctx, aws.ToString(msg.ReceiptHandle), aws.ToString(msg.Body))
		if err != nil {
			return nil, fmt.Errorf("failed to decode message: %w", err)
		}

		ads = append(ads, ad)
	}

	return ads, nil
}

func (s *SQSAdvertisementPublishingQueue) Release(ctx context.Context, jobID string) error {
	_, err := s.sqsClient.ChangeMessageVisibility(ctx, &sqs.ChangeMessageVisibilityInput{
		QueueUrl:          aws.String(s.queueID),
		ReceiptHandle:     aws.String(jobID),
		VisibilityTimeout: 0,
	})

	return err
}

// Delete deletes a job message from the SQS publisher queue.
func (s *SQSAdvertisementPublishingQueue) Delete(ctx context.Context, jobID string) error {
	_, err := s.sqsClient.DeleteMessage(ctx, &sqs.DeleteMessageInput{
		QueueUrl:      aws.String(s.queueID),
		ReceiptHandle: aws.String(jobID),
	})
	return err
}

// SQSAdvertisementPublishingDecoder provides interfaces for working with advertisements received over SQS
type SQSAdvertisementPublishingDecoder struct {
}

// NewSQSDecoder returns a new decoder for the given AWS config
func NewSQSAdvertisementPublishingDecoder() *SQSAdvertisementPublishingDecoder {
	return &SQSAdvertisementPublishingDecoder{}
}

// DecodeMessage decodes a provider caching job from the SQS message body, reading the stored index from S3
func (s *SQSAdvertisementPublishingDecoder) DecodeMessage(ctx context.Context, receiptHandle string, messageBody string) (queue.WithID[schema.Advertisement], error) {
	var decodedMsg queueMessage
	err := json.Unmarshal([]byte(messageBody), &decodedMsg)
	if err != nil {
		return queue.WithID[schema.Advertisement]{}, fmt.Errorf("deserializing message: %w", err)
	}
	return queue.WithID[schema.Advertisement]{
		ID:  receiptHandle,
		Job: schema.Advertisement(decodedMsg.Advertisement),
	}, nil
}
