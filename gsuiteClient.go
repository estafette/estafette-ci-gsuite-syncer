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
)

type GsuiteClient interface {
	GetGroups(ctx context.Context) (groups []*admin.Group, err error)
	GetGroupMembers(ctx context.Context, groups []*admin.Group) (groupMembers map[*admin.Group][]*admin.Member, err error)
}

// NewGsuiteClient returns a new GsuiteClient
func NewGsuiteClient(gsuiteDomain, gsuiteAdminEmail, gsuiteGroupPrefix string) (GsuiteClient, error) {

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

	return &gsuiteClient{
		gsuiteDomain:      gsuiteDomain,
		gsuiteGroupPrefix: gsuiteGroupPrefix,
		adminService:      adminService,
	}, nil
}

type gsuiteClient struct {
	gsuiteDomain      string
	gsuiteGroupPrefix string
	adminService      *admin.Service
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

	for _, group := range groups {
		groupMembers[group] = make([]*admin.Member, 0)
		nextPageToken := ""
		for {
			// retrieving group members (by page)
			listCall := c.adminService.Members.List(group.Email)
			if nextPageToken != "" {
				listCall.PageToken(nextPageToken)
			}
			resp, err := listCall.Do()
			if err != nil {
				return groupMembers, err
			}

			for _, member := range resp.Members {
				groupMembers[group] = append(groupMembers[group], member)
			}

			if resp.NextPageToken == "" {
				break
			}
			nextPageToken = resp.NextPageToken
		}

		groupMemberCount += len(groupMembers[group])
	}

	span.LogKV("groupmembers", groupMemberCount)

	return
}
