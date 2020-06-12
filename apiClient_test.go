package main

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetToken(t *testing.T) {
	t.Run("ReturnsToken", func(t *testing.T) {

		if testing.Short() {
			t.Skip("skipping test in short mode.")
		}

		ctx := context.Background()
		getBaseURL := os.Getenv("API_BASE_URL")
		clientID := os.Getenv("CLIENT_ID")
		clientSecret := os.Getenv("CLIENT_SECRET")
		client := NewApiClient(getBaseURL, "")

		// act
		token, err := client.GetToken(ctx, clientID, clientSecret)

		assert.Nil(t, err)
		assert.True(t, len(token) > 0)
	})
}

func TestGetOrganizations(t *testing.T) {
	t.Run("ReturnsOrganizations", func(t *testing.T) {

		if testing.Short() {
			t.Skip("skipping test in short mode.")
		}

		ctx := context.Background()
		getBaseURL := os.Getenv("API_BASE_URL")
		clientID := os.Getenv("CLIENT_ID")
		clientSecret := os.Getenv("CLIENT_SECRET")
		client := NewApiClient(getBaseURL, "")
		token, err := client.GetToken(ctx, clientID, clientSecret)
		assert.Nil(t, err)

		//act
		organizations, err := client.GetOrganizations(ctx, token)

		assert.Nil(t, err)
		assert.True(t, len(organizations) > 0)
	})
}

func TestGetGroups(t *testing.T) {
	t.Run("ReturnsGroups", func(t *testing.T) {

		if testing.Short() {
			t.Skip("skipping test in short mode.")
		}

		ctx := context.Background()
		getBaseURL := os.Getenv("API_BASE_URL")
		clientID := os.Getenv("CLIENT_ID")
		clientSecret := os.Getenv("CLIENT_SECRET")
		client := NewApiClient(getBaseURL, "")
		token, err := client.GetToken(ctx, clientID, clientSecret)
		assert.Nil(t, err)

		//act
		groups, err := client.GetGroups(ctx, token)

		assert.Nil(t, err)
		assert.True(t, len(groups) > 0)
	})
}

func TestGetUsers(t *testing.T) {
	t.Run("ReturnsUsers", func(t *testing.T) {

		if testing.Short() {
			t.Skip("skipping test in short mode.")
		}

		ctx := context.Background()
		getBaseURL := os.Getenv("API_BASE_URL")
		clientID := os.Getenv("CLIENT_ID")
		clientSecret := os.Getenv("CLIENT_SECRET")
		client := NewApiClient(getBaseURL, "")
		token, err := client.GetToken(ctx, clientID, clientSecret)
		assert.Nil(t, err)

		//act
		users, err := client.GetUsers(ctx, token)

		assert.Nil(t, err)
		assert.True(t, len(users) > 0)
	})
}
