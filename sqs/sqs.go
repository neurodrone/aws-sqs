package sqs

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
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

type SQSRequest struct {
	RegionId     string
	UUID         string
	QueueName    string
	AWSAccessKey string
	AWSSecret    string
}

func (s *SQSRequest) makeSQSRequest(params map[string]string) (io.Reader, error) {
	sqsURI := s.generateSQSURI()
	method := "POST"

	var uv = url.Values{}
	uv.Set("AWSAccessKey", s.AWSAccessKey)
	uv.Set("SignatureVersion", "2")
	uv.Set("SignatureMethod", "HmacSHA256")
	uv.Set("Version", "2012-11-05")

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

func (s *SQSRequest) generateSQSURI() string {
	var u = url.URL{
		Scheme: "https",
		Host:   fmt.Sprintf("sqs.%s.amazonaws.com", s.RegionId),
		Path:   fmt.Sprintf("/%s/%s", s.UUID, s.QueueName),
	}

	return u.String()
}

func (s *SQSRequest) SendSQSMessage(message string) (*SendMessageResponse, error) {
	params := map[string]string{
		"Action":      "SendMessage",
		"MessageBody": message,
	}

	reader, err := s.makeSQSRequest(params)
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

func (s *SQSRequest) ReceiveSQSMessage() (*RecvMessageResponse, error) {
	params := map[string]string{
		"Action": "ReceiveMessage",
	}

	reader, err := s.makeSQSRequest(params)
	if err != nil {
		return nil, err
	}

	rmr := new(RecvMessageResponse)
	err = xml.NewDecoder(reader).Decode(rmr)
	if err != nil {
		return nil, err
	}

	if rmr.MessageBody == "" && rmr.MessageMD5 == "" {
		return nil, errors.New("No message to dequeue.")
	}

	return rmr, nil
}

func (s *SQSRequest) DeleteSQSMessage(handle string) (*BasicMessageResponse, error) {
	params := map[string]string{
		"Action":        "DeleteMessage",
		"ReceiptHandle": handle,
	}

	reader, err := s.makeSQSRequest(params)
	if err != nil {
		return nil, err
	}

	bmr := new(BasicMessageResponse)
	if err = xml.NewDecoder(reader).Decode(bmr); err != nil {
		return nil, err
	}

	return bmr, nil

}