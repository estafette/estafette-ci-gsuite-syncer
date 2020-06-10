package main

import (
	"context"
	"io"
	"runtime"

	"github.com/alecthomas/kingpin"
	foundation "github.com/estafette/estafette-foundation"
	"github.com/opentracing/opentracing-go"
	"github.com/rs/zerolog/log"
	"github.com/uber/jaeger-client-go"
	jaegercfg "github.com/uber/jaeger-client-go/config"
)

var (
	appgroup  string
	app       string
	version   string
	branch    string
	revision  string
	buildDate string
	goVersion = runtime.Version()

	// params for apiClient
	apiBaseURL   = kingpin.Flag("api-base-url", "The base url of the estafette-ci-api to communicate with").Envar("API_BASE_URL").Required().String()
	clientID     = kingpin.Flag("client-id", "The id of the client as configured in Estafette, to securely communicate with the api.").Envar("CLIENT_ID").Required().String()
	clientSecret = kingpin.Flag("client-secret", "The secret of the client as configured in Estafette, to securely communicate with the api.").Envar("CLIENT_SECRET").Required().String()

	// params for gsuiteClient
	gsuiteDomain      = kingpin.Flag("gsuite-domain", "The domain used by gsuite.").Envar("GSUITE_DOMAIN").Required().String()
	gsuiteAdminEmail  = kingpin.Flag("gsuite-admin-email", "Email address for gsuite admin user that allowed the service account to impersonate him/her.").Envar("GSUITE_ADMIN_EMAIL").Required().String()
	gsuiteGroupPrefix = kingpin.Flag("gsuite-group-prefix", "The prefix to use for gsuite groups in order to leave alone all non-prefixed groups.").Envar("GSUITE_GROUP_PREFIX").Required().String()
)

func main() {

	// parse command line parameters
	kingpin.Parse()

	// init log format from envvar ESTAFETTE_LOG_FORMAT
	foundation.InitLoggingFromEnv(foundation.NewApplicationInfo(appgroup, app, version, branch, revision, buildDate))

	closer := initJaeger(app)
	defer closer.Close()

	ctx := context.Background()

	span, ctx := opentracing.StartSpanFromContext(ctx, "Main")
	defer span.Finish()

	apiClient := NewApiClient(*apiBaseURL)

	token, err := apiClient.GetToken(ctx, *clientID, *clientSecret)
	if err != nil {
		log.Fatal().Err(err).Msgf("Failed retrieving JWT token")
	}

	organizations, err := apiClient.GetOrganizations(ctx, token)
	if err != nil {
		log.Fatal().Err(err).Msgf("Failed fetching organizations")
	}

	log.Info().Msgf("Fetched %v organizations", len(organizations))

	groups, err := apiClient.GetGroups(ctx, token)
	if err != nil {
		log.Fatal().Err(err).Msgf("Failed fetching groups")
	}

	log.Info().Msgf("Fetched %v groups", len(groups))

	users, err := apiClient.GetUsers(ctx, token)
	if err != nil {
		log.Fatal().Err(err).Msgf("Failed fetching users")
	}

	log.Info().Msgf("Fetched %v users", len(users))

	gsuiteClient, err := NewGsuiteClient(ctx, *gsuiteDomain, *gsuiteAdminEmail, *gsuiteGroupPrefix)
	if err != nil {
		log.Fatal().Err(err).Msgf("Failed creating gsuite client")
	}

	gsuiteOrganizations, err := gsuiteClient.GetOrganizations(ctx)
	if err != nil {
		log.Fatal().Err(err).Msgf("Failed fetching gsuite organizations")
	}

	log.Info().Msgf("Fetched %v gsuite organizations", len(gsuiteOrganizations))

	gsuiteGroups, err := gsuiteClient.GetGroups(ctx)
	if err != nil {
		log.Fatal().Err(err).Msgf("Failed fetching gsuite groups")
	}

	log.Info().Msgf("Fetched %v gsuite groups", len(gsuiteGroups))

	gsuiteGroupMembers, err := gsuiteClient.GetGroupMembers(ctx, gsuiteGroups)
	if err != nil {
		log.Fatal().Err(err).Msgf("Failed fetching gsuite group members")
	}

	for group, members := range gsuiteGroupMembers {
		log.Info().Msgf("Fetched %v gsuite members for group %v", len(members), group.Email)
	}
}

// initJaeger returns an instance of Jaeger Tracer that can be configured with environment variables
// https://github.com/jaegertracing/jaeger-client-go#environment-variables
func initJaeger(service string) io.Closer {

	cfg, err := jaegercfg.FromEnv()
	if err != nil {
		log.Fatal().Err(err).Msg("Generating Jaeger config from environment variables failed")
	}

	closer, err := cfg.InitGlobalTracer(service, jaegercfg.Logger(jaeger.StdLogger))
	if err != nil {
		log.Fatal().Err(err).Msg("Generating Jaeger tracer failed")
	}

	return closer
}
