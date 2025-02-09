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
)

const rootDir = "../../lab_data"

type FinalAnswer struct {
	Task   string   `json:"task"`
	APIKey string   `json:"apikey"`
	Answer []string `json:"answer"`
}

func main() {
	ctx := context.Background()
	err := godotenv.Load("../../.env")

	if err != nil {
		log.Fatalf("could not load env variables %v", err)
	}

	host := os.Getenv("CENTRALA_HOST")
	aiDevsApiKey := os.Getenv("AI_DEVS_API_KEY")
	openaiApiKey := os.Getenv("OPENAI_API_KEY")

	openaiClient := openai.NewClient(option.WithAPIKey(openaiApiKey))

	content := ReadFile(fmt.Sprintf("%s/verify.txt", rootDir))

	var correctData []string
	for _, c := range content {
		id := c[0:2]
		toValidate := c[3:]
		modelMessages := []openai.ChatCompletionMessageParamUnion{prepareUserMessage(toValidate)}
		resp, err := callModel(ctx, openaiClient, modelMessages)
		if err != nil {
			log.Fatalln(err)
		}
		if resp == "Y" {
			correctData = append(correctData, id)
		}
	}

	log.Println(correctData)
	sendResult(host, aiDevsApiKey, correctData)
}

func sendResult(host string, apikey string, finalAnswer []string) {
	client := http.Client{}

	payload, _ := json.Marshal(FinalAnswer{
		Task:   "research",
		APIKey: apikey,
		Answer: finalAnswer,
	})

	fmt.Println(string(payload))

	resp, err := client.Post(fmt.Sprintf("%s/report", host), "application/json", bytes.NewBuffer(payload))

	if err != nil {
		log.Fatalln(err)
	}
	defer func() {
		if resp != nil {
			resp.Body.Close()
		}
	}()

	bytesBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalln(err)
	}

	if resp.StatusCode != http.StatusOK {
		log.Fatalf("calling endpoint failed | %v | %s", resp.StatusCode, string(bytesBody))
	}

	log.Printf("result correct!")
	log.Printf(string(bytesBody))
}

func callModel(ctx context.Context, client *openai.Client, messages []openai.ChatCompletionMessageParamUnion) (string, error) {
	resp, err := client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Messages: openai.F(messages),
		Model:    openai.F("ft:gpt-4o-2024-08-06:personal:aidevs-s0402:AynjYXCK"),
	})
	if err != nil {
		log.Fatalf("Error while sending message: %v", err)
		return "", err
	}
	answer := resp.Choices[0].Message.Content
	return answer, nil
}

func prepareUserMessage(data string) openai.ChatCompletionMessageParam {
	return openai.ChatCompletionMessageParam{
		Role:    openai.F(openai.ChatCompletionMessageParamRole("user")),
		Content: openai.F[interface{}](data),
	}
}
