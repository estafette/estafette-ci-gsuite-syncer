package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	contracts "github.com/estafette/estafette-ci-contracts"
	foundation "github.com/estafette/estafette-foundation"
	"github.com/opentracing-contrib/go-stdlib/nethttp"
	"github.com/opentracing/opentracing-go"
	"github.com/rs/zerolog/log"
	"github.com/sethgrid/pester"
	admin "google.golang.org/api/admin/directory/v1"
)

const gsuitProviderName = "gsuite"

type ApiClient interface {
	GetToken(ctx context.Context, clientID, clientSecret string) (token string, err error)
	GetOrganizations(ctx context.Context, token string) (organizations []*contracts.Organization, err error)
	GetGroups(ctx context.Context, token string) (groups []*contracts.Group, err error)
	GetUsers(ctx context.Context, token string) (users []*contracts.User, err error)
	SynchronizeGroupsAndMembers(ctx context.Context, token string, groups []*contracts.Group, users []*contracts.User, gsuiteGroupMembers map[*admin.Group][]*admin.Member) (err error)
}

// NewApiClient returns a new ApiClient
func NewApiClient(apiBaseURL, gsuiteGroupPrefix string) ApiClient {
	return &apiClient{
		apiBaseURL:        apiBaseURL,
		gsuiteGroupPrefix: gsuiteGroupPrefix,
	}
}

type apiClient struct {
	apiBaseURL        string
	gsuiteGroupPrefix string
}

func (c *apiClient) GetToken(ctx context.Context, clientID, clientSecret string) (token string, err error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "ApiClient::GetToken")
	defer span.Finish()

	clientObject := contracts.Client{
		ClientID:     clientID,
		ClientSecret: clientSecret,
	}

	bytes, err := json.Marshal(clientObject)
	if err != nil {
		return
	}

	getTokenURL := fmt.Sprintf("%v/api/auth/client/login", c.apiBaseURL)
	headers := map[string]string{
		"Content-Type": "application/json",
	}

	responseBody, err := c.postRequest(getTokenURL, span, strings.NewReader(string(bytes)), headers)

	tokenResponse := struct {
		Token string `json:"token"`
	}{}

	// unmarshal json body
	err = json.Unmarshal(responseBody, &tokenResponse)
	if err != nil {
		log.Error().Err(err).Str("body", string(responseBody)).Msgf("Failed unmarshalling get token response")
		return
	}

	return tokenResponse.Token, nil
}

func (c *apiClient) GetOrganizations(ctx context.Context, token string) (organizations []*contracts.Organization, err error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "ApiClient::GetOrganizations")
	defer span.Finish()

	pageNumber := 1
	pageSize := 100
	organizations = make([]*contracts.Organization, 0)

	for {
		orgs, pagination, err := c.getOrganizationsPage(ctx, token, pageNumber, pageSize)
		if err != nil {
			return organizations, err
		}
		organizations = append(organizations, orgs...)

		if pagination.TotalPages <= pageNumber {
			break
		}

		pageNumber++
	}

	span.LogKV("organizations", len(organizations))

	return organizations, nil
}

func (c *apiClient) getOrganizationsPage(ctx context.Context, token string, pageNumber, pageSize int) (organizations []*contracts.Organization, pagination contracts.Pagination, err error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "ApiClient::getOrganizationsPage")
	defer span.Finish()

	span.LogKV("page[number]", pageNumber, "page[size]", pageSize)

	getOrganizationsURL := fmt.Sprintf("%v/api/organizations?page[number]=%v&page[size]=%v", c.apiBaseURL, pageNumber, pageSize)
	headers := map[string]string{
		"Authorization": fmt.Sprintf("Bearer %v", token),
		"Content-Type":  "application/json",
	}

	responseBody, err := c.getRequest(getOrganizationsURL, span, nil, headers)

	var listResponse struct {
		Items      []*contracts.Organization `json:"items"`
		Pagination contracts.Pagination      `json:"pagination"`
	}

	// unmarshal json body
	err = json.Unmarshal(responseBody, &listResponse)
	if err != nil {
		log.Error().Err(err).Str("body", string(responseBody)).Msgf("Failed unmarshalling get organizations response")
		return
	}

	organizations = listResponse.Items

	span.LogKV("organizations", len(organizations))

	return organizations, listResponse.Pagination, nil
}

func (c *apiClient) GetGroups(ctx context.Context, token string) (groups []*contracts.Group, err error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "ApiClient::GetGroups")
	defer span.Finish()

	pageNumber := 1
	pageSize := 100
	groups = make([]*contracts.Group, 0)

	for {
		grps, pagination, err := c.getGroupsPage(ctx, token, pageNumber, pageSize)
		if err != nil {
			return groups, err
		}
		groups = append(groups, grps...)

		if pagination.TotalPages <= pageNumber {
			break
		}

		pageNumber++
	}

	span.LogKV("groups", len(groups))

	return groups, nil
}

func (c *apiClient) getGroupsPage(ctx context.Context, token string, pageNumber, pageSize int) (groups []*contracts.Group, pagination contracts.Pagination, err error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "ApiClient::getGroupsPage")
	defer span.Finish()

	span.LogKV("page[number]", pageNumber, "page[size]", pageSize)

	getGroupsURL := fmt.Sprintf("%v/api/groups?page[number]=%v&page[size]=%v", c.apiBaseURL, pageNumber, pageSize)
	headers := map[string]string{
		"Authorization": fmt.Sprintf("Bearer %v", token),
		"Content-Type":  "application/json",
	}

	responseBody, err := c.getRequest(getGroupsURL, span, nil, headers)

	var listResponse struct {
		Items      []*contracts.Group   `json:"items"`
		Pagination contracts.Pagination `json:"pagination"`
	}

	// unmarshal json body
	err = json.Unmarshal(responseBody, &listResponse)
	if err != nil {
		log.Error().Err(err).Str("body", string(responseBody)).Msgf("Failed unmarshalling get organizations response")
		return
	}

	groups = listResponse.Items

	span.LogKV("groups", len(groups))

	return groups, listResponse.Pagination, nil
}

func (c *apiClient) GetUsers(ctx context.Context, token string) (users []*contracts.User, err error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "ApiClient::GetUsers")
	defer span.Finish()

	pageNumber := 1
	pageSize := 100
	users = make([]*contracts.User, 0)

	for {
		usrs, pagination, err := c.getUsersPage(ctx, token, pageNumber, pageSize)
		if err != nil {
			return users, err
		}
		users = append(users, usrs...)

		if pagination.TotalPages <= pageNumber {
			break
		}

		pageNumber++
	}

	span.LogKV("users", len(users))

	return users, nil
}

func (c *apiClient) getUsersPage(ctx context.Context, token string, pageNumber, pageSize int) (users []*contracts.User, pagination contracts.Pagination, err error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "ApiClient::getUsersPage")
	defer span.Finish()

	span.LogKV("page[number]", pageNumber, "page[size]", pageSize)

	getUsersURL := fmt.Sprintf("%v/api/users?page[number]=%v&page[size]=%v", c.apiBaseURL, pageNumber, pageSize)
	headers := map[string]string{
		"Authorization": fmt.Sprintf("Bearer %v", token),
		"Content-Type":  "application/json",
	}

	responseBody, err := c.getRequest(getUsersURL, span, nil, headers)

	var listResponse struct {
		Items      []*contracts.User    `json:"items"`
		Pagination contracts.Pagination `json:"pagination"`
	}

	// unmarshal json body
	err = json.Unmarshal(responseBody, &listResponse)
	if err != nil {
		log.Error().Err(err).Str("body", string(responseBody)).Msgf("Failed unmarshalling get organizations response")
		return
	}

	users = listResponse.Items

	span.LogKV("users", len(users))

	return users, listResponse.Pagination, nil
}

func (c *apiClient) SynchronizeGroupsAndMembers(ctx context.Context, token string, groups []*contracts.Group, users []*contracts.User, gsuiteGroupMembers map[*admin.Group][]*admin.Member) (err error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "ApiClient::SynchronizeGroupsAndMembers")
	defer span.Finish()

	for _, g := range groups {
		hasMatchingGsuiteGroup := false
		for gg := range gsuiteGroupMembers {
			// check estafette group identities for provider gsuite and id equal to gsuite group email address
			for _, i := range g.Identities {
				if i.Provider == gsuitProviderName && i.ID == gg.Email {
					hasMatchingGsuiteGroup = true

					// we have a matching group in estafette, update it
					g.Name = strings.TrimPrefix(gg.Name, c.gsuiteGroupPrefix)
					err = c.updateGroup(ctx, token, g)
					if err != nil {
						return
					}
				}
			}
		}

		if !hasMatchingGsuiteGroup {
			// todo de-activate it??
		}
	}

	for gg, m := range gsuiteGroupMembers {
		hasMatchingEstafetteGroup := false
		for _, g := range groups {
			// check estafette group identities for provider gsuite and id equal to gsuite group email address
			for _, i := range g.Identities {
				if i.Provider == gsuitProviderName && i.ID == gg.Email {
					hasMatchingEstafetteGroup = true
				}
			}
		}

		if !hasMatchingEstafetteGroup && len(m) > 0 {
			// no matching group, create one

			newGroup := &contracts.Group{
				Name: strings.TrimPrefix(gg.Name, c.gsuiteGroupPrefix),
				Identities: []*contracts.GroupIdentity{
					{
						Provider: gsuitProviderName,
						ID:       gg.Email,
						Name:     gg.Name,
					},
				},
			}

			err = c.createGroup(ctx, token, newGroup)
			if err != nil {
				return
			}
		}
	}

	return nil
}

func (c *apiClient) createGroup(ctx context.Context, token string, group *contracts.Group) (err error) {

	span, ctx := opentracing.StartSpanFromContext(ctx, "ApiClient::createGroup")
	defer span.Finish()

	span.LogKV("group.Name", group.Name)

	bytes, err := json.Marshal(group)
	if err != nil {
		return
	}

	createGroupURL := fmt.Sprintf("%v/api/groups", c.apiBaseURL)
	headers := map[string]string{
		"Authorization": fmt.Sprintf("Bearer %v", token),
		"Content-Type":  "application/json",
	}

	_, err = c.postRequest(createGroupURL, span, strings.NewReader(string(bytes)), headers, http.StatusCreated)

	return
}

func (c *apiClient) updateGroup(ctx context.Context, token string, group *contracts.Group) (err error) {

	span, ctx := opentracing.StartSpanFromContext(ctx, "ApiClient::updateGroup")
	defer span.Finish()

	span.LogKV("group.ID", group.ID, "group.Name", group.Name)

	bytes, err := json.Marshal(group)
	if err != nil {
		return
	}

	updateGroupURL := fmt.Sprintf("%v/api/groups/%v", c.apiBaseURL, group.ID)
	headers := map[string]string{
		"Authorization": fmt.Sprintf("Bearer %v", token),
		"Content-Type":  "application/json",
	}

	_, err = c.putRequest(updateGroupURL, span, strings.NewReader(string(bytes)), headers)

	return
}

func (c *apiClient) getRequest(uri string, span opentracing.Span, requestBody io.Reader, headers map[string]string, allowedStatusCodes ...int) (responseBody []byte, err error) {
	return c.makeRequest("GET", uri, span, requestBody, headers, allowedStatusCodes...)
}

func (c *apiClient) postRequest(uri string, span opentracing.Span, requestBody io.Reader, headers map[string]string, allowedStatusCodes ...int) (responseBody []byte, err error) {
	return c.makeRequest("POST", uri, span, requestBody, headers, allowedStatusCodes...)
}

func (c *apiClient) putRequest(uri string, span opentracing.Span, requestBody io.Reader, headers map[string]string, allowedStatusCodes ...int) (responseBody []byte, err error) {
	return c.makeRequest("PUT", uri, span, requestBody, headers, allowedStatusCodes...)
}

func (c *apiClient) deleteRequest(uri string, span opentracing.Span, requestBody io.Reader, headers map[string]string, allowedStatusCodes ...int) (responseBody []byte, err error) {
	return c.makeRequest("DELETE", uri, span, requestBody, headers, allowedStatusCodes...)
}

func (c *apiClient) makeRequest(method, uri string, span opentracing.Span, requestBody io.Reader, headers map[string]string, allowedStatusCodes ...int) (responseBody []byte, err error) {

	// create client, in order to add headers
	client := pester.NewExtendedClient(&http.Client{Transport: &nethttp.Transport{}})
	client.MaxRetries = 3
	client.Backoff = pester.ExponentialJitterBackoff
	client.KeepLog = true
	client.Timeout = time.Second * 10

	request, err := http.NewRequest(method, uri, requestBody)
	if err != nil {
		return nil, err
	}

	// add tracing context
	request = request.WithContext(opentracing.ContextWithSpan(request.Context(), span))

	// collect additional information on setting up connections
	request, ht := nethttp.TraceRequest(span.Tracer(), request)

	// add headers
	for k, v := range headers {
		request.Header.Add(k, v)
	}

	// perform actual request
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	ht.Finish()

	if len(allowedStatusCodes) == 0 {
		allowedStatusCodes = []int{http.StatusOK}
	}

	if !foundation.IntArrayContains(allowedStatusCodes, response.StatusCode) {
		return nil, fmt.Errorf("%v responded with status code %v", uri, response.StatusCode)
	}

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return
	}

	return body, nil
}
