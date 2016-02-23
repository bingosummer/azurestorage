package model

type ServiceInstance struct {
	Id               string      `json:"id"`
	OrganizationGuid string      `json:"organization_guid"`
	PlanId           string      `json:"plan_id"`
	ServiceId        string      `json:"service_id"`
	SpaceGuid        string      `json:"space_guid"`
	Parameters       interface{} `json:"parameters, omitempty"`
}

type CreateServiceInstanceResponse struct {
       DashboardUrl string `json:"dashboard_url"`
}

type CreateLastOperationResponse struct {
       State       string `json:"state"`
       Description string `json:"description"`
}
