package sqs

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"log"
	"io"
	"net/http"
	"net/url"
	"time"
)

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
	BasicResponse
}

type RecvMessageResponse struct {
	MessageId     string `xml:"ReceiveMessageResult>Message>MessageId"`
	MessageMD5    string `xml:"ReceiveMessageResult>Message>MD5OfBody"`
	MessageBody   string `xml:"ReceiveMessageResult>Message>Body"`
	ReceiptHandle string `xml:"ReceiveMessageResult>Message>ReceiptHandle"`
	BasicResponse
}

type QueueURLResponse struct {
	QueueURL string `xml:"QueueUrl"`
	BasicResponse
}

type QueueListResponse struct {
	QueueURLs []string `xml:"ListQueuesResult>QueueUrl"`
	BasicResponse
}

type BasicResponse struct {
	RequestId string `xml:"ResponseMetadata>RequestId"`
}

type SQSRequest struct {
	RegionId     string
	UUID         string
	QueueName    string
	AWSAccessKey string
	AWSSecret    string
}

func (s *SQSRequest) makeSQSQueueRequest(params map[string]string) (io.ReadCloser, error) {
	return s.makeSQSRequest(params, true)
}

func (s *SQSRequest) makeSQSAdminRequest(params map[string]string) (io.ReadCloser, error) {
	return s.makeSQSRequest(params, false)
}

func (s *SQSRequest) makeSQSRequest(params map[string]string, isQueueRequest bool) (io.ReadCloser, error) {
	sqsURI := s.generateSQSQueueURI()
	if !isQueueRequest {
		sqsURI = s.generateSQSURI()
	}

	method := "POST"

	var uv = url.Values{}
	uv.Set("AWSAccessKeyId", s.AWSAccessKey)
	uv.Set("SignatureVersion", "2")
	uv.Set("SignatureMethod", "HmacSHA256")
	uv.Set("Version", "2012-11-05")
	uv.Set("Timestamp", time.Now().Format(time.RFC3339))

	for key, value := range params {
		uv.Set(key, value)
	}

	uv.Set("Signature", GenerateSignature(sqsURI, method, s.AWSSecret, uv))

	r, err := http.NewRequest(method, sqsURI, bytes.NewBufferString(uv.Encode()))
	if err != nil {
		return nil, err
	}

	r.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{}

	resp, err := client.Do(r)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == http.StatusOK {
		return resp.Body, nil
	}

	return resp.Body, errors.New(resp.Status)
}

func (s *SQSRequest) generateSQSQueueURI() string {
	var u = url.URL{
		Scheme: "https",
		Host:   fmt.Sprintf("sqs.%s.amazonaws.com", s.RegionId),
		Path:   fmt.Sprintf("/%s/%s/", s.UUID, s.QueueName),
	}

	return u.String()
}

func (s *SQSRequest) generateSQSURI() string {
	urlStr := s.generateSQSQueueURI()

	u, _ := url.Parse(urlStr)
	u.Path = "/"

	return u.String()
}

func (s *SQSRequest) SendSQSMessage(message string) (*SendMessageResponse, error) {
	message = url.QueryEscape(message)

	params := map[string]string{
		"Action":      "SendMessage",
		"MessageBody": message,
	}

	reader, err := s.makeSQSQueueRequest(params)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	smr := new(SendMessageResponse)
	err = xml.NewDecoder(reader).Decode(smr)
	if err != nil {
		return nil, err
	}

	return smr, nil
}

func (s *SQSRequest) ReceiveSQSMessage() (*RecvMessageResponse, error) {
	params := map[string]string{
		"Action": "ReceiveMessage",
	}

	reader, err := s.makeSQSQueueRequest(params)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	rmr := new(RecvMessageResponse)
	err = xml.NewDecoder(reader).Decode(rmr)
	if err != nil {
		return nil, err
	}

	if rmr.MessageBody == "" && rmr.MessageMD5 == "" {
		return nil, errors.New("No message to dequeue.")
	}

	rmr.MessageBody, err = url.QueryUnescape(rmr.MessageBody)
	if err != nil {
		return nil, err
	}

	return rmr, nil
}

func (s *SQSRequest) DeleteSQSMessage(handle string) (*BasicResponse, error) {
	params := map[string]string{
		"Action":        "DeleteMessage",
		"ReceiptHandle": handle,
	}

	reader, err := s.makeSQSQueueRequest(params)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	bmr := new(BasicResponse)
	if err = xml.NewDecoder(reader).Decode(bmr); err != nil {
		return nil, err
	}

	return bmr, nil
}

func (s *SQSRequest) QueueURL() (*QueueURLResponse, error) {
	params := map[string]string{
		"Action": "GetQueueUrl",
		"QueueName": s.QueueName,
	}

	reader, err := s.makeSQSAdminRequest(params)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	qur := new(QueueURLResponse)
	if err = xml.NewDecoder(reader).Decode(qur); err != nil {
		return nil, err
	}

	return qur, nil
}

func (s *SQSRequest) ListQueues(prefix string) (*QueueListResponse, error) {
	params := map[string]string{
		"Action":          "ListQueues",
		"QueueNamePrefix": prefix,
	}

	reader, err := s.makeSQSAdminRequest(params)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	qr := new(QueueListResponse)
	if err = xml.NewDecoder(reader).Decode(qr); err != nil {
		return nil, err
	}

	return qr, nil
}

func (s *SQSRequest) CreateQueue(queueName string, options map[string]string) (*QueueURLResponse, error) {
	params := map[string]string{
		"Action": "CreateQueue",
		"QueueName": queueName,
	}

	count := 1
	for name, value := range options {
		params[fmt.Sprintf("Attribute.%d.Name", count)] = name
		params[fmt.Sprintf("Attribute.%d.Value", count)] = value
		count++
	}

	reader, err := s.makeSQSAdminRequest(params)
	if err != nil {
		er := new(ErrorResponse)
		xml.NewDecoder(reader).Decode(er)
		log.Println(er)
		return nil, err
	}

	qur := new(QueueURLResponse)
	if err = xml.NewDecoder(reader).Decode(qur); err != nil {
		return nil, err
	}

	return qur, nil
}
