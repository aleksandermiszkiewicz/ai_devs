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
	"time"
)

const rootDir = "../../pliki_z_fabryki"

type FinalAnswer struct {
	Task   string            `json:"task"`
	APIKey string            `json:"apikey"`
	Answer map[string]string `json:"answer"`
}

func main() {
	ctx := context.Background()
	err := godotenv.Load("../../.env")

	if err != nil {
		log.Fatalf("could not load env variables %v", err)
	}

	apiKey := os.Getenv("OPENAI_API_KEY")

	openaiClient := openai.NewClient(option.WithAPIKey(apiKey))

	reportFiles, err := os.ReadDir(rootDir)
	if err != nil {
		log.Fatal(err)
	}

	reportsTags := assigneTags(reportFiles, rootDir, getFactFilesContent(), ctx, openaiClient)

	log.Println(reportsTags)

	sendResult(reportsTags)
}

func getFactFilesContent() string {
	factsDir := rootDir + "/facts"
	factFiles, err := os.ReadDir(factsDir)
	if err != nil {
		log.Fatal(err)
	}
	merged := ""
	for _, file := range factFiles {
		if !file.IsDir() && strings.Contains(file.Name(), ".txt") {
			fileContent, err := os.ReadFile(fmt.Sprintf("%s/%s", factsDir, file.Name()))
			if err != nil {
				panic(err)
			}
			merged = merged + fmt.Sprintf("\nFact file name: `%s` | Fact file content: `%s`", file.Name(), string(fileContent))
		}
	}
	return merged
}

func assigneTags(files []os.DirEntry, rootDir string, factsFilesContent string, ctx context.Context, client *openai.Client) map[string]string {
	tags := make(map[string]string)
	for _, file := range files {
		if !file.IsDir() && strings.Contains(file.Name(), ".txt") {
			fileContent, err := os.ReadFile(fmt.Sprintf("%s/%s", rootDir, file.Name()))
			if err != nil {
				panic(err)
			}
			responseTags, err := callModel(file.Name(), string(fileContent), factsFilesContent, ctx, client)
			if err != nil {
				log.Fatalln(err)
			}
			log.Printf("| %s | %s", file.Name(), responseTags)
			tags[file.Name()] = responseTags
			time.Sleep(5 * time.Second)
		}
	}
	return tags
}

func sendResult(tags map[string]string) {
	host := os.Getenv("CENTRALA_HOST")
	apikey := os.Getenv("AI_DEVS_API_KEY")
	client := http.Client{}

	payload, _ := json.Marshal(FinalAnswer{
		Task:   "dokumenty",
		APIKey: apikey,
		Answer: tags,
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

func callModel(fileName string, fileContent string, factsFilesContent string, ctx context.Context, client *openai.Client) (string, error) {
	resp, err := client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Messages: openai.F([]openai.ChatCompletionMessageParamUnion{prepareSystemMessage(factsFilesContent), prepareUserMessage(fileName, fileContent)}),
		Model:    openai.F(openai.ChatModelGPT4oMini),
	})
	if err != nil {
		log.Fatalf("Error while sending message: %v", err)
		return "", err
	}
	answer := resp.Choices[0].Message.Content
	return answer, nil
}

func prepareSystemMessage(factsFilesContent string) openai.ChatCompletionMessageParam {
	return openai.ChatCompletionMessageParam{
		Role: openai.F(openai.ChatCompletionMessageParamRole("system")),
		Content: openai.F[interface{}]("" +
			"You are powerful assistant which needs the help me analyzed files. The directory of the files is called \"pliki_z_fabryki\"" +
			"You need to analyzed the content of the file and generate based on the file content the keywords (in denominator), which then will help to group the files." +
			"During keywords generation take into account the name of the directory in which files are located as well as the file name." +
			"While generating keywords for reports you need to take into account also the content of the Fact files which you can find below (the fact file has name like `f01.txt`, `f02.txt`... `f09.txt` etc." +
			"You need bind the content of the report (if possible) with some fact file for example by person name / surname or location and then based on this generate keywords." +
			"Answer need to be in Polish. The answer should contain only coma seperated denominators starting from small letter. For each report generate at least 15 keywords. When any person is mentioned in the report the keywords for the given report also should include the profession as keyword. Do not forget about keywords related to Barbara Zawadzka . The keywords should not contain additional signs like `-`, `_` etc. (only white space is allowed)." +
			"The content of the Fact files can be found below: \n" + factsFilesContent,
		),
	}
}

func prepareUserMessage(fileName string, fileContent string) openai.ChatCompletionMessageParam {
	return openai.ChatCompletionMessageParam{
		Role:    openai.F(openai.ChatCompletionMessageParamRole("user")),
		Content: openai.F[interface{}](fmt.Sprintf("File name: `%s`. File Content: \n %s", fileName, fileContent)),
	}
}
