package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sse/dtos"
	"sse/internal/mocks"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"golang.org/x/exp/slices"
)

type QuestionApiTestSuite struct {
	suite.Suite
	router        *gin.Engine
	apiPrefix     string
	tokenUser_Foo string
	tokenUser_Bar string
}

func (suite *QuestionApiTestSuite) SetupSuite() {
	os.Setenv("USE_MOCK_JWKS", "true")
	start = func(r *gin.Engine) {}

	main()

	suite.router = r
	suite.apiPrefix = "/api/v1"
	suite.tokenUser_Foo = mocks.GetToken("Foo", "Foo_Tester")
	suite.tokenUser_Bar = mocks.GetToken("Bar", "Bar_Tester")
}

func (suite *QuestionApiTestSuite) SetupTest() {
	startSession(suite)
}

func (suite *QuestionApiTestSuite) TestApi_UNAUTHORIZED_401() {
	type test struct {
		name       string
		httpMethod string
		path       string
	}

	tests := []test{
		{"Question_New_UNAUTHORIZED_401", http.MethodPost, "/question/new"},
		{"Question_Upvote_UNAUTHORIZED_401", http.MethodPut, "/question/upvote/question1"},
		{"Question_Answer_UNAUTHORIZED_401", http.MethodPut, "/question/answer/question1"},
		{"Question_Rest_UNAUTHORIZED_401", http.MethodGet, "/question/session"},
		{"Question_Rest_UNAUTHORIZED_401", http.MethodPost, "/question/session/start"},
		{"Question_Rest_UNAUTHORIZED_401", http.MethodPost, "/question/session/stop"},
		{"Events_UNAUTHORIZED_401", http.MethodGet, "/events"},
	}

	for _, test := range tests {
		suite.T().Run(test.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest(test.httpMethod, fmt.Sprintf("%s%s", suite.apiPrefix, test.path), nil)
			req.Header.Set("Content-Type", "application/json")
			suite.router.ServeHTTP(w, req)

			assert.Equal(suite.T(), http.StatusUnauthorized, w.Code)
		})
	}
}

func (suite *QuestionApiTestSuite) TestApi_NOTACCEPTABLE_406_WHEN_NO_SESSION_RUNNING() {
	type test struct {
		name       string
		httpMethod string
		path       string
		payload    *bytes.Buffer
	}

	stopSession(suite)

	token := suite.tokenUser_Foo

	tests := []test{
		{"Question_New_NOTACCEPTABLE_406", http.MethodPost, "/question/new", bytes.NewBuffer([]byte(`{"text": "test question?" }`))},
		{"Question_Upvote_NOTACCEPTABLE_406", http.MethodPut, "/question/upvote/question1", nil},
		{"Question_Answer_NOTACCEPTABLE_406", http.MethodPut, "/question/answer/question1", nil},
		{"Question_Rest_NOTACCEPTABLE_406", http.MethodGet, "/question/session", nil},
	}

	for _, test := range tests {
		suite.T().Run(test.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			var req *http.Request
			if test.payload == nil {
				req, _ = http.NewRequest(test.httpMethod, fmt.Sprintf("%s%s", suite.apiPrefix, test.path), nil)
			} else {
				req, _ = http.NewRequest(test.httpMethod, fmt.Sprintf("%s%s", suite.apiPrefix, test.path), test.payload)
			}

			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
			suite.router.ServeHTTP(w, req)

			assert.Equal(suite.T(), http.StatusNotAcceptable, w.Code)
		})
	}
}

func (suite *QuestionApiTestSuite) TestNewQuestion_OK_200() {
	w := httptest.NewRecorder()

	token := suite.tokenUser_Foo

	jsonData := []byte(`{
		"text": "test question?"
	}`)

	req, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/question/new", suite.apiPrefix), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	suite.router.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusOK, w.Code)
}

func (suite *QuestionApiTestSuite) TestUpvoteQuestion_NOTFOUND_404() {
	w := httptest.NewRecorder()

	token := suite.tokenUser_Foo

	jsonData := []byte(`{
		"id": "invalid"
	}`)

	req, _ := http.NewRequest(http.MethodPut, fmt.Sprintf("%s/question/upvote", suite.apiPrefix), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	suite.router.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusNotFound, w.Code)
}

func (suite *QuestionApiTestSuite) TestAnswerQuestion_NOTFOUND_404() {
	w := httptest.NewRecorder()

	token := suite.tokenUser_Foo

	jsonData := []byte(`{
		"id": "invalid"
	}`)

	req, _ := http.NewRequest(http.MethodPut, fmt.Sprintf("%s/question/answer", suite.apiPrefix), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	suite.router.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusNotFound, w.Code)
}

func (suite *QuestionApiTestSuite) TestUpvoteQuestion_NOTACCEPTABLE_406_WHEN_DOUBLE_VOTE_FROM_USER() {
	w := httptest.NewRecorder()

	token := suite.tokenUser_Foo
	newQuestion := dtos.NewQuestionDto{Text: "new question"}
	postNewQuestion(suite, w, newQuestion, token)

	questionList := getSession(suite, w, token)

	reqv, _ := http.NewRequest(http.MethodPut, fmt.Sprintf("%s/question/upvote/%s", suite.apiPrefix, questionList[0].Id), nil)
	reqv.Header.Set("Content-Type", "application/json")
	reqv.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	suite.router.ServeHTTP(w, reqv)

	w2 := httptest.NewRecorder()
	reqv2, _ := http.NewRequest(http.MethodPut, fmt.Sprintf("%s/question/upvote/%s", suite.apiPrefix, questionList[0].Id), nil)
	reqv2.Header.Set("Content-Type", "application/json")
	reqv2.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	suite.router.ServeHTTP(w2, reqv2)

	assert.Equal(suite.T(), http.StatusNotAcceptable, w2.Code)
}

func (suite *QuestionApiTestSuite) TestUpvoteQuestion_NOTACCEPTABLE_406_WHEN_VOTING_ANSWERED_QUESTION() {
	w := httptest.NewRecorder()

	token := suite.tokenUser_Foo

	jsonData := dtos.NewQuestionDto{Text: "new question"}

	postNewQuestion(suite, w, jsonData, token)

	questionList := getSession(suite, w, token)

	reqa, _ := http.NewRequest(http.MethodPut, fmt.Sprintf("%s/question/answer/%s", suite.apiPrefix, questionList[0].Id), nil)
	reqa.Header.Set("Content-Type", "application/json")
	reqa.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	suite.router.ServeHTTP(w, reqa)

	w2 := httptest.NewRecorder()
	reqv, _ := http.NewRequest(http.MethodPut, fmt.Sprintf("%s/question/upvote/%s", suite.apiPrefix, questionList[0].Id), nil)
	reqv.Header.Set("Content-Type", "application/json")
	reqv.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	suite.router.ServeHTTP(w2, reqv)

	assert.Equal(suite.T(), http.StatusNotAcceptable, w2.Code)
}

func (suite *QuestionApiTestSuite) TestGetSession_OK_200_CREATOR_SHOWN_ONLY_FOR_OWNED_AND_NOT_ANONYMOUS_QUESTIONS() {
	w := httptest.NewRecorder()

	tokenUser_Foo := suite.tokenUser_Foo
	tokenUser_Bar := suite.tokenUser_Bar

	newQuestion_FOO_Q1 := dtos.NewQuestionDto{Text: "Foo Question1", Anonymous: false}
	newQuestion_FOO_Q2 := dtos.NewQuestionDto{Text: "Foo Question2 anonynmous", Anonymous: true}
	newQuestion_BAR_Q1 := dtos.NewQuestionDto{Text: "Bar Question1", Anonymous: false}
	newQuestion_BAR_Q2 := dtos.NewQuestionDto{Text: "Bar Question2 anonynmous", Anonymous: true}

	postNewQuestion(suite, w, newQuestion_FOO_Q1, tokenUser_Foo)
	postNewQuestion(suite, w, newQuestion_FOO_Q2, tokenUser_Foo)
	postNewQuestion(suite, w, newQuestion_BAR_Q1, tokenUser_Bar)
	postNewQuestion(suite, w, newQuestion_BAR_Q2, tokenUser_Bar)

	questionList := getSession(suite, w, tokenUser_Foo)

	question_FOO_Q1 := questionList[slices.IndexFunc(questionList, func(c dtos.QuestionDto) bool { return c.Text == "Foo Question1" })]
	question_FOO_Q2 := questionList[slices.IndexFunc(questionList, func(c dtos.QuestionDto) bool { return c.Text == "Foo Question2 anonynmous" })]
	question_BAR_Q1 := questionList[slices.IndexFunc(questionList, func(c dtos.QuestionDto) bool { return c.Text == "Bar Question1" })]
	question_BAR_Q2 := questionList[slices.IndexFunc(questionList, func(c dtos.QuestionDto) bool { return c.Text == "Bar Question2 anonynmous" })]

	assert.Equal(suite.T(), true, question_FOO_Q1.Owned)
	assert.Equal(suite.T(), false, question_FOO_Q1.Anonymous)
	assert.Equal(suite.T(), "Foo Foo_Tester", question_FOO_Q1.Creator)

	assert.Equal(suite.T(), true, question_FOO_Q2.Owned)
	assert.Equal(suite.T(), true, question_FOO_Q2.Anonymous)
	assert.Equal(suite.T(), "Foo Foo_Tester", question_FOO_Q2.Creator)

	assert.Equal(suite.T(), false, question_BAR_Q1.Owned)
	assert.Equal(suite.T(), false, question_BAR_Q1.Anonymous)
	assert.Equal(suite.T(), "Bar Bar_Tester", question_BAR_Q1.Creator)

	assert.Equal(suite.T(), false, question_BAR_Q2.Owned)
	assert.Equal(suite.T(), true, question_BAR_Q2.Anonymous)
	assert.Equal(suite.T(), "", question_BAR_Q2.Creator)

	assert.Equal(suite.T(), http.StatusOK, w.Code)
}

func (suite *QuestionApiTestSuite) TestUpdateQuestion_OK_200_WHEN_QUESTION_IS_OWNED() {
	w := httptest.NewRecorder()

	tokenUser_Foo := suite.tokenUser_Foo
	newQuestion := dtos.NewQuestionDto{Text: "Foo Question", Anonymous: false}
	postNewQuestion(suite, w, newQuestion, tokenUser_Foo)
	questionList := getSession(suite, w, tokenUser_Foo)
	question_FOO_Q := questionList[slices.IndexFunc(questionList, func(c dtos.QuestionDto) bool { return c.Text == "Foo Question" })]

	updateQuestionDto := dtos.UpdateQuestionDto{Id: question_FOO_Q.Id, Text: "Updated Foo Question", Anonymous: true}

	assert.Equal(suite.T(), "Foo Question", question_FOO_Q.Text)
	assert.Equal(suite.T(), false, question_FOO_Q.Anonymous)

	putUpdateQuestion(suite, w, updateQuestionDto, tokenUser_Foo)

	updatedQuestionList := getSession(suite, w, tokenUser_Foo)
	updated_question_FOO_Q := updatedQuestionList[slices.IndexFunc(updatedQuestionList, func(c dtos.QuestionDto) bool { return c.Id == question_FOO_Q.Id })]

	assert.Equal(suite.T(), "Updated Foo Question", updated_question_FOO_Q.Text)
	assert.Equal(suite.T(), true, updated_question_FOO_Q.Anonymous)
	assert.Equal(suite.T(), http.StatusOK, w.Code)
}

func (suite *QuestionApiTestSuite) TestUpdateQuestion_FORBIDDEN_403_WHEN_QUESTION_IS_NOT_OWNED() {
	w := httptest.NewRecorder()

	tokenUser_Foo := suite.tokenUser_Foo
	tokenUser_Bar := suite.tokenUser_Bar

	newQuestion := dtos.NewQuestionDto{Text: "Foo Question", Anonymous: false}
	postNewQuestion(suite, w, newQuestion, tokenUser_Foo)
	questionList := getSession(suite, w, tokenUser_Foo)
	question_FOO_Q := questionList[slices.IndexFunc(questionList, func(c dtos.QuestionDto) bool { return c.Text == "Foo Question" })]

	updateQuestionDto := dtos.UpdateQuestionDto{Id: question_FOO_Q.Id, Text: "Updated Foo Question", Anonymous: true}

	assert.Equal(suite.T(), "Foo Question", question_FOO_Q.Text)
	assert.Equal(suite.T(), false, question_FOO_Q.Anonymous)

	w2 := httptest.NewRecorder()
	putUpdateQuestion(suite, w2, updateQuestionDto, tokenUser_Bar)

	assert.Equal(suite.T(), http.StatusForbidden, w2.Code)
}

func (suite *QuestionApiTestSuite) TestDeleteQuestion_OK_200_WHEN_QUESTION_IS_OWNED() {
	w := httptest.NewRecorder()

	tokenUser_Foo := suite.tokenUser_Foo
	newQuestion := dtos.NewQuestionDto{Text: "Foo Question", Anonymous: false}
	postNewQuestion(suite, w, newQuestion, tokenUser_Foo)
	questionList := getSession(suite, w, tokenUser_Foo)
	question_FOO_Q := questionList[slices.IndexFunc(questionList, func(c dtos.QuestionDto) bool { return c.Text == "Foo Question" })]

	assert.Equal(suite.T(), "Foo Question", question_FOO_Q.Text)
	assert.Equal(suite.T(), false, question_FOO_Q.Anonymous)

	deleteQuestion(suite, w, question_FOO_Q.Id, tokenUser_Foo)

	updatedQuestionList := getSession(suite, w, tokenUser_Foo)
	idx := slices.IndexFunc(updatedQuestionList, func(c dtos.QuestionDto) bool { return c.Id == question_FOO_Q.Id })

	assert.Equal(suite.T(), -1, idx)
	assert.Equal(suite.T(), http.StatusOK, w.Code)
}

func (suite *QuestionApiTestSuite) TestDeleteQuestion_FORBIDDEN_403_WHEN_QUESTION_IS_NOT_OWNED() {
	w := httptest.NewRecorder()

	tokenUser_Foo := suite.tokenUser_Foo
	tokenUser_Bar := suite.tokenUser_Bar

	newQuestion := dtos.NewQuestionDto{Text: "Foo Question", Anonymous: false}
	postNewQuestion(suite, w, newQuestion, tokenUser_Foo)
	questionList := getSession(suite, w, tokenUser_Foo)
	question_FOO_Q := questionList[slices.IndexFunc(questionList, func(c dtos.QuestionDto) bool { return c.Text == "Foo Question" })]

	assert.Equal(suite.T(), "Foo Question", question_FOO_Q.Text)
	assert.Equal(suite.T(), false, question_FOO_Q.Anonymous)

	w2 := httptest.NewRecorder()
	deleteQuestion(suite, w2, question_FOO_Q.Id, tokenUser_Bar)

	assert.Equal(suite.T(), http.StatusForbidden, w2.Code)
}

func TestQuestionApiSuite(t *testing.T) {
	suite.Run(t, new(QuestionApiTestSuite))
}

func postNewQuestion(suite *QuestionApiTestSuite, w *httptest.ResponseRecorder, question dtos.NewQuestionDto, token string) {
	newQuestion, _ := json.Marshal(question)
	req, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/question/new", suite.apiPrefix), bytes.NewBuffer(newQuestion))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	suite.router.ServeHTTP(w, req)
}

func putUpdateQuestion(suite *QuestionApiTestSuite, w *httptest.ResponseRecorder, question dtos.UpdateQuestionDto, token string) {
	updateQuestion, _ := json.Marshal(question)
	req, _ := http.NewRequest(http.MethodPut, fmt.Sprintf("%s/question/update", suite.apiPrefix), bytes.NewBuffer(updateQuestion))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	suite.router.ServeHTTP(w, req)
}

func deleteQuestion(suite *QuestionApiTestSuite, w *httptest.ResponseRecorder, id string, token string) {
	req, _ := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/question/delete/%s", suite.apiPrefix, id), nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	suite.router.ServeHTTP(w, req)
}

func startSession(suite *QuestionApiTestSuite) {
	w := httptest.NewRecorder()

	tokenUser_Foo := suite.tokenUser_Foo

	req, _ := http.NewRequest("POST", fmt.Sprintf("%s/question/session/start", suite.apiPrefix), nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", tokenUser_Foo))
	suite.router.ServeHTTP(w, req)
}

func stopSession(suite *QuestionApiTestSuite) {
	w := httptest.NewRecorder()

	tokenUser_Foo := suite.tokenUser_Foo

	req, _ := http.NewRequest("POST", fmt.Sprintf("%s/question/session/stop", suite.apiPrefix), nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", tokenUser_Foo))
	suite.router.ServeHTTP(w, req)
}

func getSession(suite *QuestionApiTestSuite, w *httptest.ResponseRecorder, token string) []dtos.QuestionDto {
	reql, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/question/session", suite.apiPrefix), nil)
	reql.Header.Set("Content-Type", "application/json")
	reql.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	suite.router.ServeHTTP(w, reql)

	var questionList []dtos.QuestionDto
	body, _ := io.ReadAll(w.Body)
	json.Unmarshal(body, &questionList)

	return questionList
}
