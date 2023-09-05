package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/gin-gonic/gin"
	"github.com/gin-contrib/cors"
)

type Fortune struct {
	Id   int    `json:"id"`
	Name string `json:"name"`
}

func (fortune Fortune) GetKey() map[string]types.AttributeValue {
	id, err := attributevalue.Marshal(fortune.Id)
	if err != nil {
		panic(err)
	}
	return map[string]types.AttributeValue{
		"id": id,
	}
}

type DynamoTable struct {
	DynamoDbClient *dynamodb.Client
	TableName      string
}

func (dynamoTable DynamoTable) TableExists() (bool, error) {
	exists := true
	_, err := dynamoTable.DynamoDbClient.DescribeTable(
		context.TODO(), &dynamodb.DescribeTableInput{TableName: aws.String(dynamoTable.TableName)},
	)
	if err != nil {
		var notFoundEx *types.ResourceNotFoundException
		if errors.As(err, &notFoundEx) {
			log.Printf("Table %v does not exist.\n", dynamoTable.TableName)
		} else {
			log.Printf("Couldn't determine existence of table %v. Here's why: %v\n", dynamoTable.TableName, err)
		}
		exists = false
	}
	return exists, err
}

func (dynamoTable DynamoTable) CreateFortuneTable() (*types.TableDescription, error) {
	var tableDesc *types.TableDescription
	table, err := dynamoTable.DynamoDbClient.CreateTable(context.TODO(), &dynamodb.CreateTableInput{
		AttributeDefinitions: []types.AttributeDefinition{{
			AttributeName: aws.String("id"),
			AttributeType: types.ScalarAttributeTypeN,
		}},
		KeySchema: []types.KeySchemaElement{{
			AttributeName: aws.String("id"),
			KeyType:       types.KeyTypeHash,
		}},
		TableName: aws.String(dynamoTable.TableName),
		ProvisionedThroughput: &types.ProvisionedThroughput{
			ReadCapacityUnits:  aws.Int64(5),
			WriteCapacityUnits: aws.Int64(5),
		},
	})
	if err != nil {
		log.Printf("Couldn't create table %v. Here's why: %v\n", dynamoTable.TableName, err)
	} else {
		waiter := dynamodb.NewTableExistsWaiter(dynamoTable.DynamoDbClient)
		err = waiter.Wait(context.TODO(), &dynamodb.DescribeTableInput{
			TableName: aws.String(dynamoTable.TableName)}, 5*time.Minute)
		if err != nil {
			log.Printf("Wait for table exists failed. Here's why: %v\n", err)
		}
		tableDesc = table.TableDescription
	}
	return tableDesc, err
}

func (dynamoTable DynamoTable) AddFortune(fortune Fortune) error {
	item, err := attributevalue.MarshalMap(fortune)
	if err != nil {
		panic(err)
	}
	_, err = dynamoTable.DynamoDbClient.PutItem(context.TODO(), &dynamodb.PutItemInput{
		TableName: aws.String(dynamoTable.TableName), Item: item,
	})
	if err != nil {
		log.Printf("Couldn't add item to table. Here's why: %v\n", err)
	}
	return err
}

func (dynamoTable DynamoTable) GetFortune(id int) (Fortune, error) {
	fortune := Fortune{Id: id}
	response, err := dynamoTable.DynamoDbClient.GetItem(context.TODO(), &dynamodb.GetItemInput{
		Key: fortune.GetKey(), TableName: aws.String(dynamoTable.TableName),
	})
	if err != nil {
		log.Printf("Couldn't get info about %v. Here's why: %v\n", id, err)
	} else {
		err = attributevalue.UnmarshalMap(response.Item, &fortune)
		if err != nil {
			log.Printf("Couldn't unmarshal response. Here's why: %v\n", err)
		}
	}
	return fortune, err
}

func (dynamoTable DynamoTable) Scan() ([]Fortune, error) {
	var fortunes []Fortune
	var err error
	var response *dynamodb.ScanOutput
	projEx := expression.NamesList(expression.Name("id"), expression.Name("name"))
	expr, err := expression.NewBuilder().WithProjection(projEx).Build()
	if err != nil {
		log.Printf("Couldn't build expressions for scan. Here's why: %v\n", err)
	} else {
		response, err = dynamoTable.DynamoDbClient.Scan(context.TODO(), &dynamodb.ScanInput{
			TableName:                 aws.String(dynamoTable.TableName),
			ExpressionAttributeNames:  expr.Names(),
			ExpressionAttributeValues: expr.Values(),
			FilterExpression:          expr.Filter(),
			ProjectionExpression:      expr.Projection(),
		})
		if err != nil {
			log.Printf("Couldn't scan for fortunes. Here's why: %v\n", err)
		} else {
			err = attributevalue.UnmarshalListOfMaps(response.Items, &fortunes)
			if err != nil {
				log.Printf("Couldn't unmarshal query response. Here's why: %v\n", err)
			}
		}
	}
	return fortunes, err
}

func (dynamoTable DynamoTable) DeleteFortune(fortune Fortune) error {
	_, err := dynamoTable.DynamoDbClient.DeleteItem(context.TODO(), &dynamodb.DeleteItemInput{
		TableName: aws.String(dynamoTable.TableName), Key: fortune.GetKey(),
	})
	if err != nil {
		log.Printf("Couldn't delete %v from the table. Here's why: %v\n", fortune.Name, err)
	}
	return err
}

func (dynamoTable DynamoTable) GetFortuneHandler(c *gin.Context) {
	fortunes, err := dynamoTable.Scan()
	if err != nil {
		log.Printf("Couldn't get fortunes")
	}
	c.IndentedJSON(http.StatusOK, fortunes)
}

func (dynamoTable DynamoTable) GetFortuneByIdHandler(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		log.Printf("Couldn't convert id to integer.")
		c.IndentedJSON(http.StatusNotFound, gin.H{"message": "fortune not found"})
		return
	}
	fortune, err := dynamoTable.GetFortune(id)
	if err != nil {
		log.Printf("Failed to get fortune.")
		c.IndentedJSON(http.StatusNotFound, gin.H{"message": "fortune not found"})
		return
	}
	c.IndentedJSON(http.StatusOK, fortune)
}

func DefaultHandler(c *gin.Context) {
	c.IndentedJSON(http.StatusOK, gin.H{"message": "Hello"})
}

func main() {
	config, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		log.Fatal(err)
	}
	dynamoClient := dynamodb.NewFromConfig(config)
	fortuneTable := DynamoTable{
		DynamoDbClient: dynamoClient,
		TableName:      "fortune-of-the-day",
	}
	exists, err := fortuneTable.TableExists()
	if err != nil {
		if !exists {
			log.Printf("Table 'fortune-of-the-day' not found. Create a new one...\n")
			_, err := fortuneTable.CreateFortuneTable()
			if err != nil {
				log.Fatal(err)
			}
		} else {
			log.Fatal(err)
		}
	}
	router := gin.Default()
	corsConfig := cors.DefaultConfig()
	corsConfig.AllowOrigins = []string{"https://fortune.lnkphm.online"}
	router.Use(cors.New(corsConfig))
	router.GET("/", DefaultHandler)
	router.GET("/fortunes", fortuneTable.GetFortuneHandler)
	router.GET("/fortunes/:id", fortuneTable.GetFortuneByIdHandler)
	router.Run(":5000")
}
