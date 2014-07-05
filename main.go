package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/xml"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
)

var (
	awsAccessKey = flag.String("accesskey", "", "AWS Access Key")
	awsSecret    = flag.String("secret", "", "AWS Secret Key")

	regionId  = flag.String("region", "", "AWS Region ID")
	uuid      = flag.String("uuid", "", "AWS Unique ID")
	queueName = flag.String("queue", "", "AWS Queue Name")
)

type Errors []error

type ErrorResponse struct {
	Type    string `xml:"Error>Type"`
	Code    string `xml:"Error>Code"`
	Message string `xml:"Error>Message"`
}

func (er *ErrorResponse) String() string {
	return fmt.Sprintf("Type: %s, Code: %s, Message: %s", er.Type, er.Code, er.Message)
}

type SendMessageResponse struct {
	MessageId  string `xml:"SendMessageResult>MessageId"`
	MessageMD5 string `xml:"SendMessageResult>MD5OfMessageBody"`
	BasicMessageResponse
}

type RecvMessageResponse struct {
	MessageId     string `xml:"ReceiveMessageResult>Message>MessageId"`
	MessageMD5    string `xml:"ReceiveMessageResult>Message>MD5OfBody"`
	MessageBody   string `xml:"ReceiveMessageResult>Message>Body"`
	ReceiptHandle string `xml:"ReceiveMessageResult>Message>ReceiptHandle"`
	BasicMessageResponse
}

type BasicMessageResponse struct {
	RequestId string `xml:"ResponseMetadata>RequestId"`
}

func main() {
	flag.Parse()

	errs := validateInputs()
	if len(errs) > 0 {
		log.Fatalf("Aborting.")
	}

/*
	message := "This is a test message"
	_, err := sendSQSMessage(message)
	if err != nil {
		log.Fatalf("Unable to enqueue message: %s", err)
	}

	log.Println("Message sent.")
*/
	msgResp, err := receiveSQSMessage()
	if err != nil {
		log.Fatalf("Unable to receive message: %s", err)
	}

	log.Println(msgResp.MessageId, "received.")
	log.Println(msgResp.MessageBody)

	err = deleteSQSMessage(msgResp.ReceiptHandle)
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

func makeSQSRequest(params map[string]string) (io.Reader, error) {
	sqsURI := generateSQSURI(*regionId, *uuid, *queueName)
	method := "POST"

	var uv = url.Values{}
	uv.Set("AWSAccessKey", *awsAccessKey)
	uv.Set("SignatureVersion", "2")
	uv.Set("SignatureMethod", "HmacSHA256")
	uv.Set("Version", "2012-11-05")

	for key, value := range params {
		uv.Set(key, value)
	}

	uv.Set("Signature", getSignature(sqsURI, method, *awsSecret, uv))

	req, err := http.NewRequest(method, sqsURI, bytes.NewBufferString(uv.Encode()))
	if err != nil {
		return nil, err
	}

	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == http.StatusOK {
		return resp.Body, nil
	}

	return resp.Body, errors.New(resp.Status)
}

func sendSQSMessage(message string) (*SendMessageResponse, error) {
	params := map[string]string{
		"Action":      "SendMessage",
		"MessageBody": message,
	}

	reader, err := makeSQSRequest(params)
	if err != nil {
		return nil, err
	}

	smr := new(SendMessageResponse)
	err = xml.NewDecoder(reader).Decode(smr)
	if err != nil {
		return nil, err
	}

	return smr, nil
}

func receiveSQSMessage() (*RecvMessageResponse, error) {
	params := map[string]string{
		"Action": "ReceiveMessage",
	}

	reader, err := makeSQSRequest(params)
	if err != nil {
		return nil, err
	}

	rmr := new(RecvMessageResponse)
	err = xml.NewDecoder(reader).Decode(rmr)
	if err != nil {
		return nil, err
	}

	if rmr.MessageBody == "" && rmr.MessageMD5 == "" {
		return nil, fmt.Errorf("No message to dequeue.")
	}

	return rmr, nil
}

func deleteSQSMessage(handle string) error {
	params := map[string]string{
		"Action":        "DeleteMessage",
		"ReceiptHandle": handle,
	}

	reader, err := makeSQSRequest(params)
	if err != nil {
		return err
	}

	bmr := new(BasicMessageResponse)
	err = xml.NewDecoder(reader).Decode(bmr)
	if err != nil {
		return err
	}

	return nil
}

func getSignature(sqsURI, method, secret string, uv url.Values) string {
	u, err := url.Parse(sqsURI)
	if err != nil {
		return ""
	}

	sigPayload := strings.Join([]string{
		method,
		u.Host,
		u.Path,
		uv.Encode(),
	}, "\n")

	h := hmac.New(sha256.New, []byte(secret))
	fmt.Fprint(h, sigPayload)

	b64 := base64.StdEncoding
	sig := make([]byte, b64.EncodedLen(h.Size()))
	b64.Encode(sig, h.Sum(nil))

	return string(sig)
}

func generateSQSURI(region, uuid, queueName string) string {
	var u = url.URL{
		Scheme: "https",
		Host:   fmt.Sprintf("sqs.%s.amazonaws.com", region),
		Path:   fmt.Sprintf("/%s/%s", uuid, queueName),
	}

	return u.String()
}
