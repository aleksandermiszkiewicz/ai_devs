package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/joho/godotenv"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
)

type FinalAnswer struct {
	Task   string          `json:"task"`
	APIKey string          `json:"apikey"`
	Answer CalibrationData `json:"answer"`
}

type CalibrationData struct {
	APIKey      string     `json:"apikey"`
	Description string     `json:"description"`
	Copyright   string     `json:"copyright"`
	TestData    []TestData `json:"test-data"`
}

type TestData struct {
	Question string `json:"question"`
	Answer   int    `json:"answer"`
	Test     *Test  `json:"test,omitempty"`
}

type Test struct {
	Question string `json:"q,omitempty"`
	Answer   string `json:"a,omitempty"`
}

func main() {
	ctx := context.Background()
	err := godotenv.Load("../../.env")
	if err != nil {
		log.Fatalf("could not load env variables %v", err)
	}
	loadJsonFile, err := os.ReadFile("json.txt")
	if err != nil {
		log.Fatalln("something went wrong while loading json file", err)
	}
	var calibrationData = CalibrationData{}

	if err := json.Unmarshal(loadJsonFile, &calibrationData); err != nil {
		log.Fatalln("something went wrong while reading json data", err)
	}

	var questionsToModel []string
	for _, d := range calibrationData.TestData {
		if d.Test != nil && len(d.Test.Question) > 0 {
			questionsToModel = append(questionsToModel, d.Test.Question)
		}
	}

	if len(questionsToModel) == 0 {
		log.Fatal("questions not found")
	}

	log.Printf("there are questions to the model")
	openaiClient := openai.NewClient(option.WithAPIKey(os.Getenv("OPENAI_API_KEY")))
	response := callModelForAnswer(ctx, openaiClient, questionsToModel)

	var finalTestData []Test
	if err := json.Unmarshal([]byte(response), &finalTestData); err != nil {
		log.Fatalln("could not unmarshall model response", err)
	}
	for i := range calibrationData.TestData {
		if calibrationData.TestData[i].Test != nil && calibrationData.TestData[i].Test.Question == "" && calibrationData.TestData[i].Test.Answer == "" {
			calibrationData.TestData[i].Test = nil
		} else {
			for _, r := range finalTestData {
				if calibrationData.TestData[i].Test != nil && calibrationData.TestData[i].Test.Question == r.Question {
					calibrationData.TestData[i].Test.Answer = r.Answer
					break
				}
			}
		}
		calibrationData.TestData[i].Answer = validateCalculation(calibrationData.TestData[i].Question)
	}
	final := &FinalAnswer{
		APIKey: os.Getenv("AI_DEVS_API_KEY"),
		Task:   "JSON",
		Answer: calibrationData,
	}

	jsonString, _ := json.Marshal(final)
	if err := os.WriteFile("final.json", jsonString, os.ModePerm); err != nil {
		log.Fatalln("writing json file failed", jsonString)
	}

	client := http.Client{}
	resp, err := client.Post(fmt.Sprintf("%s/report", os.Getenv("CENTRALA_HOST")), "application/json", bytes.NewBuffer(jsonString))
	if err != nil {
		log.Fatalln("something went wrong while sending final report", err)
	}
	content, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalln("could not read fina report response", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusBadRequest {
		log.Fatalf("bad request %s", string(content))
	}
	if resp.StatusCode == http.StatusOK {
		log.Println("final report sent with success")
		log.Println(string(content))
	}
}

func validateCalculation(question string) int {
	numbers := strings.Split(question, " + ")
	first, _ := strconv.Atoi(numbers[0])
	second, _ := strconv.Atoi(numbers[1])
	return first + second
}

func prepareSystemMessage() openai.ChatCompletionMessageParam {
	return openai.ChatCompletionMessageParam{
		Role: openai.F(openai.ChatCompletionMessageParamRole("system")),
		Content: openai.F[interface{}]("You will receive the list of questions. " +
			"You need to answer as short as possible, the best answer is 1 word if possible. " +
			"The response need to be written in json format. " +
			"The example of json format is presented below:  " +
			"[{\"q\":\"What is the capital city of Germany?\",\"a\":\"Berlin\"}]"),
	}
}

func prepareUserMessage(questions []string) openai.ChatCompletionMessageParam {
	stringQuestions := strings.Join(questions, "\n")
	return openai.ChatCompletionMessageParam{
		Role:    openai.F(openai.ChatCompletionMessageParamRole("user")),
		Content: openai.F[interface{}](stringQuestions),
	}
}

func callModelForAnswer(ctx context.Context, client *openai.Client, questions []string) string {
	chatCompletion, err := client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Messages: openai.F([]openai.ChatCompletionMessageParamUnion{prepareSystemMessage(), prepareUserMessage(questions)}),
		Model:    openai.F(openai.ChatModelGPT4oMini),
	})

	if err != nil {
		log.Fatalln("error while calling openai", err)
	}
	answers := chatCompletion.Choices[0].Message.Content
	log.Printf("response %s", answers)
	return answers
}
