package kafka

type KafkaArgs struct {
	Addresses []string `help:"Kafka addresses." env:"ADDRESSES"`
	Topic     string   `help:"Kafka topic." env:"TOPIC"`

	// Username         string   `help:"Redis username." env:"USERNAME"`
	// Password         string   `help:"Redis password." env:"PASSWORD"`

	GssServiceName        string `help:"GSSAPI service name." env:"GSSAPI_SERVICE_NAME"`
	GssRealm              string `help:"GSSAPI realm." env:"GSSAPI_REALM"`
	GssPrincipal          string `help:"GSSAPI principal." env:"GSSAPI_PRINCIPAL"`
	GssKeyTabPath         string `help:"GSSAPI keytab path." env:"GSSAPI_KEYTAB_PATH"`
	GssKerberosConfigPath string `help:"GSSAPI service name." env:"GSSAPI_KERBEROS_CONFIG_PATH"`

	// TLSEnabled       bool     `help:"TLS enablement for Redis connection." env:"TLS_ENABLED" default:"false"`
	// TLSSkipVerify    bool     `help:"TLS skipping certificate verification." env:"TLS_SKIP_VERIFY" default:"false"`
	// TLSCertificate   string   `help:"TLS Certificate to connect to Redis." env:"TLS_CERTIFICATE"`
	// TLSKey           string   `help:"TLS Certificate key to connect to Redis." env:"TLS_KEY"`
	// TLSCACertificate string   `help:"CA Certificate to connect to Redis." name:"tls-ca-certificate" env:"TLS_CA_CERTIFICATE"`

	ConsumerGroup string `help:"Kafka consumer group name." env:"CONSUMER_GROUP" default:"default"`
	// Instance at the Kafka consumer group. Copied from the InstanceName at the global args.
	Instance string `kong:"-"`

	TrackingIDEnabled bool `help:"Enables adding Kafka Offset as a CloudEvent attribute." env:"TRACKING_ID_ENABLED" default:"false"`
}
