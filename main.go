package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"sync"
	"text/template"

	"os"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dynamodbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/ses"
	sestypes "github.com/aws/aws-sdk-go-v2/service/ses/types"
)

type Invocation struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Content     string `json:"content"`
	Plain       string `json:"plain"`
	Link        string `json:"link"`
}

type Subscriber struct {
	Email string
	Id    string
}

type SendEmailErrors struct {
	Messages []error
	sync.Mutex
}

// Find list items with the given confirmation status
func scanForSubscribers(svc *dynamodb.Client, confirm bool) (*dynamodb.ScanOutput, error) {
	table := os.Getenv("DB_TABLE_NAME")
	input := &dynamodb.ScanInput{
		ExpressionAttributeNames: map[string]string{
			"#C": "confirm",
		},
		ExpressionAttributeValues: map[string]dynamodbtypes.AttributeValue{
			":yes": &dynamodbtypes.AttributeValueMemberBOOL{Value: confirm},
		},
		FilterExpression:     aws.String("#C = :yes"),
		ProjectionExpression: aws.String("email, id"),
		TableName:            aws.String(table),
	}

	result, err := svc.Scan(context.Background(), input)
	if err != nil {
		log.Print("DynamoDB scan error: ", err.Error())
	}
	return result, err
}

// Update subscriber IDs
func updateIdsInDynamoDB(svc *dynamodb.Client, email string, id string, timestamp string, confirm bool) (*dynamodb.UpdateItemOutput, error) {
	table := os.Getenv("DB_TABLE_NAME")

	input := &dynamodb.UpdateItemInput{
		// Provide the key to use for finding the right item.
		Key: map[string]dynamodbtypes.AttributeValue{
			"email": &dynamodbtypes.AttributeValueMemberS{Value: email},
		},
		// Give the keys a shorthand to reference
		ExpressionAttributeNames: map[string]string{
			"#ID": "id",
			"#T":  "timestamp",
			"#C":  "confirm",
		},
		// Give the incoming values a shorthand to reference
		ExpressionAttributeValues: map[string]dynamodbtypes.AttributeValue{
			":idval":   &dynamodbtypes.AttributeValueMemberS{Value: id},
			":timeval": &dynamodbtypes.AttributeValueMemberS{Value: timestamp},
			":yes":     &dynamodbtypes.AttributeValueMemberBOOL{Value: true},
		},
		// Use the shorthand references to update these keys
		ConditionExpression: aws.String("#C = :yes"),
		UpdateExpression:    aws.String("SET #T = :timeval, #ID = :idval"),
		TableName:           aws.String(table),
	}

	result, err := svc.UpdateItem(context.Background(), input)
	if err != nil {
		log.Print("DynamoDB update error: ", err.Error())
	}
	return result, err
}

func buildEmail(event Invocation, emailAddress string, id string) *ses.SendEmailInput {
	var htmlBody bytes.Buffer
	templateData := struct {
		Title       string
		Description string
		Content     string
	}{
		Title:       event.Title,
		Description: event.Description,
		Content:     event.Content,
	}

	unsubLink := os.Getenv("UNSUBSCRIBE_LINK")

	// Build the rich text email
	t, terr := template.ParseFiles("template.html")
	if terr != nil {
		log.Fatalf("could not get email template: %s", terr)
	}
	t.Execute(&htmlBody, templateData)
	rich := htmlBody.String() + "\n\n<hr>\n\n<p style=\"font-size: 0.9em;\">You're subscribed to <a href=\"" + os.Getenv("WEBSITE") + "\">" + os.Getenv("TITLE") + "</a>. Click here to <a href=\"" + unsubLink + "?email=" + emailAddress + "&id=" + id + "\">unsubscribe</a>.</p>"

	// Build plain text format
	plain := event.Title + "\n\n" + event.Description + "\n\n---\n\nYou can view this email as HTML, or read this on my site: \n" + event.Link + "\n\n---\n\n" + event.Plain + "\n\n---\n\nYou've subscribed at " + os.Getenv("WEBSITE") + ". To unsubscribe, use: " + unsubLink + "?email=" + emailAddress + "&id=" + id

	// Build the "from" value
	source := fmt.Sprintf("\"%s\" <%s>", os.Getenv("SENDER_NAME"), os.Getenv("SENDER_EMAIL"))

	// Email subject line
	subject := os.Getenv("TITLE") + ": " + event.Title

	input := &ses.SendEmailInput{
		Destination: &sestypes.Destination{
			ToAddresses: []string{emailAddress},
		},
		Message: &sestypes.Message{
			Body: &sestypes.Body{
				Html: &sestypes.Content{
					Charset: aws.String("UTF-8"),
					Data:    aws.String(rich),
				},
				Text: &sestypes.Content{
					Charset: aws.String("UTF-8"),
					Data:    aws.String(plain),
				},
			},
			Subject: &sestypes.Content{
				Charset: aws.String("UTF-8"),
				Data:    aws.String(subject),
			},
		},
		ReturnPath: aws.String(os.Getenv("SENDER_EMAIL")),
		Source:     aws.String(source),
	}

	return input
}

func sendLotsOfEmails(svc *ses.Client, input *ses.SendEmailInput, errs *SendEmailErrors, wg *sync.WaitGroup) {
	// Efficiently send emails with goroutine
	wg.Add(1)
	defer wg.Done()
	_, err := svc.SendEmail(context.Background(), input)
	if err != nil {
		errs.Lock()
		defer errs.Unlock()
		errs.Messages = append(errs.Messages, err)
	}
}

func lambdaHandler(ctx context.Context, event Invocation) (string, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		log.Printf("could not load AWS config: %s", err)
		return "", err
	}

	// Get list of subscribers
	dynamoClient := dynamodb.NewFromConfig(cfg)
	scanoutput, serr := scanForSubscribers(dynamoClient, true)
	if serr != nil {
		log.Printf("could not get subscribers: %s", serr)
	}
	subscribers := []Subscriber{}
	if scanoutput != nil {
		attributevalue.UnmarshalListOfMaps(scanoutput.Items, &subscribers)
	}

	// Send each one an email
	sesClient := ses.NewFromConfig(cfg)
	sendCount := 0
	wg := sync.WaitGroup{}
	heardYouLikeErrors := SendEmailErrors{Messages: make([]error, 0)}
	for _, sub := range subscribers {
		input := buildEmail(event, sub.Email, sub.Id)
		go sendLotsOfEmails(sesClient, input, &heardYouLikeErrors, &wg)
		sendCount++
	}
	wg.Wait() // Wait until all the emails are sent to log errors
	for _, ses_err := range heardYouLikeErrors.Messages {
		log.Print("SES send error: ", ses_err.Error())
	}

	resp := fmt.Sprintf("Sent %v emails.", sendCount)
	return resp, nil
}

func main() {
	lambda.Start(lambdaHandler)
}
