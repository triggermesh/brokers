package replay

import (
	cloudevents "github.com/cloudevents/sdk-go/v2"
	pkgadapter "knative.dev/eventing/pkg/adapter/v2"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	targetce "github.com/triggermesh/triggermesh/pkg/targets/adapter/cloudevents"
)

// EnvAccessorCtor for configuration parameters
func EnvAccessorCtor() pkgadapter.EnvConfigAccessor {
	return &envAccessor{}
}

type envAccessor struct {
	pkgadapter.EnvConfig
	// BridgeIdentifier is the name of the bridge workflow this target is part of
	BridgeIdentifier string `envconfig:"EVENTS_BRIDGE_IDENTIFIER"`
	// CloudEvents responses parametrization
	CloudEventPayloadPolicy string `envconfig:"EVENTS_PAYLOAD_POLICY" default:"error"`
	// Sink defines the sink to replay events to. Normally this would be pointed
	// to the broker.
	Sink string `envconfig:"K_SINK" required:"true"`
	// Redis database parameters
	RedisAddress  string `envconfig:"REDIS_ADDRESS" required:"true"`
	RedisPassword string `envconfig:"REDIS_PASSWORD" required:"true"`
	StartTime     string `envconfig:"START_TIME"`
	EndTime       string `envconfig:"END_TIME"`
	Filter        string `envconfig:"FILTER"`
	FilterKind    string `envconfig:"FILTER_KIND"`
}

type replayadapter struct {
	sink       string
	replier    *targetce.Replier
	ceClient   cloudevents.Client
	logger     *zap.SugaredLogger
	client     *redis.Client
	key        string
	startTime  string
	endTime    string
	filter     string
	filterKind string
}

// REvent represents the structure of an expected Cloudevent stored
// in Reddis via the XMessage interface.
type REvent struct {
	ID string `json:"ID"`
	Ce []struct {
		Specversion     string `json:"specversion"`
		ID              string `json:"id"`
		Source          string `json:"source"`
		Type            string `json:"type"`
		Datacontenttype string `json:"datacontenttype"`
	} `json:"ce"`
}
