package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"text/template"
	"time"

	"os"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/ses"
	"github.com/google/uuid"
)

type Invocation struct {
	Title string `json:"title"`
	Description string `json:"description"`
	Content string `json:"content"`
	Plain string `json:"plain"`
	Link string `json:"link"`
}

type Subscriber struct {
	Email string
	Id string
}

// Find list items with the given confirmation status
func scanForSubscribers(confirm bool) (*dynamodb.ScanOutput, error) {
	table := os.Getenv("DB_TABLE_NAME")
	svc := dynamodb.New(session.New())
	input := &dynamodb.ScanInput{
		ExpressionAttributeNames: map[string]*string{
			"#C": aws.String("confirm"),
		},
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":yes": {BOOL: aws.Bool(confirm)},
		},
		FilterExpression: aws.String("#C = :yes"),
		ProjectionExpression: aws.String("email, id"),
		TableName: aws.String(table),
	}

	result, err := svc.Scan(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case dynamodb.ErrCodeProvisionedThroughputExceededException:
				log.Print(dynamodb.ErrCodeProvisionedThroughputExceededException, aerr.Error())
			case dynamodb.ErrCodeResourceNotFoundException:
				log.Print(dynamodb.ErrCodeResourceNotFoundException, aerr.Error())
			case dynamodb.ErrCodeRequestLimitExceeded:
				log.Print(dynamodb.ErrCodeRequestLimitExceeded, aerr.Error())
			case dynamodb.ErrCodeInternalServerError:
				log.Print(dynamodb.ErrCodeInternalServerError, aerr.Error())
			default:
				log.Print("DynamoDB scan error: ", aerr.Error())
			}
		} else {
			// Print the error, cast err to awserr.Error to get the Code and
			// Message from an error.
			log.Print("DynamoDB scan error: ", err.Error())
		}
	}
	return result, err
}

// Update subscriber IDs
func updateIdsInDynamoDB(email string, id string, timestamp string, confirm bool) (*dynamodb.UpdateItemOutput, error) {
	table := os.Getenv("DB_TABLE_NAME")
	svc := dynamodb.New(session.New())

	input := &dynamodb.UpdateItemInput{
		// Provide the key to use for finding the right item.
		Key: map[string]*dynamodb.AttributeValue{
			"email": {
				S: aws.String(email),
			},
		},
		// Give the keys a shorthand to reference
		ExpressionAttributeNames: map[string]*string{
			"#ID": aws.String("id"),
			"#T":  aws.String("timestamp"),
			"#C":  aws.String("confirm"),
		},
		// Give the incoming values a shorthand to reference
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":idval": {
				S: aws.String(id),
			},
			":timeval": {
				S: aws.String(timestamp),
			},
			":yes": {
				BOOL: aws.Bool(true),
			},
		},
		// Use the shorthand references to update these keys
		ConditionExpression: aws.String("#C = :yes"),
		UpdateExpression: aws.String("SET #T = :timeval, #ID = :idval"),
		TableName:        aws.String(table),
	}

	result, err := svc.UpdateItem(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case dynamodb.ErrCodeConditionalCheckFailedException:
				log.Print(dynamodb.ErrCodeConditionalCheckFailedException, aerr.Error())
			case dynamodb.ErrCodeProvisionedThroughputExceededException:
				log.Print(dynamodb.ErrCodeProvisionedThroughputExceededException, aerr.Error())
			case dynamodb.ErrCodeResourceNotFoundException:
				log.Print(dynamodb.ErrCodeResourceNotFoundException, aerr.Error())
			case dynamodb.ErrCodeItemCollectionSizeLimitExceededException:
				log.Print(dynamodb.ErrCodeItemCollectionSizeLimitExceededException, aerr.Error())
			case dynamodb.ErrCodeTransactionConflictException:
				log.Print(dynamodb.ErrCodeTransactionConflictException, aerr.Error())
			case dynamodb.ErrCodeRequestLimitExceeded:
				log.Print(dynamodb.ErrCodeRequestLimitExceeded, aerr.Error())
			case dynamodb.ErrCodeInternalServerError:
				log.Print(dynamodb.ErrCodeInternalServerError, aerr.Error())
			default:
				log.Print("DynamoDB update error: ", aerr.Error())
			}
		} else {
			// Print the error, cast err to awserr.Error to get the Code and
			// Message from an error.
			log.Print("DynamoDB update error: ", err.Error())
		}
	}
	return result, err
}

func sendEmailWithSES(subject string, rich string, text string, emailAddress string, id string) (*ses.SendEmailOutput, error) {
	svc := ses.New(session.New())

	unsubLink := os.Getenv("UNSUBSCRIBE_LINK")

	// HTML format
	msg := rich + "\n\n<hr>\n\n<p style=\"font-size: 0.9em;\">You're subscribed to <a href=\"" + os.Getenv("WEBSITE") + "\">" + os.Getenv("TITLE") + "</a>. Click here to <a href=\"" + unsubLink + "?email=" + emailAddress + "&id=" + id + "\">unsubscribe</a>.</p></body></html>"

	// Plain text format
	txt := text + "\n\n---\n\nYou've subscribed at " + os.Getenv("WEBSITE") + ". To unsubscribe, use: " + unsubLink + "?email=" + emailAddress + "&id=" + id

	// Build the "from" value
	source := fmt.Sprintf("\"%s\" <%s>", os.Getenv("SENDER_NAME"), os.Getenv("SENDER_EMAIL"))

	input := &ses.SendEmailInput{
		Destination: &ses.Destination{
			ToAddresses: []*string{
				aws.String(emailAddress),
			},
		},
		Message: &ses.Message{
			Body: &ses.Body{
				Html: &ses.Content{
					Charset: aws.String("UTF-8"),
					Data:    aws.String(msg),
				},
				Text: &ses.Content{
					Charset: aws.String("UTF-8"),
					Data:    aws.String(txt),
				},
			},
			Subject: &ses.Content{
				Charset: aws.String("UTF-8"),
				Data:    aws.String(subject),
			},
		},
		ReturnPath: aws.String(os.Getenv("SENDER_EMAIL")),
		Source:     aws.String(source),
	}

	result, err := svc.SendEmail(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case ses.ErrCodeMessageRejected:
				log.Print(ses.ErrCodeMessageRejected, aerr.Error())
			case ses.ErrCodeMailFromDomainNotVerifiedException:
				log.Print(ses.ErrCodeMailFromDomainNotVerifiedException, aerr.Error())
			case ses.ErrCodeConfigurationSetDoesNotExistException:
				log.Print(ses.ErrCodeConfigurationSetDoesNotExistException, aerr.Error())
			case ses.ErrCodeConfigurationSetSendingPausedException:
				log.Print(ses.ErrCodeConfigurationSetSendingPausedException, aerr.Error())
			case ses.ErrCodeAccountSendingPausedException:
				log.Print(ses.ErrCodeAccountSendingPausedException, aerr.Error())
			default:
				log.Print("SES send error: ", aerr.Error())
			}
		} else {
			// Print the error, cast err to awserr.Error to get the Code and
			// Message from an error.
			log.Print("SES send error:", err.Error())
		}
	}

	return result, nil
}

func lambdaHandler(ctx context.Context, event Invocation) (string, error) {
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

	// Build the email
	subject := os.Getenv("TITLE") + ": " + event.Title

	t, _ := template.ParseFiles("template.html")
	t.Execute(&htmlBody, templateData)

	rich := htmlBody.String()

	text := event.Title + "\n\n" + event.Description + "\n\n---\n\nYou can view this email as HTML, or read this on my site: \n" + event.Link + "\n\n---\n\n" + event.Plain

	// Get list of subscribers
	scanoutput, err := scanForSubscribers(true)
	if err != nil {
		log.Printf("could not get subscribers: %s", err)
	}
	subscribers := []Subscriber{}
	dynamodbattribute.UnmarshalListOfMaps(scanoutput.Items, &subscribers)
	
	// Send each one an email
	sendCount := 0
	for _,sub := range subscribers {
		_, err := sendEmailWithSES(subject, rich, text, sub.Email, sub.Id)
		if err != nil {
			log.Printf("could not send email to %s: %s", sub.Email, err)
		}
		if err == nil {
			sendCount ++
			
			// Reset subscriber ID
			now := time.Now().Format("2006-01-02 15:04:05")
			newId := uuid.New().String()
			_, rerr := updateIdsInDynamoDB(sub.Email, newId, now, true)
			if rerr != nil {
				log.Printf("could not update ID for %s: %s", sub.Email, err)
			}
		}
	}

	resp := fmt.Sprintf("Sent %v emails.", sendCount)
	return resp, nil
}

func main() {
	lambda.Start(lambdaHandler)
}
