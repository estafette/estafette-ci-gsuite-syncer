package main

import (
	"context"
	"io/ioutil"
	"os"
	"strings"

	"github.com/opentracing/opentracing-go"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	admin "google.golang.org/api/admin/directory/v1"
	crmv1 "google.golang.org/api/cloudresourcemanager/v1"
	iam "google.golang.org/api/iam/v1"
)

type GsuiteClient interface {
	GetOrganizations(ctx context.Context) (organizations []*crmv1.Organization, err error)
	GetGroups(ctx context.Context) (groups []*admin.Group, err error)
	GetGroupMembers(ctx context.Context, groups []*admin.Group) (groupMembers map[*admin.Group][]*admin.Member, err error)
}

// NewGsuiteClient returns a new GsuiteClient
func NewGsuiteClient(ctx context.Context, gsuiteDomain, gsuiteAdminEmail, gsuiteGroupPrefix string) (GsuiteClient, error) {

	// use service account with G Suite Domain-wide Delegation enabled to authenticate against gsuite apis
	serviceAccountKeyFileBytes, err := ioutil.ReadFile(os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"))
	if err != nil {
		return nil, err
	}

	jwtConfig, err := google.JWTConfigFromJSON(serviceAccountKeyFileBytes, admin.AdminDirectoryGroupReadonlyScope, admin.AdminDirectoryGroupMemberReadonlyScope, admin.AdminDirectoryUserReadonlyScope)
	if err != nil {
		return nil, err
	}

	// set subject to user that allowed service account with g-suite delegation to impersonate that user
	jwtConfig.Subject = gsuiteAdminEmail
	googleClientForGSuite := jwtConfig.Client(oauth2.NoContext)

	adminService, err := admin.New(googleClientForGSuite)
	if err != nil {
		return nil, err
	}

	// use service account to authenticate against gcp apis
	googleClient, err := google.DefaultClient(ctx, iam.CloudPlatformScope)
	if err != nil {
		return nil, err
	}

	crmv1Service, err := crmv1.New(googleClient)
	if err != nil {
		return nil, err
	}

	return &gsuiteClient{
		gsuiteDomain:      gsuiteDomain,
		gsuiteGroupPrefix: gsuiteGroupPrefix,
		adminService:      adminService,
		crmv1Service:      crmv1Service,
	}, nil
}

type gsuiteClient struct {
	gsuiteDomain      string
	gsuiteGroupPrefix string
	adminService      *admin.Service
	crmv1Service      *crmv1.Service
}

func (c *gsuiteClient) GetOrganizations(ctx context.Context) (organizations []*crmv1.Organization, err error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "GsuiteClient::GetOrganizations")
	defer span.Finish()

	resp, err := c.crmv1Service.Organizations.Search(&crmv1.SearchOrganizationsRequest{}).Do()
	if err != nil {
		return organizations, err
	}

	organizations = resp.Organizations

	span.LogKV("organizations", len(organizations))

	return organizations, nil
}

func (c *gsuiteClient) GetGroups(ctx context.Context) (groups []*admin.Group, err error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "GsuiteClient::GetGroups")
	defer span.Finish()

	groups = make([]*admin.Group, 0)
	nextPageToken := ""

	for {
		// retrieving groups (by page)
		listCall := c.adminService.Groups.List()
		listCall.Domain(c.gsuiteDomain)
		if nextPageToken != "" {
			listCall.PageToken(nextPageToken)
		}
		resp, err := listCall.Do()
		if err != nil {
			return groups, err
		}

		for _, group := range resp.Groups {
			if strings.HasPrefix(group.Name, c.gsuiteGroupPrefix) {
				groups = append(groups, group)
			}
		}

		if resp.NextPageToken == "" {
			break
		}
		nextPageToken = resp.NextPageToken
	}

	span.LogKV("groups", len(groups))

	return
}

func (c *gsuiteClient) GetGroupMembers(ctx context.Context, groups []*admin.Group) (groupMembers map[*admin.Group][]*admin.Member, err error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "GsuiteClient::GetGroupMembers")
	defer span.Finish()

	groupMembers = map[*admin.Group][]*admin.Member{}

	groupMemberCount := 0

	// http://jmoiron.net/blog/limiting-concurrency-in-go/
	concurrency := 10
	semaphore := make(chan bool, concurrency)

	resultChannel := make(chan struct {
		group   *admin.Group
		members []*admin.Member
		err     error
	}, len(groups))

	for _, group := range groups {
		// try to fill semaphore up to it's full size otherwise wait for a routine to finish
		semaphore <- true

		go func(ctx context.Context, semaphore chan bool, group *admin.Group) {
			// lower semaphore once the routine's finished, making room for another one to start
			defer func() { <-semaphore }()

			members, err := c.getGroupMembersPage(ctx, group)

			resultChannel <- struct {
				group   *admin.Group
				members []*admin.Member
				err     error
			}{group, members, err}
		}(ctx, semaphore, group)
	}

	// try to fill semaphore up to it's full size which only succeeds if all routines have finished
	for i := 0; i < cap(semaphore); i++ {
		semaphore <- true
	}

	close(resultChannel)
	for r := range resultChannel {
		if r.err != nil {
			return groupMembers, err
		}

		groupMembers[r.group] = r.members
		groupMemberCount += len(r.members)
	}

	span.LogKV("groupmembers", groupMemberCount)

	return
}

func (c *gsuiteClient) getGroupMembersPage(ctx context.Context, group *admin.Group) (members []*admin.Member, err error) {
	members = make([]*admin.Member, 0)

	span, ctx := opentracing.StartSpanFromContext(ctx, "GsuiteClient::getGroupMembersPage")
	defer span.Finish()

	nextPageToken := ""
	for {
		// retrieving group members (by page)
		listCall := c.adminService.Members.List(group.Email)
		if nextPageToken != "" {
			listCall.PageToken(nextPageToken)
		}
		resp, err := listCall.Do()
		if err != nil {
			return members, err
		}

		members = append(members, resp.Members...)

		if resp.NextPageToken == "" {
			break
		}
		nextPageToken = resp.NextPageToken
	}

	return members, nil
}
