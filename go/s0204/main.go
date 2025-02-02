package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/joho/godotenv"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

const modelType = "gemini-2.0-flash-exp"
const rootDir = "../../pliki_z_fabryki"

type FinalAnswer struct {
	Task   string         `json:"task"`
	APIKey string         `json:"apikey"`
	Answer Classification `json:"answer"`
}

type Classification struct {
	People   []string `json:"people"`
	Hardware []string `json:"hardware"`
}

func main() {
	ctx := context.Background()
	err := godotenv.Load("../../.env")
	if err != nil {
		log.Fatalf("could not load env variables %v", err)
	}

	apiKey := os.Getenv("GEMINI_API_KEY")

	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		log.Fatalf("Error creating client: %v", err)
	}

	defer client.Close()

	files, err := os.ReadDir(rootDir)
	if err != nil {
		log.Fatal(err)
	}

	people, hardware := categorizeFiles(files, ctx, client)

	log.Printf("people: %s", people)
	log.Printf("hardware: %s", hardware)
	sendResult(people, hardware)
}

func categorizeFiles(files []os.DirEntry, ctx context.Context, client *genai.Client) ([]string, []string) {
	var people []string
	var hardware []string
	for _, file := range files {
		fileContent, err := os.ReadFile(fmt.Sprintf("%s/%s", rootDir, file.Name()))
		if err != nil {
			panic(err)
		}
		var requestContent []genai.Part

		if strings.Contains(file.Name(), ".png") {
			requestContent = append(requestContent, genai.ImageData("png", fileContent))
		} else if strings.Contains(file.Name(), ".txt") {
			requestContent = append(requestContent, genai.Text(file.Name()), genai.Text(fileContent))
		} else if strings.Contains(file.Name(), ".mp3") {
			requestContent = append(requestContent, genai.Text(file.Name()), genai.Blob{MIMEType: "audio/mp3", Data: fileContent})
		} else {
			log.Fatalf("error: unknown file type %s", file.Name())
		}

		requestContent = append(requestContent, preparePrompt())
		category, err := callModel(requestContent, ctx, client)
		if err != nil {
			log.Fatalln(err)
		}
		if strings.Contains(category, "PEOPLE") {
			log.Printf("file %s classified to PEOPLE", file.Name())
			people = append(people, file.Name())
		} else if strings.Contains(category, "HARDWARE") {
			log.Printf("file %s classified to HARDWARE", file.Name())
			hardware = append(hardware, file.Name())
		} else {
			log.Printf("file %s not classified to any category", file.Name())
		}
		time.Sleep(10 * time.Second)
	}
	return people, hardware
}

func sendResult(people []string, hardware []string) {
	host := os.Getenv("CENTRALA_HOST")
	apikey := os.Getenv("AI_DEVS_API_KEY")
	client := http.Client{}

	payload, _ := json.Marshal(FinalAnswer{
		Task:   "kategorie",
		APIKey: apikey,
		Answer: Classification{People: people, Hardware: hardware},
	})

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

func callModel(requestContent []genai.Part, ctx context.Context, client *genai.Client) (string, error) {
	model := client.GenerativeModel(modelType)

	model.SetTemperature(0.5)
	model.SetTopK(40)
	model.SetTopP(0.95)
	model.SetMaxOutputTokens(8192)

	session := model.StartChat()
	session.History = []*genai.Content{}

	resp, err := session.SendMessage(ctx, requestContent...)
	if err != nil {
		log.Fatalf("Error sending message: %v", err)
		return "", err
	}

	for _, part := range resp.Candidates[0].Content.Parts {
		content := fmt.Sprintf("%v", part)
		return content, nil
	}
	return "", nil
}

func preparePrompt() genai.Text {
	return "You are helpful assistant. Based on the provided instructions, classify the content of the send files (.mp3, .png and .txt) into one of the following categories:" +
		" - \"PEOPLE\" if the note contains information about captured people or traced of their presence." +
		" - \"HARDWARE\" if the note contains information only about the repaired hardware faults, software issues should not be included to this category." +
		" - \"UNKNOWN\" if the file can not be assigned to PEOPLE or HARDWARE category." +
		"Your response should contains only category."
}
