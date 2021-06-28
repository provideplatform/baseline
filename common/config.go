package common

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	logger "github.com/kthomas/go-logger"
	"github.com/provideplatform/ident/common"
	"github.com/provideplatform/provide-go/api/ident"
	"github.com/provideplatform/provide-go/api/nchain"
	"github.com/provideplatform/provide-go/api/vault"
	"github.com/provideplatform/provide-go/common/util"
)

var (
	// BaselineOrganizationAddress is the baseline organization address
	BaselineOrganizationAddress *string

	// BaselineRegistryContractAddress is a contract address
	BaselineRegistryContractAddress *string

	// BaselineRegistryContract is a compiled contract artifact
	BaselineRegistryContract *nchain.CompiledArtifact

	// ConsumeNATSStreamingSubscriptions is a flag the indicates if the ident instance is running in API or consumer mode
	ConsumeNATSStreamingSubscriptions bool

	// DefaultCounterparties are the default counterparties
	DefaultCounterparties []map[string]string

	// Log is the configured logger
	Log *logger.Logger

	// InternalSOR is the internal system of record
	InternalSOR map[string]interface{}

	// NChainBaselineNetworkID baseline network id
	NChainBaselineNetworkID *string

	// OrganizationID is the id of the org
	OrganizationID *string

	// OrganizationMessagingEndpoint is the public organziation messaging endpoint
	OrganizationMessagingEndpoint *string

	// OrganizationProxyEndpoint is the configured endpoint for the baseline proxy REST API
	OrganizationProxyEndpoint *string

	// OrganizationRefreshToken is the refresh token for the org
	OrganizationRefreshToken *string

	// Vault is the vault instance
	Vault *vault.Vault
)

func init() {
	requireLogger()

	requireOrganization()
	requireVault()

	requireInternalSOR()
	requireBaseline()

	ConsumeNATSStreamingSubscriptions = strings.ToLower(os.Getenv("CONSUME_NATS_STREAMING_SUBSCRIPTIONS")) == "true"
}

func requireLogger() {
	lvl := os.Getenv("LOG_LEVEL")
	if lvl == "" {
		lvl = "INFO"
	}

	var endpoint *string
	if os.Getenv("SYSLOG_ENDPOINT") != "" {
		endpt := os.Getenv("SYSLOG_ENDPOINT")
		endpoint = &endpt
	}

	Log = logger.NewLogger("baseline", lvl, endpoint)
}

func requireBaseline() {
	if os.Getenv("BASELINE_ORGANIZATION_ADDRESS") == "" {
		Log.Warningf("BASELINE_ORGANIZATION_ADDRESS not provided")
	}
	BaselineOrganizationAddress = common.StringOrNil(os.Getenv("BASELINE_ORGANIZATION_ADDRESS"))

	if os.Getenv("BASELINE_REGISTRY_CONTRACT_ADDRESS") == "" {
		Log.Warningf("BASELINE_REGISTRY_CONTRACT_ADDRESS not provided")
	}
	BaselineRegistryContractAddress = common.StringOrNil(os.Getenv("BASELINE_REGISTRY_CONTRACT_ADDRESS"))

	if os.Getenv("NCHAIN_BASELINE_NETWORK_ID") == "" {
		Log.Warningf("NCHAIN_BASELINE_NETWORK_ID not provided")
	}
	NChainBaselineNetworkID = common.StringOrNil(os.Getenv("NCHAIN_BASELINE_NETWORK_ID"))

	ResolveBaselineContract()
}

// FIXME -- return error
func ResolveBaselineContract() {
	if NChainBaselineNetworkID == nil || OrganizationRefreshToken == nil {
		Log.Warning("unable to resolve baseline contract without configured network id and organization refresh token")
		return
	}

	capabilities, err := util.ResolveCapabilitiesManifest()
	if baseline, baselineOk := capabilities["baseline"].(map[string]interface{}); baselineOk {
		if contracts, contractsOk := baseline["contracts"].([]interface{}); contractsOk {
			for _, contract := range contracts {
				if name, nameOk := contract.(map[string]interface{})["name"].(string); nameOk && strings.ToLower(name) == "orgregistry" {
					raw, _ := json.Marshal(contract)
					err := json.Unmarshal(raw, &BaselineRegistryContract)
					if err != nil {
						Log.Warningf("failed to parse registry contract from capabilities; %s", err.Error())
					} else {
						Log.Debug("resolved baseline registry contract artifact")
					}
				}
			}
		}
	}

	if BaselineRegistryContract == nil {
		Log.Warning("failed to parse registry contract from capabilities")
		return
	}

	token, err := ident.CreateToken(*OrganizationRefreshToken, map[string]interface{}{
		"grant_type":      "refresh_token",
		"organization_id": *OrganizationID,
	})
	if err != nil {
		Log.Warningf("failed to vend organization access token; %s", err.Error())
		return
	}

	contract, err := nchain.GetContractDetails(*token.AccessToken, *BaselineRegistryContractAddress, map[string]interface{}{})
	if err != nil || contract == nil {
		wallet, err := nchain.CreateWallet(*token.AccessToken, map[string]interface{}{
			"purpose": 44,
		})
		if err != nil {
			Log.Warningf("failed to initialize wallet for organization; %s", err.Error())
		} else {
			Log.Debugf("created HD wallet for organization: %s", wallet.ID)
		}

		cntrct, err := nchain.CreateContract(*token.AccessToken, map[string]interface{}{
			"address":    *BaselineRegistryContractAddress,
			"name":       BaselineRegistryContract.Name,
			"network_id": NChainBaselineNetworkID,
			"params": map[string]interface{}{
				"argv":              []interface{}{},
				"compiled_artifact": BaselineRegistryContract,
				"wallet_id":         wallet.ID,
			},
			"type": "organization-registry",
		})
		if err != nil {
			Log.Warningf("failed to initialize registry contract; %s", err.Error())
		} else {
			Log.Debugf("resolved baseline organization registry contract: %s", *cntrct.Address)
		}
	} else {
		Log.Debugf("resolved baseline organization registry contract: %s", *contract.Address)
	}
}

func requireInternalSOR() {
	if os.Getenv("PROVIDE_SOR_IDENTIFIER") == "" {
		Log.Warningf("PROVIDE_SOR_IDENTIFIER not provided")
	}

	if os.Getenv("PROVIDE_SOR_URL") == "" {
		Log.Warningf("PROVIDE_SOR_URL not provided")
	}

	InternalSOR = map[string]interface{}{
		"identifier": os.Getenv("PROVIDE_SOR_IDENTIFIER"),
	}

	if os.Getenv("PROVIDE_SOR_URL") != "" && os.Getenv("PROVIDE_SOR_URL") != "https://" {
		InternalSOR["url"] = os.Getenv("PROVIDE_SOR_URL")
	}

	if os.Getenv("PROVIDE_SOR_ORGANIZATION_CODE") != "" {
		InternalSOR["organization_code"] = os.Getenv("PROVIDE_SOR_ORGANIZATION_CODE")
	}
}

func requireOrganization() {
	if os.Getenv("PROVIDE_ORGANIZATION_ID") == "" {
		Log.Warningf("PROVIDE_ORGANIZATION_ID not provided")
	}
	OrganizationID = StringOrNil(os.Getenv("PROVIDE_ORGANIZATION_ID"))

	if os.Getenv("PROVIDE_ORGANIZATION_REFRESH_TOKEN") == "" {
		Log.Warningf("PROVIDE_ORGANIZATION_REFRESH_TOKEN not provided")
	}
	OrganizationRefreshToken = StringOrNil(os.Getenv("PROVIDE_ORGANIZATION_REFRESH_TOKEN"))

	OrganizationMessagingEndpoint = StringOrNil(os.Getenv("BASELINE_ORGANIZATION_MESSAGING_ENDPOINT"))
	if OrganizationMessagingEndpoint == nil {
		Log.Warningf("BASELINE_ORGANIZATION_MESSAGING_ENDPOINT not provided")
	}

	OrganizationProxyEndpoint = StringOrNil(os.Getenv("BASELINE_ORGANIZATION_PROXY_ENDPOINT"))
	if OrganizationProxyEndpoint == nil {
		Log.Panicf("BASELINE_ORGANIZATION_PROXY_ENDPOINT is required")
	}
}

func requireVault() {
	util.RequireVault()

	vaults, err := vault.ListVaults(util.DefaultVaultAccessJWT, map[string]interface{}{})
	if err != nil {
		Log.Panicf("failed to fetch vaults for given token; %s", err.Error())
	}

	if len(vaults) > 0 {
		// HACK
		Vault = vaults[0]
		Log.Debugf("resolved default vault instance for proxy: %s", Vault.ID.String())
	} else {
		Vault, err = vault.CreateVault(util.DefaultVaultAccessJWT, map[string]interface{}{
			"name":        fmt.Sprintf("nchain vault %d", time.Now().Unix()),
			"description": "default organizational keystore",
		})
		if err != nil {
			Log.Panicf("failed to create default vaults for proxy instance; %s", err.Error())
		}
		Log.Debugf("created default vault instance for proxy: %s", Vault.ID.String())
	}
}
