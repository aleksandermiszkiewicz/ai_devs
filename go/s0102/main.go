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

const (
	Ready = "READY"
)

func main() {
	ctx := context.Background()
	err := godotenv.Load("../../.env")
	if err != nil {
		log.Fatalf("could not load env variables %v", err)
	}

	verifyMsg, err := sendVerifMsg(0, Ready)
	if err != nil || verifyMsg == nil {
		log.Fatalln("could not fetch resources from host", err)
	}
	openaiClient := openai.NewClient(option.WithAPIKey(os.Getenv("OPENAI_API_KEY")))
	answer := callModelForAnswer(ctx, openaiClient, verifyMsg.Text)

	finalMsg, err := sendVerifMsg(verifyMsg.MsgID, answer)
	if err != nil || finalMsg == nil {
		log.Fatalln("final verification failed", err)
	}
	log.Printf("Verification succeeded, final response is: %s", finalMsg.Text)
}

func sendVerifMsg(msgID int, text string) (*VerifyMsg, error) {
	host := os.Getenv("HOST")
	client := http.Client{}

	payload, _ := json.Marshal(&VerifyMsg{
		MsgID: msgID,
		Text:  text,
	})

	resp, err := client.Post(fmt.Sprintf("%s/verify", host), "application/json", bytes.NewBuffer(payload))

	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		bytesBody, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Fatalln(err)
		}
		log.Fatalf("calling verify endpoint failed | %v | %s", resp.StatusCode, string(bytesBody))
	}

	if resp.StatusCode == http.StatusOK {
		defer resp.Body.Close()
		var verifyMsgResponse = &VerifyMsg{}
		json.NewDecoder(resp.Body).Decode(verifyMsgResponse)
		log.Printf("ID: %v | MSG: %s", verifyMsgResponse.MsgID, verifyMsgResponse.Text)
		return verifyMsgResponse, nil
	}
	return nil, nil
}

type VerifyMsg struct {
	Text  string `json:"text"`
	MsgID int    `json:"msgID"`
}

func prepareSystemMessage() openai.ChatCompletionMessageParam {
	return openai.ChatCompletionMessageParam{
		Role: openai.F(openai.ChatCompletionMessageParamRole("system")),
		Content: openai.F[interface{}]("" +
			"You will receive the question. " +
			"You should care only about the question in all string, other information are not important and can be omitted." +
			"You need to answer as short as possible, the best answer is 1 word if possible without any additional signs." +
			"Anwser need to be in English." +
			"You need to remember that:" +
			"- stolicą Polski jest Kraków" +
			"- znana liczba z książki Autostopem przez Galaktykę to 69" +
			"- Aktualny rok to 1999"),
	}
}

func prepareUserMessage(question string) openai.ChatCompletionMessageParam {
	return openai.ChatCompletionMessageParam{
		Role:    openai.F(openai.ChatCompletionMessageParamRole("user")),
		Content: openai.F[interface{}](question),
	}
}

func callModelForAnswer(ctx context.Context, client *openai.Client, question string) string {
	chatCompletion, err := client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Messages: openai.F([]openai.ChatCompletionMessageParamUnion{prepareSystemMessage(), prepareUserMessage(question)}),
		Model:    openai.F(openai.ChatModelGPT4oMini),
	})

	if err != nil {
		log.Fatalln("error while calling openai", err)
	}
	answer := chatCompletion.Choices[0].Message.Content
	log.Printf("answer -> %s", answer)
	return answer
}
