package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/joho/godotenv"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

type FinalAnswer struct {
	Task   string `json:"task"`
	APIKey string `json:"apikey"`
	Answer string `json:"answer"`
}

type Request struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Response struct {
	Model              string    `json:"model"`
	CreatedAt          time.Time `json:"created_at"`
	Message            Message   `json:"message"`
	Done               bool      `json:"done"`
	TotalDuration      int64     `json:"total_duration"`
	LoadDuration       int       `json:"load_duration"`
	PromptEvalCount    int       `json:"prompt_eval_count"`
	PromptEvalDuration int       `json:"prompt_eval_duration"`
	EvalCount          int       `json:"eval_count"`
	EvalDuration       int64     `json:"eval_duration"`
}

func main() {
	err := godotenv.Load("../../.env")
	if err != nil {
		log.Fatalln("could not load env variables", err)
	}
	ollamaHost := os.Getenv("OLLAMA_HOST")
	centralaHost := os.Getenv("CENTRALA_HOST")
	apiKey := os.Getenv("AI_DEVS_API_KEY")

	contentToCensor, err := fetchContentToCensor(centralaHost, apiKey)
	fmt.Println(contentToCensor)
	if err != nil {
		log.Fatalln(err)
	}
	req := Request{
		Model:    "llama3.2",
		Stream:   false,
		Messages: []Message{prepareSystemMessage(), prepareUserMessage(contentToCensor)},
	}

	resp, err := callOllama(ollamaHost, req)
	if err != nil {
		log.Fatalln("calling ollama failed", err)
	}

	err = sendFinalAnswer(centralaHost, apiKey, resp.Message.Content)
	if err != nil {
		log.Fatalln(err)
	}
}

func callOllama(ollamaHost string, ollamaReq Request) (*Response, error) {
	js, err := json.Marshal(&ollamaReq)
	if err != nil {
		return nil, err
	}
	client := http.Client{}
	httpReq, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/api/chat", ollamaHost), bytes.NewReader(js))
	if err != nil {
		return nil, err
	}
	httpResp, err := client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer httpResp.Body.Close()
	ollamaResp := Response{}
	err = json.NewDecoder(httpResp.Body).Decode(&ollamaResp)
	return &ollamaResp, err
}

func fetchContentToCensor(host string, apiKey string) (string, error) {
	url := host + "/data/" + apiKey + "/cenzura.txt"
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("bad status: %s", resp.Status)
	}

	defer resp.Body.Close()

	contentBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("could not read content %s", err)
	}

	return string(contentBytes), nil
}

func sendFinalAnswer(host string, apiKey string, answer string) error {
	js, err := json.Marshal(&FinalAnswer{
		APIKey: apiKey,
		Task:   "CENZURA",
		Answer: answer,
	})
	if err != nil {
		return err
	}
	fmt.Println(answer)
	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/report", host), bytes.NewReader(js))
	if err != nil {
		return err
	}

	client := http.Client{}
	resp, err := client.Do(req)

	if err != nil {
		return err
	}

	defer resp.Body.Close()
	respContent, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		log.Printf("bad status: %s", resp.Status)
		return fmt.Errorf("bad status: %s, %s", resp.Status, string(respContent))
	}

	log.Printf("request succeded, final anwser accepted -> %s", string(respContent))
	return nil
}

func prepareSystemMessage() Message {
	return Message{
		Role: "system",
		Content: "You will receive string with data to censor." +
			"You need to change all sensitive data to the CENZURA word." +
			"First name and last name should be treated as one world so for example Jakub Wożniak should be parsed to CENZURA." +
			"Street name with number is special use case as this need to be treated as one, for example ul. Słonecznej 20  should be parsed to CENZURA." +
			"City, country or age are also sensitive data." +
			"The structure of the string needs to be same (dots, comas need to stay).",
	}
}

func prepareUserMessage(stringToCensor string) Message {
	return Message{
		Role:    "user",
		Content: stringToCensor,
	}
}
