package main

import (
	"encoding/json"
	"net/http"
)

type AzureMetadataInstanceResponse struct {
	Compute struct {
		Location             string `json:"location"`
		Name                 string `json:"name"`
		Offer                string `json:"offer"`
		OsType               string `json:"osType"`
		PlacementGroupID     string `json:"placementGroupId"`
		PlatformFaultDomain  string `json:"platformFaultDomain"`
		PlatformUpdateDomain string `json:"platformUpdateDomain"`
		Publisher            string `json:"publisher"`
		ResourceGroupName    string `json:"resourceGroupName"`
		Sku                  string `json:"sku"`
		SubscriptionID       string `json:"subscriptionId"`
		Tags                 string `json:"tags"`
		Version              string `json:"version"`
		VMID                 string `json:"vmId"`
		VMSize               string `json:"vmSize"`
	} `json:"compute"`
}

func detectNodeName() string {
	metadata := fetchApiInstanceMetadata()
	return metadata.Compute.Name
}

func fetchApiInstanceMetadata() *AzureMetadataInstanceResponse {
	ret := &AzureMetadataInstanceResponse{}

	req, err := http.NewRequest("GET", opts.InstanceApiUrl, nil)
	if err != nil {
		panic(err)
	}
	req.Header.Add("Metadata", "true")

	resp, err := httpClient.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	err = json.NewDecoder(resp.Body).Decode(&ret)
	if err != nil {
		panic(err)
	}

	return ret
}
