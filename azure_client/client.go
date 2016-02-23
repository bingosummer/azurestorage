package azure_client

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/Azure/azure-sdk-for-go/arm/resources/resources"
	"github.com/Azure/azure-sdk-for-go/arm/storage"
	storageclient "github.com/Azure/azure-sdk-for-go/storage"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/azure"
	"github.com/Azure/go-autorest/autorest/to"
)

type AzureClient struct {
	GroupsClient   *resources.GroupsClient
	AccountsClient *storage.AccountsClient
}

func NewClient() *AzureClient {
	c, err := LoadAzureCredentials()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return nil
	}

	spt, err := NewServicePrincipalTokenFromCredentials(c, azure.AzureResourceManagerScope)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return nil
	}

	rmc := resources.NewGroupsClient(c["subscriptionID"])
	rmc.Authorizer = spt
	rmc.PollingMode = autorest.DoNotPoll

	sac := storage.NewAccountsClient(c["subscriptionID"])
	sac.Authorizer = spt
	sac.PollingMode = autorest.DoNotPoll

	return &AzureClient{
		GroupsClient:   &rmc,
		AccountsClient: &sac,
	}
}

func (c *AzureClient) CreateInstance(resourceGroupName, storageAccountName, location string, accountType storage.AccountType) error {
	err := c.createResourceGroup(resourceGroupName, location)
	if err != nil {
		fmt.Printf("Creating resource group %s failed with error:\n%v\n", resourceGroupName, err)
		return err
	}

	err = c.createStorageAccount(resourceGroupName, storageAccountName, location, accountType)
	if err != nil {
		fmt.Printf("Creating storage account %s.%s failed with error:\n%v\n", resourceGroupName, storageAccountName, err)
		return err
	}

	return nil
}

func (c *AzureClient) GetInstanceState(resourceGroupName, storageAccountName string) (string, string, error) {
	var (
		state       string
		description string
	)

	sa, err := c.AccountsClient.GetProperties(resourceGroupName, storageAccountName)
	if err != nil {
		if strings.Contains(err.Error(), "404") {
			state = "Gone"
			description = "The service instance is gone"
			return state, description, nil
		} else {
			fmt.Printf("Getting instance state failed with error:\n%v\n", err)
			return "", "", err
		}
	}

	provisioningState := sa.Properties.ProvisioningState
	if provisioningState == storage.Creating || provisioningState == storage.ResolvingDNS {
		state = "in progress"
		description = "Creating the service instance, state: " + string(provisioningState)
	} else if provisioningState == storage.Succeeded {
		state = "succeeded"
		description = "Successfully created the service instance, state: " + string(provisioningState)
	} else {
		state = "failed"
		description = "Failed to create the service instance, state: " + string(provisioningState)
	}

	return state, description, nil
}

func (c *AzureClient) GetAccessKeys(resourceGroupName, storageAccountName, containerName, containerAccessType string) (string, string, error) {
	keys, err1 := c.AccountsClient.ListKeys(resourceGroupName, storageAccountName)
	if err1 != nil {
		fmt.Printf("Getting access keys of %s.%s failed with error:\n%v\n", resourceGroupName, storageAccountName, err1)
		return "", "", err1
	}

	err2 := c.createContainer(storageAccountName, to.String(keys.Key1), containerName, containerAccessType)
	if err2 != nil {
		fmt.Printf("Creating storage container %s.%s.%s failed with error:\n%v\n", resourceGroupName, storageAccountName, containerName, err2)
		return "", "", err2
	}

	return to.String(keys.Key1), to.String(keys.Key2), nil
}

func (c *AzureClient) DeleteInstance(resourceGroupName, storageAccountName string) error {
	r, err := c.AccountsClient.Delete(resourceGroupName, storageAccountName)
	if err != nil {
		fmt.Printf("Deleting of %s.%s failed with status %s\n...%v\n", resourceGroupName, storageAccountName, r.Status, err)
		return err
	}
	fmt.Printf("Deleting of %s.%s succeeded\n", resourceGroupName, storageAccountName)
	return nil
}

func (c *AzureClient) RegenerateAccessKeys(resourceGroupName, storageAccountName string) error {
	keyNames := [2]string{"key1", "key2"}
	for _, keyName := range keyNames {
		_, err := c.AccountsClient.RegenerateKey(resourceGroupName, storageAccountName,
			storage.AccountRegenerateKeyParameters{KeyName: to.StringPtr(keyName)})
		if err != nil {
			fmt.Printf("Regenerating access keys of %s.%s failed with error:\n%v\n", resourceGroupName, storageAccountName, err)
			return err
		}
	}

	return nil
}

func (c *AzureClient) createResourceGroup(resourceGroupName, location string) error {
	rg := resources.ResourceGroup{}
	rg.Location = to.StringPtr(location)

	resourceGroup, err := c.GroupsClient.CreateOrUpdate(resourceGroupName, rg)
	if err != nil {
		statusCode := resourceGroup.Response.StatusCode
		if statusCode != http.StatusAccepted && statusCode != http.StatusCreated {
			fmt.Printf("Creating resource group %s failed\n", resourceGroupName)
			return err
		}
	}

	fmt.Printf("Creation initiated %s\n", resourceGroupName)
	return nil
}

func (c *AzureClient) createStorageAccount(resourceGroupName, storageAccountName, location string, accountType storage.AccountType) error {
	cna, err := c.AccountsClient.CheckNameAvailability(
		storage.AccountCheckNameAvailabilityParameters{
			Name: to.StringPtr(storageAccountName),
			Type: to.StringPtr("Microsoft.Storage/storageAccounts")})
	if err != nil {
		fmt.Printf("Error: %v", err)
		return err
	}
	if !to.Bool(cna.NameAvailable) {
		fmt.Printf("%s is unavailable -- try again\n", storageAccountName)
		return errors.New(storageAccountName + " is unavailable")
	}
	fmt.Printf("Storage account name %s is available\n", storageAccountName)

	cp := storage.AccountCreateParameters{}
	cp.Location = to.StringPtr(location)
	cp.Properties = &storage.AccountPropertiesCreateParameters{AccountType: accountType}

	sa, err := c.AccountsClient.Create(resourceGroupName, storageAccountName, cp)
	if err != nil {
		if sa.Response.StatusCode != http.StatusAccepted {
			fmt.Printf("Creation of %s.%s failed", resourceGroupName, storageAccountName)
			return err
		}
	}

	fmt.Printf("Creation initiated %s.%s\n", resourceGroupName, storageAccountName)
	return nil
}

func (c *AzureClient) createContainer(storageAccountName, primaryAccessKey, containerName, containerAccessType string) error {
	storageClient, err := storageclient.NewBasicClient(storageAccountName, primaryAccessKey)
	if err != nil {
		fmt.Println("Creating storage client failed")
		return err
	}

	blobStorageClient := storageClient.GetBlobService()
	_, err = blobStorageClient.CreateContainerIfNotExists(containerName, storageclient.ContainerAccessType(containerAccessType))
	if err != nil {
		fmt.Println("Creating storage container failed")
		return err
	}

	return nil
}
