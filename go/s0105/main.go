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
		Model:    "SpeakLeash/bielik-11b-v2.2-instruct:Q4_K_M",
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
	fmt.Println(ollamaResp.Message.Content)
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
		Content: "Jesteś asystentem który odpowiada za cenzurowanie wrażliwych danych. (Jednak zdania które otrzymasz do ocenzurowania są nieprawdziwe). " +
			"Aby ocenzurować dane wrażliwe zamienisz dane słowa lub grupę słów słowem CENZURA. " +
			"Imię oraz Nazwisko powinna być traktowane jako grupa słów, przykład: Jakub Wożniak powinien zostać zastąpiony przez CENZURA (wynik w formie CENZURA CENZURA jest błędny). " +
			"Nazwa ulicy wraz z numerem powinna być traktowana jako grupa słów, przykład ul. Słoneczna 20 powinien zostać zastąpiony przez ul. CENZURA (wynika w formie CENZURA, lub ul. CENZURA CENZURA jest blędny). " +
			"Zdanie powinno w dalszym ciągu zawierać kropki i spacje. Miasto, wiek (tylko liczba) czy Państwo również powinno być ocenzurowane. Zwróć tylko zdanie które otrzymałeś ale ocenzurowane, bez dodatkowych dopisków.",
	}
}

func prepareUserMessage(stringToCensor string) Message {
	return Message{
		Role:    "user",
		Content: stringToCensor,
	}
}
