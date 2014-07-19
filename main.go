package main

import (
	"bytes"
	"encoding/gob"
	"flag"
	"fmt"
	"github.com/neurodrone/aws-sqs/sqs"
	"log"
	"os"
)

var (
	awsAccessKey = flag.String("accesskey", "", "AWS Access Key")
	awsSecret    = flag.String("secret", "", "AWS Secret Key")

	regionId  = flag.String("region", "", "AWS Region ID")
	uuid      = flag.String("uuid", "", "AWS Unique ID")
	queueName = flag.String("queue", "", "AWS Queue Name")
)

type SampleMessageStruct struct {
	SomeStr string
	SomeInt int
}

func main() {
	flag.Parse()

	e := validateInputs()
	if e.hasErrors() {
		e.printErrors(os.Stderr)
		log.Panicf("Aborting.")
	}

	sqsReq := &sqs.SQSRequest{
		*regionId,
		*uuid,
		*queueName,
		*awsAccessKey,
		*awsSecret,
	}

	qur, err := sqsReq.CreateQueue("stats-test3", map[string]string{
		"VisibilityTimeout": "40",
	})
	if err != nil {
		log.Panicf("Unable to create queue: %s", err)
	}
	log.Println("Successfully created queue at:", qur.QueueURL)

	qur, err = sqsReq.QueueURL()
	if err != nil {
		log.Panicf("Unable to fetch queue url: %s", err)
	}
	log.Println(qur.QueueURL)

	qlr, err := sqsReq.ListQueues("stat")
	if err != nil {
		log.Panicf("Unable to list queues: %s", err)
	}
	log.Println(qlr.QueueURLs)

	var buf bytes.Buffer
	var message string
	var m *SampleMessageStruct

	m = &SampleMessageStruct{"strVal", 7}
	gob.NewEncoder(&buf).Encode(m)

	_, err = sqsReq.SendSQSMessage(buf.Bytes())
	if err != nil {
		log.Panicf("Unable to enqueue message: %s", err)
	}
	log.Println("Message sent.")

	msgResp, err := sqsReq.ReceiveSQSMessage()
	if err != nil {
		log.Panicf("Unable to receive message: %s", err)
	}

	log.Println(msgResp.MessageId, "received.")
	message = msgResp.MessageBody

	m = new(SampleMessageStruct)
	gob.NewDecoder(bytes.NewBufferString(message)).Decode(m)
	log.Println(m)

	_, err = sqsReq.DeleteSQSMessage(msgResp.ReceiptHandle)
	if err != nil {
		log.Panicf("Unable to delete message: %s", msgResp.MessageId)
	}

	log.Println("Successfully received and deleted.")
}

func validateInputs() Errors {
	errs := make(Errors, 0)

	flag.VisitAll(func(fl *flag.Flag) {
		if fl.Value.String() == fl.DefValue {
			errs = append(errs, fmt.Errorf("%s needs to be set.", fl.Usage))
		}
	})

	return errs
}
