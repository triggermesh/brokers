package kafka

import (
	"errors"
	"fmt"
	"strings"
)

type KafkaArgs struct {
	Addresses []string `help:"Kafka addresses." env:"ADDRESSES"`
	Topic     string   `help:"Kafka topic." env:"TOPIC" default:"triggermesh"`

	// Username         string   `help:"Redis username." env:"USERNAME"`
	// Password         string   `help:"Redis password." env:"PASSWORD"`

	GssServiceName        string `help:"GSSAPI service name." env:"GSSAPI_SERVICE_NAME"`
	GssRealm              string `help:"GSSAPI realm." env:"GSSAPI_REALM"`
	GssPrincipal          string `help:"GSSAPI principal." env:"GSSAPI_PRINCIPAL"`
	GssKeyTabPath         string `help:"GSSAPI keytab path." name:"gss-keytab-path" env:"GSSAPI_KEYTAB_PATH"`
	GssKerberosConfigPath string `help:"GSSAPI service name." env:"GSSAPI_KERBEROS_CONFIG_PATH"`

	// TLSEnabled       bool     `help:"TLS enablement for Redis connection." env:"TLS_ENABLED" default:"false"`
	// TLSSkipVerify    bool     `help:"TLS skipping certificate verification." env:"TLS_SKIP_VERIFY" default:"false"`
	// TLSCertificate   string   `help:"TLS Certificate to connect to Redis." env:"TLS_CERTIFICATE"`
	// TLSKey           string   `help:"TLS Certificate key to connect to Redis." env:"TLS_KEY"`
	// TLSCACertificate string   `help:"CA Certificate to connect to Redis." name:"tls-ca-certificate" env:"TLS_CA_CERTIFICATE"`

	ConsumerGroupPrefix string `help:"Kafka consumer group name." env:"CONSUMER_GROUP_PREFIX" default:"default"`
	// Instance at the Kafka consumer group. Copied from the InstanceName at the global args.
	Instance string `kong:"-"`

	TrackingIDEnabled bool `help:"Enables adding Kafka Offset as a CloudEvent attribute." env:"TRACKING_ID_ENABLED" default:"false"`
}

func (ka *KafkaArgs) Validate() error {
	msg := []string{}

	// Since there is a default value at addresses, we only check that cluster addresses and a value for
	// and standalone instance that is different to the default must not be provided.
	if len(ka.Addresses) == 0 {
		msg = append(msg, "At least one Kafka broker address must be provided.")
	}

	if _, err := ka.IsGSSAPI(); err != nil {
		msg = append(msg, err.Error())
	}

	if len(msg) == 0 {
		return nil
	}

	return fmt.Errorf(strings.Join(msg, " "))
}

func (ka *KafkaArgs) IsGSSAPI() (bool, error) {
	if ka.GssServiceName == "" && ka.GssRealm == "" && ka.GssPrincipal == "" &&
		ka.GssKeyTabPath == "" && ka.GssKerberosConfigPath == "" {
		return false, nil
	}

	if ka.GssServiceName == "" || ka.GssRealm == "" ||
		ka.GssPrincipal == "" || ka.GssKerberosConfigPath == "" {
		return false, errors.New("incomplete configuration for GSSAPI")
	}

	if ka.GssKeyTabPath == "" {
		return false, errors.New("incomplete authentication information for GSSAPI")
	}

	return true, nil

}
