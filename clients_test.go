package main

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/ses"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockDynamoDBClient is a mock implementation of DynamoDBAPI.
type MockDynamoDBClient struct {
	mock.Mock
}

func (m *MockDynamoDBClient) Scan(ctx context.Context, params *dynamodb.ScanInput, optFns ...func(*dynamodb.Options)) (*dynamodb.ScanOutput, error) {
	args := m.Called(ctx, params)
	out, _ := args.Get(0).(*dynamodb.ScanOutput)
	return out, args.Error(1)
}

func (m *MockDynamoDBClient) UpdateItem(ctx context.Context, params *dynamodb.UpdateItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error) {
	args := m.Called(ctx, params)
	out, _ := args.Get(0).(*dynamodb.UpdateItemOutput)
	return out, args.Error(1)
}

// MockSESClient is a mock implementation of SESAPI.
type MockSESClient struct {
	mock.Mock
}

func (m *MockSESClient) SendEmail(ctx context.Context, params *ses.SendEmailInput, optFns ...func(*ses.Options)) (*ses.SendEmailOutput, error) {
	args := m.Called(ctx, params)
	out, _ := args.Get(0).(*ses.SendEmailOutput)
	return out, args.Error(1)
}

func TestScanForSubscribers(t *testing.T) {
	t.Setenv("DB_TABLE_NAME", "subscribers")
	m := new(MockDynamoDBClient)
	want := &dynamodb.ScanOutput{Count: 2}
	m.On("Scan", mock.Anything, mock.Anything).Return(want, nil)

	got, err := scanForSubscribers(m, true)

	assert.NoError(t, err)
	assert.Equal(t, want, got)
	m.AssertExpectations(t)
}

func TestScanForSubscribersError(t *testing.T) {
	t.Setenv("DB_TABLE_NAME", "subscribers")
	m := new(MockDynamoDBClient)
	m.On("Scan", mock.Anything, mock.Anything).
		Return((*dynamodb.ScanOutput)(nil), errors.New("scan failed"))

	_, err := scanForSubscribers(m, true)

	assert.Error(t, err)
	m.AssertExpectations(t)
}

func TestUpdateIdsInDynamoDB(t *testing.T) {
	t.Setenv("DB_TABLE_NAME", "subscribers")
	m := new(MockDynamoDBClient)
	want := &dynamodb.UpdateItemOutput{}
	m.On("UpdateItem", mock.Anything, mock.Anything).Return(want, nil)

	got, err := updateIdsInDynamoDB(m, "a@example.com", "id-1", "2024-01-01T00:00:00Z", true)

	assert.NoError(t, err)
	assert.Equal(t, want, got)
	m.AssertExpectations(t)
}

func TestSendLotsOfEmailsSuccess(t *testing.T) {
	m := new(MockSESClient)
	m.On("SendEmail", mock.Anything, mock.Anything).Return(&ses.SendEmailOutput{}, nil)

	errs := &SendEmailErrors{Messages: make([]error, 0)}
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go sendLotsOfEmails(m, &ses.SendEmailInput{}, errs, wg)
	wg.Wait()

	assert.Empty(t, errs.Messages)
	m.AssertExpectations(t)
}

func TestSendLotsOfEmailsRecordsError(t *testing.T) {
	m := new(MockSESClient)
	m.On("SendEmail", mock.Anything, mock.Anything).
		Return((*ses.SendEmailOutput)(nil), errors.New("ses unavailable"))

	errs := &SendEmailErrors{Messages: make([]error, 0)}
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go sendLotsOfEmails(m, &ses.SendEmailInput{}, errs, wg)
	wg.Wait()

	assert.Len(t, errs.Messages, 1)
	m.AssertExpectations(t)
}
