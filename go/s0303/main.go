package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/joho/godotenv"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"io"
	"log"
	"net/http"
	"os"
	"slices"
	"strings"
	"time"
)

type TableStructure struct {
	Table       string `json:"Table"`
	CreateTable string `json:"Create Table"`
}

type TableInBanana struct {
	Table string `json:"Tables_in_banana"`
}

type User struct {
	Id          string `json:"id"`
	Username    string `json:"username"`
	AccessLevel string `json:"access_level"`
	IsActive    string `json:"is_active"`
	LastLog     string `json:"lastlog"`
}

type Datacenter struct {
	DCId     string `json:"dc_id"`
	Location string `json:"location"`
	Manager  string `json:"manager"`
	IsActive string `json:"is_active"`
}

type Connection struct {
	User1Id string `json:"user1_id"`
	User2Id string `json:"user2_id"`
}

type DBResponse[T any] struct {
	Reply []T    `json:"reply"`
	Error string `json:"error"`
}

type QueryRequest struct {
	Task   string `json:"task"`
	APIKey string `json:"apikey"`
	Query  string `json:"query"`
}

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

	tables := []string{"users", "datacenters", "connections"}
	tablesStructure := getTablesStructure(host, aiDevsApiKey, tables)
	users := getUsersData(host, aiDevsApiKey)
	datacenters := getDatacentersData(host, aiDevsApiKey)
	connections := getConnectionsData(host, aiDevsApiKey)

	strTbStructures, _ := json.Marshal(tablesStructure)
	strUsers, _ := json.Marshal(users)
	strDCs, _ := json.Marshal(datacenters)
	strConnections, _ := json.Marshal(connections)

	systemMsg := prepareSystemMessage(string(strTbStructures), string(strUsers), string(strDCs), string(strConnections))

	question := "Which active datacenters (DC_ID) are managed by employees which are on leave (is_active=0). The final response should contains only datacenters ID (DC_ID) and looks like -> answer: 123, 321, 111."
	userMsg := prepareUserMessage(question)

	modelMessages := []openai.ChatCompletionMessageParamUnion{systemMsg, userMsg}

	openaiClient := openai.NewClient(option.WithAPIKey(openaiApiKey))
	var finalResp string
	for {
		finalResp, err = callModel(ctx, openaiClient, modelMessages)
		if err != nil {
			log.Fatalf("something went wrong %s", err)
		}
		log.Println(fmt.Sprintf("resp -> %s ", finalResp))
		if strings.Contains(finalResp, "query:") {
			sqlQuery := strings.TrimSpace(strings.TrimPrefix(finalResp, "query:"))
			dbResponse, err := callDbApi(host, aiDevsApiKey, sqlQuery)
			if err != nil {
				log.Fatalln("call DB API failed ", err)
			}
			modelMessages = append(modelMessages, prepareUserMessage(fmt.Sprintf("Additional Data: %s", dbResponse)))
			time.Sleep(5 * time.Second)
		} else {
			break
		}
	}

	log.Println(finalResp)

	if strings.Contains(finalResp, "answer:") {
		answer := strings.TrimSpace(strings.TrimPrefix(finalResp, "answer:"))
		result := strings.Split(answer, ",")
		sendResult(host, aiDevsApiKey, result)
	} else {
		log.Fatalln(fmt.Sprintf("unknown response type: %s", finalResp))
	}
}

func getTablesStructure(host string, apiKey string, tables []string) *[]TableStructure {
	var resp DBResponse[TableStructure]
	var tablesStructure []TableStructure
	for _, table := range tables {
		query := "show create table " + table
		bytesContent, err := callDbApi(host, apiKey, query)
		if err != nil {
			log.Fatalln(err)
		}
		if err := json.Unmarshal(bytesContent, &resp); err != nil {
			log.Fatalln(err)
		}
		tablesStructure = slices.Concat(tablesStructure, resp.Reply)
	}
	return &tablesStructure
}

func getUsersData(host string, apiKey string) *[]User {
	var resp DBResponse[User]
	query := "select * from users"
	bytesContent, err := callDbApi(host, apiKey, query)
	if err != nil {
		log.Fatalln(err)
	}
	if err := json.Unmarshal(bytesContent, &resp); err != nil {
		log.Fatalln(err)
	}

	return &resp.Reply
}

func getDatacentersData(host string, apiKey string) *[]Datacenter {
	var resp DBResponse[Datacenter]
	query := "select * from datacenters"
	bytesContent, err := callDbApi(host, apiKey, query)
	if err != nil {
		log.Fatalln(err)
	}
	if err := json.Unmarshal(bytesContent, &resp); err != nil {
		log.Fatalln(err)
	}

	return &resp.Reply
}

func getConnectionsData(host string, apiKey string) *[]Connection {
	var resp DBResponse[Connection]
	query := "select * from datacenters"
	bytesContent, err := callDbApi(host, apiKey, query)
	if err != nil {
		log.Fatalln(err)
	}
	if err := json.Unmarshal(bytesContent, &resp); err != nil {
		log.Fatalln(err)
	}

	return &resp.Reply
}

func callDbApi(host string, apiKey string, query string) ([]byte, error) {
	client := http.Client{}
	payload, _ := json.Marshal(QueryRequest{
		Task:   "database",
		APIKey: apiKey,
		Query:  query,
	})
	resp, err := client.Post(fmt.Sprintf("%s/apidb", host), "application/json", bytes.NewBuffer(payload))
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
		log.Println("ERROR: %s", string(bytesBody))
		return nil, errors.New("Something went wrong while calling DB API")
	}
	return bytesBody, nil
}

func sendResult(host string, apikey string, finalAnswer []string) {
	client := http.Client{}

	payload, _ := json.Marshal(FinalAnswer{
		Task:   "database",
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
		Model:    openai.F(openai.ChatModelGPT4oMini),
	})
	if err != nil {
		log.Fatalf("Error while sending message: %v", err)
		return "", err
	}
	answer := resp.Choices[0].Message.Content
	return answer, nil
}

func prepareSystemMessage(tbStrutures string, users string, datacenters string, connections string) openai.ChatCompletionMessageParam {
	return openai.ChatCompletionMessageParam{
		Role: openai.F(openai.ChatCompletionMessageParamRole("system")),
		Content: openai.F[interface{}]("" +
			"You are powerful assistant which needs to help me to extract data from one database." +
			"Database has tables: users, datacenters, connections. Tables structure is presented in the below jsons: \n" +
			tbStrutures + "\n" +
			"Table `users` contain following data: \n" + users + "\n" +
			"Table `datacenters` contain following data: \n" + datacenters + "\n" +
			"Table `connections` contain following data: \n" + connections + "\n" +
			"Your goal is the help me further search the database to answers on different questions." +
			"You can answer in two ways: " +
			"1. If you need to do a query to the database to get more data you should send SQL query (everything in lowercase), then your answer should looks like: \"query: <sql_query>\". For example: query: select * from users where is_active=1" +
			"2. If you know the answer on my question you need to response only in the format like: answer: <answer on my question>" +
			"Next message will contain the my question.",
		),
	}
}

func prepareUserMessage(someData string) openai.ChatCompletionMessageParam {
	return openai.ChatCompletionMessageParam{
		Role:    openai.F(openai.ChatCompletionMessageParamRole("user")),
		Content: openai.F[interface{}](someData),
	}
}
