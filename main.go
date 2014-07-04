package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/xml"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
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
	return fmt.Sprintf("\tType: %s\n\tCode: %s\n\tMessage: %s", er.Type, er.Code, er.Message)
}

func main() {
	flag.Parse()

	errs := validateInputs()
	if len(errs) > 0 {
		log.Fatalf("Aborting.")
	}

	message := "This is a test message"
	err := sendSQSMessage(message)
	if err != nil {
		log.Fatalf("Unable to send message: %s", err)
	}
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

func sendSQSMessage(message string) error {
	sqsURI := generateSQSURI(*regionId, *uuid, *queueName)
	method := "POST"

	var uv = url.Values{}
	uv.Set("Action", "SendMessage")
	uv.Set("MessageBody", message)
	uv.Set("AWSAccessKey", *awsAccessKey)
	uv.Set("SignatureVersion", "2")
	uv.Set("SignatureMethod", "HmacSHA256")
	uv.Set("Version", "2011-10-01")

	uv.Set("Signature", getSignature(sqsURI, method, *awsSecret, uv))

	log.Println(uv.Encode())

	req, err := http.NewRequest(method, sqsURI, bytes.NewBufferString(uv.Encode()))
	if err != nil {
		return err
	}

	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errResponse, err := getErrorResponse(resp.Body)
		if err != nil {
			log.Println("Failed getting error response:", err)
		} else {
			log.Println(errResponse)
		}

		return nil
	}

	buf, _ := ioutil.ReadAll(resp.Body)
	log.Println(string(buf))
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
	fmt.Fprintf(h, "%s", sigPayload)

	b64 := base64.StdEncoding
	sig := make([]byte, b64.EncodedLen(h.Size()))
	b64.Encode(sig, h.Sum(nil))

	return string(sig)
}

func getErrorResponse(r io.Reader) (*ErrorResponse, error) {
	er := new(ErrorResponse)

	err := xml.NewDecoder(r).Decode(er)
	if err != nil {
		return nil, err
	}

	return er, nil
}

func generateSQSURI(region, uuid, queueName string) string {
	var u = url.URL{
		Scheme: "https",
		Host:   fmt.Sprintf("sqs.%s.amazonaws.com", region),
		Path:   fmt.Sprintf("/%s/%s", uuid, queueName),
	}

	return u.String()
}
