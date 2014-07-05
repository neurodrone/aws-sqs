package main

import (
	"flag"
	"fmt"
	"log"
	"github.com/neurodrone/aws-sqs/sqs"
)

var (
	awsAccessKey = flag.String("accesskey", "", "AWS Access Key")
	awsSecret    = flag.String("secret", "", "AWS Secret Key")

	regionId  = flag.String("region", "", "AWS Region ID")
	uuid      = flag.String("uuid", "", "AWS Unique ID")
	queueName = flag.String("queue", "", "AWS Queue Name")
)

type Errors []error

func main() {
	flag.Parse()

	errs := validateInputs()
	if len(errs) > 0 {
		log.Fatalf("Aborting.")
	}

	sqsReq := &sqs.SQSRequest{
		*regionId,
		*uuid,
		*queueName,
		*awsAccessKey,
		*awsSecret,
	}
/*
	message := "This is a test message"
	_, err := sqsReq.SendSQSMessage(message)
	if err != nil {
		log.Fatalf("Unable to enqueue message: %s", err)
	}

	log.Println("Message sent.")
*/
	msgResp, err := sqsReq.ReceiveSQSMessage()
	if err != nil {
		log.Fatalf("Unable to receive message: %s", err)
	}

	log.Println(msgResp.MessageId, "received.")
	log.Println(msgResp.MessageBody)

	err = sqsReq.DeleteSQSMessage(msgResp.ReceiptHandle)
	if err != nil {
		log.Fatalf("Unable to delete message: %s", msgResp.MessageId)
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

	if len(errs) > 0 {
		log.Println("Encountered errors:")
		for _, err := range errs {
			log.Println(err)
		}
	}

	return errs
}
