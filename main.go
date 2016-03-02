package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/Azure/azure-sdk-for-go/arm/storage"
	ac "github.com/bingosummer/azurestorage/azure_client"
	"github.com/bingosummer/azurestorage/model"
	"github.com/bingosummer/azurestorage/utils"
)

var (
	operation     string
	parametersStr string
	instance      model.ServiceInstance
)

const (
	RESOURCE_GROUP_NAME_PREFIX  = "cloud-foundry-"
	STORAGE_ACCOUNT_NAME_PREFIX = "cf"
	CONTAINER_NAME_PREFIX       = "cloud-foundry-"
	LOCATION                    = "eastus"
)

func init() {
	flag.StringVar(&operation, "operation", "", "The operation (Catalog, Provision, Poll, Bind, Unbind, Deprovision) to manage the service instance")
	flag.StringVar(&parametersStr, "parameters", "", "The paramters to manage the service instance")
}

func main() {
	flag.Parse()

	if operation == "" {
		os.Exit(1)
	}

	if operation == "Catalog" {
		bytes, err := utils.ReadFile("catalog.json")
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			panic("Error reading catalog.json...")
		}
		fmt.Println(string(bytes))
		os.Exit(0)
	}

	serviceClient := ac.NewClient()
	if serviceClient == nil {
		panic("Error creating a service client...")
	}

	err := json.Unmarshal([]byte(parametersStr), &instance)

	instanceId := instance.Id
	resourceGroupName := RESOURCE_GROUP_NAME_PREFIX + instanceId
	storageAccountName := STORAGE_ACCOUNT_NAME_PREFIX + strings.Replace(instanceId, "-", "", -1)[0:22]

	if operation == "Provision" {
		location := LOCATION
		accountType := storage.StandardLRS

		err = serviceClient.CreateInstance(resourceGroupName, storageAccountName, location, accountType)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
		}
	} else if operation == "Deprovision" {
		err = serviceClient.DeleteInstance(resourceGroupName, storageAccountName)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
		}
	} else if operation == "Poll" {
		state, description, err := serviceClient.GetInstanceState(resourceGroupName, storageAccountName)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
		}
		lastOperationResponse := model.CreateLastOperationResponse{
			State:       state,
			Description: description,
		}

		response, _ := json.Marshal(lastOperationResponse)
		fmt.Println(string(response))
	} else if operation == "Bind" {
		containerName := CONTAINER_NAME_PREFIX + instanceId
		containerAccessType := "blob"
		key1, key2, err := serviceClient.GetAccessKeys(resourceGroupName, storageAccountName, containerName, containerAccessType)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
		}

		credentials := model.Credentials{
			StorageAccountName: storageAccountName,
			ContainerName:      containerName,
			PrimaryAccessKey:   key1,
			SecondaryAccessKey: key2,
		}

		response, _ := json.Marshal(credentials)
		fmt.Println(string(response))
	} else if operation == "Unbind" {
		err = serviceClient.RegenerateAccessKeys(resourceGroupName, storageAccountName)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
		}
	}
}
