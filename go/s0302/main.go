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
	"strings"
)

const rootDir = "../../pliki_z_fabryki/do-not-share"

type FinalAnswer struct {
	Task   string `json:"task"`
	APIKey string `json:"apikey"`
	Answer string `json:"answer"`
}

func main() {
	ctx := context.Background()
	err := godotenv.Load("../../.env")

	if err != nil {
		log.Fatalf("could not load env variables %v", err)
	}

	apiKey := os.Getenv("OPENAI_API_KEY")

	openaiClient := openai.NewClient(option.WithAPIKey(apiKey))

	finalDate, err := callModel(getFilesContent(), ctx, openaiClient)
	if err != nil {
		log.Fatalf("something went wrong %s", err)
	}
	log.Println(finalDate)
	sendResult(finalDate)
}

func getFilesContent() string {
	files, err := os.ReadDir(rootDir)
	if err != nil {
		log.Fatal(err)
	}
	merged := ""
	for _, file := range files {
		if !file.IsDir() && strings.Contains(file.Name(), ".txt") {
			fileContent, err := os.ReadFile(fmt.Sprintf("%s/%s", rootDir, file.Name()))
			if err != nil {
				panic(err)
			}
			merged = merged + fmt.Sprintf("\nFile name: `%s` | File content: `%s`", file.Name(), string(fileContent))
		}
	}
	print(merged)
	return merged
}

func sendResult(finalDate string) {
	host := os.Getenv("CENTRALA_HOST")
	apikey := os.Getenv("AI_DEVS_API_KEY")
	client := http.Client{}

	payload, _ := json.Marshal(FinalAnswer{
		Task:   "wektory",
		APIKey: apikey,
		Answer: finalDate,
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

func callModel(filesContent string, ctx context.Context, client *openai.Client) (string, error) {
	resp, err := client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Messages: openai.F([]openai.ChatCompletionMessageParamUnion{prepareSystemMessage(), prepareUserMessage(filesContent)}),
		Model:    openai.F(openai.ChatModelGPT4oMini),
	})
	if err != nil {
		log.Fatalf("Error while sending message: %v", err)
		return "", err
	}
	answer := resp.Choices[0].Message.Content
	return answer, nil
}

func prepareSystemMessage() openai.ChatCompletionMessageParam {
	return openai.ChatCompletionMessageParam{
		Role: openai.F(openai.ChatCompletionMessageParamRole("system")),
		Content: openai.F[interface{}]("" +
			"You are powerful assistant which needs the help me analyzed files. You will receive the files (file name and file contents) which you will need to analyse. The file name contains the date in which the report was prepared." +
			"In report from which day there is information about the theft of a weapon prototype, please provide the answer which is only the date when it happens. The date should be in format YYYY-MM-DD",
		),
	}
}

func prepareUserMessage(fileContent string) openai.ChatCompletionMessageParam {
	return openai.ChatCompletionMessageParam{
		Role:    openai.F(openai.ChatCompletionMessageParamRole("user")),
		Content: openai.F[interface{}](fmt.Sprintf("Files content: \n %s", fileContent)),
	}
}
