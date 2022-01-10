package baseline

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dgrijalva/jwt-go"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/gin-gonic/gin"
	"github.com/jinzhu/gorm"
	dbconf "github.com/kthomas/go-db-config"
	natsutil "github.com/kthomas/go-natsutil"
	uuid "github.com/kthomas/go.uuid"
	"github.com/provideplatform/baseline/common"
	"github.com/provideplatform/provide-go/api/baseline"
	"github.com/provideplatform/provide-go/api/ident"
	provide "github.com/provideplatform/provide-go/common"
	"github.com/provideplatform/provide-go/common/util"
)

// InstallBPIAPI installs public API for interacting with the baseline protocol abstraction
// layer, i.e., with `Subject`, `SubjectContext` and `BPIAccount`
func InstallBPIAPI(r *gin.Engine) {
	r.GET("/api/v1/bpi_accounts", listBPIAccountsHandler)
	r.GET("/api/v1/bpi_accounts/:id", bpiAccountDetailsHandler)
	r.POST("/api/v1/bpi_accounts", createBPIAccountHandler)

	r.POST("/api/v1/protocol_messages", createProtocolMessageHandler)

	r.GET("/api/v1/subjects", listSubjectsHandler)
	r.GET("/api/v1/subjects/:id", subjectDetailsHandler)
	r.POST("/api/v1/subjects", createSubjectHandler)
	r.PUT("/api/v1/subjects/:id", updateSubjectHandler)

	r.GET("/api/v1/subjects/:id/accounts", listSubjectAccountsHandler)
	r.GET("/api/v1/subjects/:id/accounts/:accountId", subjectAccountDetailsHandler)
	r.POST("/api/v1/subjects/:id/accounts", createSubjectAccountsHandler)
	r.PUT("/api/v1/subjects/:id/accounts/:accountId", updateSubjectAccountsHandler)
}

// InstallConfigAPI installs public API for interacting with the local baseline stack config
func InstallConfigAPI(r *gin.Engine) {
	r.GET("/api/v1/config", configDetailsHandler)
	r.PUT("/api/v1/config", updateConfigHandler)
}

// InstallCredentialsAPI installs public API for interacting with verifiable credentials
func InstallCredentialsAPI(r *gin.Engine) {
	r.POST("/api/v1/credentials", issueVerifiableCredentialHandler)
}

// InstallMappingsAPI installs mapping management APIs
func InstallMappingsAPI(r *gin.Engine) {
	r.GET("/api/v1/mappings", listMappingsHandler)
	r.POST("/api/v1/mappings", createMappingHandler)
	r.PUT("/api/v1/mappings/:id", updateMappingHandler)
	r.DELETE("/api/v1/mappings/:id", deleteMappingHandler)
}

// InstallObjectsAPI installs system of record proxy objects API
func InstallObjectsAPI(r *gin.Engine) {
	r.POST("/api/v1/objects", createObjectHandler)
	r.PUT("/api/v1/objects/:id", updateObjectHandler)

	// remain backward compatible for now...
	r.POST("/api/v1/business_objects", createObjectHandler)
	r.PUT("/api/v1/business_objects/:id", updateObjectHandler)
}

// InstallPublicWorkgroupAPI installs an API servicing a configured public workgroup
func InstallPublicWorkgroupAPI(r *gin.Engine) {
	r.POST("/api/v1/pub/invite", createPublicWorkgroupInviteHandler)
}

// InstallWorkgroupsAPI installs workgroup management APIs
func InstallWorkgroupsAPI(r *gin.Engine) {
	r.GET("/api/v1/workgroups", listWorkgroupsHandler)
	r.GET("/api/v1/workgroups/:id", workgroupDetailsHandler)
	r.POST("/api/v1/workgroups", createWorkgroupHandler)
}

// InstallWorkflowsAPI installs workflow management APIs
func InstallWorkflowsAPI(r *gin.Engine) {
	r.GET("/api/v1/workflows", listWorkflowsHandler)
	r.GET("/api/v1/workflows/:id", workflowDetailsHandler)
	r.POST("/api/v1/workflows", createWorkflowHandler)
	r.PUT("/api/v1/workflows/:id", updateWorkflowHandler)
	r.POST("/api/v1/workflows/:id/deploy", deployWorkflowHandler)
	r.GET("/api/v1/workflows/:id/versions", listWorkflowVersionsHandler)
	r.POST("/api/v1/workflows/:id/versions", versionWorkflowHandler)
	r.DELETE("/api/v1/workflows/:id", deleteWorkflowHandler)
}

// InstallWorkstepsAPI installs workstep management APIs
func InstallWorkstepsAPI(r *gin.Engine) {
	r.GET("/api/v1/worksteps", listWorkstepsHandler)
	r.GET("/api/v1/workflows/:id/worksteps", listWorkstepsHandler)
	r.GET("/api/v1/workflows/:id/worksteps/:workstepId", workstepDetailsHandler)
	r.POST("/api/v1/workflows/:id/worksteps", createWorkstepHandler)
	r.PUT("/api/v1/workflows/:id/worksteps/:workstepId", updateWorkstepHandler)
	r.DELETE("/api/v1/workflows/:id/worksteps/:workstepId", deleteWorkstepHandler)
	r.POST("/api/v1/workflows/:id/worksteps/:workstepId/execute", executeWorkstepHandler)
	// r.GET("/api/v1/workflows/:id/worksteps/:workstepId/participants", listWorkstepParticipantsHandler)
	// r.POST("/api/v1/workflows/:id/worksteps/:workstepId/participants", createWorkstepParticipantHandler)
	// r.DELETE("/api/v1/workflows/:id/worksteps/:workstepId/participants/:participantId", deleteWorkstepParticipantHandler)
}

func configDetailsHandler(c *gin.Context) {
	if common.OrganizationID == nil {
		provide.RenderError("config not initialized", 404, c)
		return
	}

	organizationID := util.AuthorizedSubjectID(c, "organization")
	if organizationID != nil && organizationID.String() != *common.OrganizationID {
		provide.RenderError("forbidden", 403, c)
		return
	}

	// TODO-- implement this pattern and refactor common vars into common.Config
	// raw := json.Marshal(common.Config)
	// var cfg *Config
	// err := json.Unmarshal(raw, &cfg)

	var workgroupUUID, organizationUUID, networkUUID *uuid.UUID
	if common.WorkgroupID != nil {
		id := uuid.FromStringOrNil(*common.WorkgroupID)
		workgroupUUID = &id
	}
	if common.OrganizationID != nil {
		id := uuid.FromStringOrNil(*common.OrganizationID)
		organizationUUID = &id
	}
	if common.NChainBaselineNetworkID != nil {
		id := uuid.FromStringOrNil(*common.NChainBaselineNetworkID)
		networkUUID = &id
	}

	cfg := &baseline.Config{
		WorkgroupID:              workgroupUUID,
		NetworkID:                networkUUID,
		OrganizationAddress:      common.BaselineOrganizationAddress,
		OrganizationID:           organizationUUID,
		OrganizationRefreshToken: common.OrganizationRefreshToken,
		RegistryContractAddress:  common.BaselineRegistryContractAddress,
	}

	// FIXME
	// if organizationID == nil {
	// 	cfg.OrganizationRefreshToken = nil
	// }

	provide.Render(cfg, 200, c)
}

func updateConfigHandler(c *gin.Context) {
	organizationID := util.AuthorizedSubjectID(c, "organization")

	// TODO: KT
	if common.OrganizationID == nil && organizationID != nil {
		common.OrganizationID = common.StringOrNil(organizationID.String())
		common.Log.Debugf("previously unset organization id initialized by bearer: %s", organizationID)
	}

	if organizationID == nil {
		provide.RenderError("unauthorized", 401, c)
		return
	} else if common.OrganizationID != nil && organizationID.String() != *common.OrganizationID {
		provide.RenderError("forbidden", 403, c)
		return
	}

	buf, err := c.GetRawData()
	if err != nil {
		provide.RenderError(err.Error(), 400, c)
		return
	}

	cfg := &Config{}
	err = json.Unmarshal(buf, cfg)
	if err != nil {
		provide.RenderError(err.Error(), 422, c)
		return
	}

	if cfg.OrganizationID != nil && cfg.OrganizationID.String() != organizationID.String() {
		provide.RenderError("forbidden", 403, c)
		return
	}

	if cfg.apply() {
		provide.Render(nil, 204, c)
	} else {
		obj := map[string]interface{}{}
		obj["errors"] = cfg.Errors
		provide.Render(obj, 422, c)
	}
}

func createObjectHandler(c *gin.Context) {
	organizationID := util.AuthorizedSubjectID(c, "organization")
	if organizationID == nil {
		provide.RenderError("unauthorized", 401, c)
		return
	} else if common.OrganizationID != nil && organizationID.String() != *common.OrganizationID {
		provide.RenderError("forbidden", 403, c)
		return
	}

	buf, err := c.GetRawData()
	if err != nil {
		provide.RenderError(err.Error(), 400, c)
		return
	}

	message := &Message{}
	err = json.Unmarshal(buf, message)
	if err != nil {
		provide.RenderError(err.Error(), 422, c)
		return
	}

	if message.baselineOutbound() {
		message.ProtocolMessage.Payload.Object = nil
		provide.Render(message.ProtocolMessage, 202, c)
	} else {
		obj := map[string]interface{}{}
		obj["errors"] = message.Errors
		provide.Render(obj, 422, c)
	}
}

func updateObjectHandler(c *gin.Context) {
	organizationID := util.AuthorizedSubjectID(c, "organization")
	if organizationID == nil {
		provide.RenderError("unauthorized", 401, c)
		return
	} else if common.OrganizationID != nil && organizationID.String() != *common.OrganizationID {
		provide.RenderError("forbidden", 403, c)
		return
	}

	buf, err := c.GetRawData()
	if err != nil {
		provide.RenderError(err.Error(), 400, c)
		return
	}

	record := lookupBaselineRecordByInternalID(c.Param("id"))
	if record == nil {
		provide.RenderError("baseline record not found", 404, c)
		return
	}

	message := &Message{}
	err = json.Unmarshal(buf, message)
	if err != nil {
		provide.RenderError(err.Error(), 422, c)
		return
	}

	if message.baselineOutbound() {
		message.ProtocolMessage.Payload.Object = nil
		provide.Render(message.ProtocolMessage, 202, c)
	} else {
		obj := map[string]interface{}{}
		obj["errors"] = message.Errors
		provide.Render(obj, 422, c)
	}
}

func createProtocolMessageHandler(c *gin.Context) {
	provide.RenderError("not implemented", 501, c)
}

func createWorkgroupHandler(c *gin.Context) {
	organizationID := util.AuthorizedSubjectID(c, "organization")
	if organizationID == nil {
		provide.RenderError("unauthorized", 401, c)
		return
	} else if common.OrganizationID != nil && organizationID.String() != *common.OrganizationID {
		provide.RenderError("forbidden", 403, c)
		return
	}

	var params map[string]interface{}

	buf, err := c.GetRawData()
	if err != nil {
		provide.RenderError(err.Error(), 400, c)
		return
	}

	err = json.Unmarshal(buf, &params)
	if err != nil {
		msg := fmt.Sprintf("failed to umarshal workgroup invitation acceptance request; %s", err.Error())
		common.Log.Warning(msg)
		provide.RenderError(msg, 422, c)
		return
	}

	if params["token"] == nil {
		provide.RenderError("workgroup invitation token is required", 400, c)
		return
	}

	bearerToken := c.Param("token")

	token, err := jwt.Parse(bearerToken, func(_jwtToken *jwt.Token) (interface{}, error) {
		var kid *string
		if kidhdr, ok := _jwtToken.Header["kid"].(string); ok {
			kid = &kidhdr
		}

		publicKey, _, _, _ := util.ResolveJWTKeypair(kid)
		if publicKey == nil {
			msg := "failed to resolve a valid JWT verification key"
			if kid != nil {
				msg = fmt.Sprintf("%s; invalid kid specified in header: %s", msg, *kid)
			} else {
				msg = fmt.Sprintf("%s; no default verification key configured", msg)
			}
			return nil, fmt.Errorf(msg)
		}

		return publicKey, nil
	})

	if err != nil {
		msg := fmt.Sprintf("failed to accept workgroup invitation; failed to parse jwt; %s", err.Error())
		common.Log.Warningf(msg)
		provide.RenderError(msg, 422, c)
		return
	}

	claims := token.Claims.(jwt.MapClaims)
	// prvd := claims["prvd"].(map[string]interface{})
	// data := prvd["data"].(map[string]interface{})
	baselineClaim := claims["baseline"].(map[string]interface{})

	var identifier *string
	if id, identifierOk := baselineClaim["workgroup_id"].(string); identifierOk {
		identifier = common.StringOrNil(id)
	}

	identifierUUID, err := uuid.FromString(*identifier)
	if err != nil {
		msg := fmt.Sprintf("failed to accept workgroup invitation; invalid identifier; %s", err.Error())
		common.Log.Warningf(msg)
		provide.RenderError(msg, 422, c)
		return
	}

	var invitorAddress *string
	if addr, invitorAddressOk := baselineClaim["invitor_organization_address"].(string); invitorAddressOk {
		invitorAddress = common.StringOrNil(addr)
	} else {
		msg := "no invitor address provided in vc"
		common.Log.Warningf(msg)
		provide.RenderError(msg, 422, c)
		return
	}

	var registryContractAddress *string
	if addr, registryContractAddressOk := baselineClaim["registry_contract_address"].(string); registryContractAddressOk {
		registryContractAddress = common.StringOrNil(addr)
	} else {
		msg := fmt.Sprintf("no registry contract address provided by invitor: %s", *invitorAddress)
		common.Log.Warningf(msg)
		provide.RenderError(msg, 422, c)
		return
	}

	if registryContractAddress == nil || *registryContractAddress != *common.BaselineRegistryContractAddress {
		msg := fmt.Sprintf("given registry contract address (%s) did not match configured address (%s)", *invitorAddress, *common.BaselineRegistryContractAddress)
		common.Log.Warningf(msg)
		provide.RenderError(msg, 422, c)
		return
	}

	var vc *string
	if bearerToken, bearerTokenOk := params["authorized_bearer_token"].(string); bearerTokenOk {
		vc = common.StringOrNil(bearerToken)
	}

	invitor := &Participant{
		baseline.Participant{
			Address: invitorAddress,
		},
		invitorAddress,
		make([]*Workgroup, 0),
		make([]*Workflow, 0),
		make([]*Workstep, 0),
	}
	invitor.Cache()

	participants := make([]*Participant, 0)
	participants = append(participants, invitor)

	err = CacheBaselineOrganizationIssuedVC(*invitorAddress, *vc)
	if err != nil {
		msg := fmt.Sprintf("failed to cache organization-issued vc; %s", err.Error())
		common.Log.Warningf(msg)
		provide.RenderError(msg, 422, c)
		return
	}

	// workgroup := &proxy.Workgroup{
	// 	Identifier:   &identifierUUID,
	// 	Participants: participants,
	// }

	// err = workgroup.Cache()
	// if err != nil {
	// 	msg := fmt.Sprintf("failed to accept workgroup invitation; failed to cache workflow; %s", err.Error())
	// 	common.Log.Warningf(msg)
	// 	provide.RenderError(msg, 422, c)
	// 	return
	// }

	// FIXME-- ensure org registry and shield is available via nchain...

	// 	'organization-registry': {
	// 	  address: invite.prvd.data.params.organization_registry_contract_address,
	// 	  name: 'OrgRegistry',
	// 	  network_id: this.baselineConfig?.networkId,
	// 	  params: {
	// 		compiled_artifact: contracts['organization-registry'].params?.compiled_artifact
	// 	  },
	// 	  type: 'organization-registry',
	// 	},
	// 	'shield': {
	// 	  address: invite.prvd.data.params.shield_contract_address,
	// 	  name: 'Shield',
	// 	  network_id: this.baselineConfig?.networkId,
	// 	  params: {
	// 		compiled_artifact: contracts['shield'].params?.compiled_artifact
	// 	  },
	// 	  type: 'shield',
	// 	},

	//   await this.registerOrganization(this.baselineConfig.orgName, this.natsConfig.natsServers[0]);
	// async registerOrganization(name: string, messagingEndpoint: string): Promise<any> {
	// 	this.org = await this.baseline?.createOrganization({
	// 	  name: name,
	// 	  metadata: {
	// 		messaging_endpoint: messagingEndpoint,
	// 	  },
	// 	});

	// 	if (this.org) {
	// 	  const vault = await this.requireVault();
	// 	  this.babyJubJub = await this.createVaultKey(vault.id!, 'babyJubJub');
	// 	  await this.createVaultKey(vault.id!, 'secp256k1');
	// 	  this.hdwallet = await this.createVaultKey(vault.id!, 'BIP39');
	// 	  await this.registerWorkgroupOrganization();
	// 	}

	// 	return this.org;
	//   }

	var authorizedVC *string // TODO: vend NATS bearer token

	msg := &ProtocolMessage{
		baseline.ProtocolMessage{
			Opcode:     common.StringOrNil(baseline.ProtocolMessageOpcodeJoin),
			Identifier: &identifierUUID,
			Payload: &baseline.ProtocolMessagePayload{
				Object: map[string]interface{}{
					"address":                 *common.BaselineOrganizationAddress,
					"authorized_bearer_token": authorizedVC,
				},
			},
		},
	}
	payload, _ := json.Marshal(msg)

	common.Log.Debugf("attempting to broadcast %d-byte protocol message", len(payload))
	_, err = natsutil.NatsJetstreamPublish(natsDispatchProtocolMessageSubject, payload)

	if err == nil {
		provide.Render(nil, 204, c)
	} else {
		obj := map[string]interface{}{}
		obj["errors"] = []interface{}{} // FIXME
		provide.Render(obj, 422, c)
	}
}

func listWorkgroupsHandler(c *gin.Context) {
	organizationID := util.AuthorizedSubjectID(c, "organization")
	if organizationID == nil {
		provide.RenderError("unauthorized", 401, c)
		return
	} else if common.OrganizationID != nil && organizationID.String() != *common.OrganizationID {
		provide.RenderError("forbidden", 403, c)
		return
	}

	// token, _ := util.ParseBearerAuthorizationHeader(c, nil)
	// resp, err := ident.ListApplications(token.Raw, map[string]interface{}{})

	// if err == nil {
	// 	provide.Render(resp, 200, c)
	// } else {
	// 	provide.RenderError(fmt.Sprintf("failed to list workgroups; %s", err.Error()), 500, c)
	// }

	var workgroups []*Workgroup

	db := dbconf.DatabaseConnection()
	var query *gorm.DB

	if query == nil {
		query = db.Order("workgroups.created_at DESC")
	} else {
		query = query.Order("workgroups.created_at DESC")
	}

	provide.Paginate(c, query, &Workflow{}).Find(&workgroups)
	provide.Render(workgroups, 200, c)
}

func workgroupDetailsHandler(c *gin.Context) {
	organizationID := util.AuthorizedSubjectID(c, "organization")
	if organizationID == nil {
		provide.RenderError("unauthorized", 401, c)
		return
	} else if common.OrganizationID != nil && organizationID.String() != *common.OrganizationID {
		provide.RenderError("forbidden", 403, c)
		return
	}

	workgroup := LookupBaselineWorkgroup(c.Param("id"))

	if workgroup != nil {
		provide.Render(workgroup, 200, c)
	} else {
		provide.RenderError("workgroup not found", 404, c)
	}
}

func createPublicWorkgroupInviteHandler(c *gin.Context) {
	if common.BaselinePublicWorkgroupID == nil {
		provide.RenderError("no public workgroup configured", 501, c)
		return
	}

	token, err := common.RefreshPublicWorkgroupAccessToken()
	if err != nil {
		msg := fmt.Sprintf("failed to authorize public workgroup access token; %s", err.Error())
		common.Log.Warningf(msg)

		provide.RenderError(msg, 500, c)
		return
	}

	buf, err := c.GetRawData()
	if err != nil {
		provide.RenderError(err.Error(), 400, c)
		return
	}

	params := &baseline.PublicWorkgroupInvitationRequest{}
	err = json.Unmarshal(buf, &params)
	if err != nil {
		provide.RenderError(err.Error(), 422, c)
		return
	}

	ident.CreateInvitation(*token, map[string]interface{}{
		"email":      params.Email,
		"first_name": params.FirstName,
		"last_name":  params.LastName,
		"params": map[string]interface{}{
			"organization_name": params.OrganizationName,
		},
	})
}

func listMappingsHandler(c *gin.Context) {
	organizationID := util.AuthorizedSubjectID(c, "organization")
	if organizationID == nil {
		provide.RenderError("unauthorized", 401, c)
		return
	} else if common.OrganizationID != nil && organizationID.String() != *common.OrganizationID {
		provide.RenderError("forbidden", 403, c)
		return
	}

	var mappings []*Mapping

	db := dbconf.DatabaseConnection()
	var query *gorm.DB
	if c.Query("workgroup_id") != "" {
		workgroupID, err := uuid.FromString(c.Query("workgroup_id"))
		if err != nil {
			provide.RenderError(err.Error(), 422, c)
			return
		}
		query = db.Where("organization_id = ? AND workgroup_id = ?", organizationID, workgroupID)
	} else {
		query = db.Where("organization_id = ?", organizationID)
	}

	query = query.Order("type DESC")
	provide.Paginate(c, query, &Mapping{}).Find(&mappings)

	for _, mapping := range mappings {
		mapping.enrich()
	}

	provide.Render(mappings, 200, c)
}

func createMappingHandler(c *gin.Context) {
	organizationID := util.AuthorizedSubjectID(c, "organization")
	if organizationID == nil {
		provide.RenderError("unauthorized", 401, c)
		return
	} else if common.OrganizationID != nil && organizationID.String() != *common.OrganizationID {
		provide.RenderError("forbidden", 403, c)
		return
	}

	buf, err := c.GetRawData()
	if err != nil {
		provide.RenderError(err.Error(), 400, c)
		return
	}

	var mapping *Mapping
	err = json.Unmarshal(buf, &mapping)
	if err != nil {
		provide.RenderError(err.Error(), 422, c)
		return
	}

	mapping.OrganizationID = organizationID

	if mapping.Create() {
		provide.Render(mapping, 201, c)
	} else if len(mapping.Errors) > 0 {
		obj := map[string]interface{}{}
		obj["errors"] = mapping.Errors
		provide.Render(obj, 422, c)
	} else {
		provide.RenderError("internal persistence error", 500, c)
	}
}

func updateMappingHandler(c *gin.Context) {
	organizationID := util.AuthorizedSubjectID(c, "organization")
	if organizationID == nil {
		provide.RenderError("unauthorized", 401, c)
		return
	} else if common.OrganizationID != nil && organizationID.String() != *common.OrganizationID {
		provide.RenderError("forbidden", 403, c)
		return
	}

	buf, err := c.GetRawData()
	if err != nil {
		provide.RenderError(err.Error(), 400, c)
		return
	}

	mappingIDStr := c.Param("id")
	mappingID, err := uuid.FromString(mappingIDStr)
	if err != nil {
		provide.RenderError(err.Error(), 422, c)
		return
	}

	mapping := FindMappingByID(mappingID)
	if mapping == nil {
		provide.RenderError("not found", 404, c)
		return
	}

	_mapping := &Mapping{}
	err = json.Unmarshal(buf, _mapping)
	if err != nil {
		provide.RenderError(err.Error(), 422, c)
		return
	}

	if _mapping.ID != uuid.Nil && mapping.ID != _mapping.ID {
		provide.RenderError("cannot modify mapping id", 400, c)
		return
	}

	if mapping.Update(_mapping) {
		provide.Render(nil, 204, c)
	} else if len(mapping.Errors) > 0 {
		obj := map[string]interface{}{}
		obj["errors"] = mapping.Errors
		provide.Render(obj, 422, c)
	} else {
		provide.RenderError("internal persistence error", 500, c)
	}
}

func deleteMappingHandler(c *gin.Context) {
	organizationID := util.AuthorizedSubjectID(c, "organization")
	if organizationID == nil {
		provide.RenderError("unauthorized", 401, c)
		return
	} else if common.OrganizationID != nil && organizationID.String() != *common.OrganizationID {
		provide.RenderError("forbidden", 403, c)
		return
	}

	mappingIDStr := c.Param("id")
	mappingID, err := uuid.FromString(mappingIDStr)
	if err != nil {
		provide.RenderError(err.Error(), 422, c)
		return
	}

	mapping := FindMappingByID(mappingID)
	if mapping == nil {
		provide.RenderError("not found", 404, c)
		return
	}

	if mapping.Delete() {
		provide.Render(nil, 204, c)
	} else if len(mapping.Errors) > 0 {
		obj := map[string]interface{}{}
		obj["errors"] = mapping.Errors
		provide.Render(obj, 422, c)
	}
}

func createWorkflowHandler(c *gin.Context) {
	organizationID := util.AuthorizedSubjectID(c, "organization")
	if organizationID == nil {
		provide.RenderError("unauthorized", 401, c)
		return
	} else if common.OrganizationID != nil && organizationID.String() != *common.OrganizationID {
		provide.RenderError("forbidden", 403, c)
		return
	}

	buf, err := c.GetRawData()
	if err != nil {
		provide.RenderError(err.Error(), 400, c)
		return
	}

	var workflow *Workflow
	err = json.Unmarshal(buf, &workflow)
	if err != nil {
		provide.RenderError(err.Error(), 422, c)
		return
	}

	if workflow.Create(nil) {
		provide.Render(workflow, 201, c)
	} else if len(workflow.Errors) > 0 {
		obj := map[string]interface{}{}
		obj["errors"] = workflow.Errors
		provide.Render(obj, 422, c)
	} else {
		provide.RenderError("internal persistence error", 500, c)
	}
}

func deployWorkflowHandler(c *gin.Context) {
	organizationID := util.AuthorizedSubjectID(c, "organization")
	if organizationID == nil {
		provide.RenderError("unauthorized", 401, c)
		return
	} else if common.OrganizationID != nil && organizationID.String() != *common.OrganizationID {
		provide.RenderError("forbidden", 403, c)
		return
	}

	buf, err := c.GetRawData()
	if err != nil {
		provide.RenderError(err.Error(), 400, c)
		return
	}

	var params map[string]interface{}
	err = json.Unmarshal(buf, &params)
	if err != nil {
		provide.RenderError(err.Error(), 422, c)
		return
	}

	workflowID, err := uuid.FromString(c.Param("id"))
	if err != nil {
		provide.RenderError(err.Error(), 422, c)
		return
	}

	workflow := FindWorkflowByID(workflowID)
	if workflow == nil {
		provide.RenderError("not found", 404, c)
		return
	}

	_workflow := &Workflow{}
	_workflow.Status = common.StringOrNil(workflowStatusDeployed) // HACK!!!
	_workflow.Version = workflow.Version

	if workflow.Update(_workflow) {
		provide.Render(workflow, 202, c)
	} else if len(workflow.Errors) > 0 {
		obj := map[string]interface{}{}
		obj["errors"] = workflow.Errors
		provide.Render(obj, 422, c)
	} else {
		provide.RenderError("internal persistence error", 500, c)
	}
}

func versionWorkflowHandler(c *gin.Context) {
	organizationID := util.AuthorizedSubjectID(c, "organization")
	if organizationID == nil {
		provide.RenderError("unauthorized", 401, c)
		return
	} else if common.OrganizationID != nil && organizationID.String() != *common.OrganizationID {
		provide.RenderError("forbidden", 403, c)
		return
	}

	buf, err := c.GetRawData()
	if err != nil {
		provide.RenderError(err.Error(), 400, c)
		return
	}

	workflowID, err := uuid.FromString(c.Param("id"))
	if err != nil {
		provide.RenderError(err.Error(), 422, c)
		return
	}

	workflow := FindWorkflowByID(workflowID)
	if workflow == nil {
		provide.RenderError("not found", 404, c)
		return
	}

	var params map[string]interface{}
	err = json.Unmarshal(buf, &params)
	if err != nil {
		provide.RenderError(err.Error(), 422, c)
		return
	}

	var name *string
	if nme, ok := params["name"].(string); ok {
		name = common.StringOrNil(nme)
	}

	var description *string
	if desc, ok := params["description"].(string); ok {
		name = common.StringOrNil(desc)
	}

	var version *string
	if vrsn, ok := params["version"].(string); ok {
		version = common.StringOrNil(vrsn)
	} else {
		provide.RenderError("version is required", 422, c)
		return
	}

	raw, err := json.Marshal(workflow)
	if err != nil {
		provide.RenderError(err.Error(), 500, c)
		return
	}

	var _workflow *Workflow
	err = json.Unmarshal(raw, &_workflow)
	if err != nil {
		provide.RenderError(err.Error(), 500, c)
		return
	}

	_workflow.ID = uuid.Nil
	_workflow.Status = common.StringOrNil(workflowStatusDraft)
	_workflow.Version = version

	if name != nil {
		_workflow.Name = name
	}

	if description != nil {
		_workflow.Description = description
	}

	if _workflow.createVersion(workflow, *version) {
		provide.Render(_workflow, 201, c)
	} else if len(_workflow.Errors) > 0 {
		obj := map[string]interface{}{}
		obj["errors"] = _workflow.Errors
		provide.Render(obj, 422, c)
	} else {
		provide.RenderError("internal persistence error", 500, c)
	}
}

func listWorkflowVersionsHandler(c *gin.Context) {
	organizationID := util.AuthorizedSubjectID(c, "organization")
	if organizationID == nil {
		provide.RenderError("unauthorized", 401, c)
		return
	} else if common.OrganizationID != nil && organizationID.String() != *common.OrganizationID {
		provide.RenderError("forbidden", 403, c)
		return
	}

	workflowID, err := uuid.FromString(c.Param("id"))
	if err != nil {
		provide.RenderError(err.Error(), 422, c)
		return
	}

	workflow := FindWorkflowByID(workflowID)
	if workflow == nil {
		provide.RenderError("not found", 404, c)
		return
	}

	db := dbconf.DatabaseConnection()
	versions := workflow.listVersions(db)
	// var versions []*WorkflowVersion
	// query := workflow.listVersionsQuery()
	// provide.Paginate(c, query, &WorkflowVersion{}).Find(&versions)
	provide.Render(versions, 200, c)
}

func updateWorkflowHandler(c *gin.Context) {
	organizationID := util.AuthorizedSubjectID(c, "organization")
	if organizationID == nil {
		provide.RenderError("unauthorized", 401, c)
		return
	} else if common.OrganizationID != nil && organizationID.String() != *common.OrganizationID {
		provide.RenderError("forbidden", 403, c)
		return
	}

	buf, err := c.GetRawData()
	if err != nil {
		provide.RenderError(err.Error(), 400, c)
		return
	}

	workflowID, err := uuid.FromString(c.Param("id"))
	if err != nil {
		provide.RenderError(err.Error(), 422, c)
		return
	}

	workflow := FindWorkflowByID(workflowID)
	if workflow == nil {
		provide.RenderError("not found", 404, c)
		return
	}

	var _workflow *Workflow
	err = json.Unmarshal(buf, &_workflow)
	if err != nil {
		provide.RenderError(err.Error(), 422, c)
		return
	}

	if workflow.Update(_workflow) {
		provide.Render(nil, 204, c)
	} else if len(workflow.Errors) > 0 {
		obj := map[string]interface{}{}
		obj["errors"] = workflow.Errors
		provide.Render(obj, 422, c)
	} else {
		provide.RenderError("internal persistence error", 500, c)
	}
}

func deleteWorkflowHandler(c *gin.Context) {
	organizationID := util.AuthorizedSubjectID(c, "organization")
	if organizationID == nil {
		provide.RenderError("unauthorized", 401, c)
		return
	} else if common.OrganizationID != nil && organizationID.String() != *common.OrganizationID {
		provide.RenderError("forbidden", 403, c)
		return
	}

	workflowID, err := uuid.FromString(c.Param("id"))
	if err != nil {
		provide.RenderError(err.Error(), 422, c)
		return
	}

	workflow := FindWorkflowByID(workflowID)
	if workflow == nil {
		provide.RenderError("not found", 404, c)
		return
	}

	if workflow.Delete() {
		provide.Render(nil, 204, c)
	} else if len(workflow.Errors) > 0 {
		obj := map[string]interface{}{}
		obj["errors"] = workflow.Errors
		provide.Render(obj, 422, c)
	} else {
		provide.RenderError("internal persistence error", 500, c)
	}
}

func listWorkflowsHandler(c *gin.Context) {
	organizationID := util.AuthorizedSubjectID(c, "organization")
	if organizationID == nil {
		provide.RenderError("unauthorized", 401, c)
		return
	} else if common.OrganizationID != nil && organizationID.String() != *common.OrganizationID {
		provide.RenderError("forbidden", 403, c)
		return
	}

	var workflows []*Workflow

	filterInstances := strings.ToLower(c.Query("filter_instances")) == "true"
	filterPrototypes := strings.ToLower(c.Query("filter_prototypes")) == "true"

	db := dbconf.DatabaseConnection()
	var query *gorm.DB

	if c.Query("workgroup_id") != "" {
		query = db.Where("workflows.workgroup_id = ?", c.Query("workgroup_id"))
	}
	if filterInstances {
		query = db.Where("workflows.workflow_id IS NULL")
	}
	if filterPrototypes {
		query = db.Where("workflows.workflow_id IS NOT NULL")
	}

	if query == nil {
		query = db.Order("workflows.created_at DESC")
	} else {
		query = query.Order("workflows.created_at DESC")
	}

	provide.Paginate(c, query, &Workflow{}).Find(&workflows)
	provide.Render(workflows, 200, c)
}

func workflowDetailsHandler(c *gin.Context) {
	organizationID := util.AuthorizedSubjectID(c, "organization")
	if organizationID == nil {
		provide.RenderError("unauthorized", 401, c)
		return
	} else if common.OrganizationID != nil && organizationID.String() != *common.OrganizationID {
		provide.RenderError("forbidden", 403, c)
		return
	}

	workflowID, err := uuid.FromString(c.Param("id"))
	if err != nil {
		provide.RenderError(err.Error(), 422, c)
		return
	}

	workflow := FindWorkflowByID(workflowID)

	if workflow != nil {
		workflow.enrich()
		provide.Render(workflow, 200, c)
	} else {
		provide.RenderError("workflow not found", 404, c)
	}
}

func createWorkstepHandler(c *gin.Context) {
	organizationID := util.AuthorizedSubjectID(c, "organization")
	if organizationID == nil {
		provide.RenderError("unauthorized", 401, c)
		return
	} else if common.OrganizationID != nil && organizationID.String() != *common.OrganizationID {
		provide.RenderError("forbidden", 403, c)
		return
	}

	buf, err := c.GetRawData()
	if err != nil {
		provide.RenderError(err.Error(), 400, c)
		return
	}

	var workstep *Workstep
	err = json.Unmarshal(buf, &workstep)
	if err != nil {
		provide.RenderError(err.Error(), 422, c)
		return
	}

	workflowID, err := uuid.FromString(c.Param("id"))
	if err != nil {
		provide.RenderError(err.Error(), 422, c)
		return
	}

	// workstep.OrganizationID = organizationID
	workstep.WorkflowID = &workflowID

	workflow := FindWorkflowByID(workflowID)
	if workflow != nil && !workflow.isPrototype() {
		provide.RenderError("cannot add workstep to workflow instance", 400, c)
		return
	}

	if workflow != nil && workflow.Status != nil && *workflow.Status != workflowStatusDraft {
		provide.RenderError("cannot add worksteps to non-draft workflow prototype", 400, c)
		return
	}

	if workstep.Create(nil) {
		provide.Render(workstep, 201, c)
	} else if len(workstep.Errors) > 0 {
		obj := map[string]interface{}{}
		obj["errors"] = workstep.Errors
		provide.Render(obj, 422, c)
	} else {
		provide.RenderError("internal persistence error", 500, c)
	}
}

func updateWorkstepHandler(c *gin.Context) {
	organizationID := util.AuthorizedSubjectID(c, "organization")
	if organizationID == nil {
		provide.RenderError("unauthorized", 401, c)
		return
	} else if common.OrganizationID != nil && organizationID.String() != *common.OrganizationID {
		provide.RenderError("forbidden", 403, c)
		return
	}

	buf, err := c.GetRawData()
	if err != nil {
		provide.RenderError(err.Error(), 400, c)
		return
	}

	workflowID, err := uuid.FromString(c.Param("id"))
	if err != nil {
		provide.RenderError(err.Error(), 422, c)
		return
	}

	workstepID, err := uuid.FromString(c.Param("workstepId"))
	if err != nil {
		provide.RenderError(err.Error(), 422, c)
		return
	}

	workstep := FindWorkstepByID(workstepID)
	if workstep == nil {
		provide.RenderError("not found", 404, c)
		return
	} else if workstep.WorkflowID == nil || workstep.WorkflowID.String() != workflowID.String() {
		provide.RenderError("forbidden", 403, c)
		return
	}

	var _workstep *Workstep
	err = json.Unmarshal(buf, &_workstep)
	if err != nil {
		provide.RenderError(err.Error(), 422, c)
		return
	}

	var __workstep map[string]interface{}
	err = json.Unmarshal(buf, &__workstep)
	if err != nil {
		provide.RenderError(err.Error(), 422, c)
		return
	}
	if _, ok := __workstep["cardinality"].(float64); ok {
		_workstep.userInputCardinality = true
	}

	if workstep.Status != nil && _workstep.Status != nil && *workstep.Status != *_workstep.Status {
		provide.RenderError("cannot modify workstep status", 400, c)
		return
	}

	if workstep.Update(_workstep) {
		provide.Render(nil, 204, c)
	} else if len(workstep.Errors) > 0 {
		obj := map[string]interface{}{}
		obj["errors"] = workstep.Errors
		provide.Render(obj, 422, c)
	} else {
		provide.RenderError("internal persistence error", 500, c)
	}
}

func deleteWorkstepHandler(c *gin.Context) {
	organizationID := util.AuthorizedSubjectID(c, "organization")
	if organizationID == nil {
		provide.RenderError("unauthorized", 401, c)
		return
	} else if common.OrganizationID != nil && organizationID.String() != *common.OrganizationID {
		provide.RenderError("forbidden", 403, c)
		return
	}

	workflowID, err := uuid.FromString(c.Param("id"))
	if err != nil {
		provide.RenderError(err.Error(), 422, c)
		return
	}

	workstepID, err := uuid.FromString(c.Param("workstepId"))
	if err != nil {
		provide.RenderError(err.Error(), 422, c)
		return
	}

	workstep := FindWorkstepByID(workstepID)
	if workstep == nil {
		provide.RenderError("not found", 404, c)
		return
	} else if workstep.WorkflowID == nil || workstep.WorkflowID.String() != workflowID.String() {
		provide.RenderError("forbidden", 403, c)
		return
	}

	if workstep.Delete() {
		provide.Render(nil, 204, c)
	} else if len(workstep.Errors) > 0 {
		obj := map[string]interface{}{}
		obj["errors"] = workstep.Errors
		provide.Render(obj, 422, c)
	} else {
		provide.RenderError("internal persistence error", 500, c)
	}
}

func executeWorkstepHandler(c *gin.Context) {
	organizationID := util.AuthorizedSubjectID(c, "organization")
	if organizationID == nil {
		provide.RenderError("unauthorized", 401, c)
		return
	} else if common.OrganizationID != nil && organizationID.String() != *common.OrganizationID {
		provide.RenderError("forbidden", 403, c)
		return
	}

	buf, err := c.GetRawData()
	if err != nil {
		provide.RenderError(err.Error(), 400, c)
		return
	}

	workflowID, err := uuid.FromString(c.Param("id"))
	if err != nil {
		provide.RenderError(err.Error(), 422, c)
		return
	}

	workstepID, err := uuid.FromString(c.Param("workstepId"))
	if err != nil {
		provide.RenderError(err.Error(), 422, c)
		return
	}

	workstep := FindWorkstepByID(workstepID)
	if workstep == nil {
		provide.RenderError("not found", 404, c)
		return
	} else if workstep.WorkflowID == nil || workstep.WorkflowID.String() != workflowID.String() {
		provide.RenderError("forbidden", 403, c)
		return
	}

	if workstep.Status != nil && *workstep.Status != workstepStatusInit && *workstep.Status != workstepStatusRunning {
		provide.RenderError("cannot execute workstep", 400, c)
		return
	}

	var payload *baseline.ProtocolMessagePayload
	err = json.Unmarshal(buf, &payload)
	if err != nil {
		provide.RenderError(err.Error(), 422, c)
		return
	}

	token, _ := util.ParseBearerAuthorizationHeader(c, nil)
	proof, err := workstep.execute(token.Raw, payload)
	if err != nil {
		provide.RenderError(fmt.Sprintf("cannot execute workstep; %s", err.Error()), 400, c)
		return
	}

	if proof != nil {
		provide.Render(proof, 201, c)
	} else if len(workstep.Errors) > 0 {
		obj := map[string]interface{}{}
		obj["errors"] = workstep.Errors
		provide.Render(obj, 422, c)
	} else {
		provide.RenderError("internal persistence error", 500, c)
	}
}

func listWorkstepsHandler(c *gin.Context) {
	organizationID := util.AuthorizedSubjectID(c, "organization")
	if organizationID == nil {
		provide.RenderError("unauthorized", 401, c)
		return
	} else if common.OrganizationID != nil && organizationID.String() != *common.OrganizationID {
		provide.RenderError("forbidden", 403, c)
		return
	}

	var worksteps []*Workstep

	filterInstances := strings.ToLower(c.Query("filter_instances")) == "true"
	filterPrototypes := strings.ToLower(c.Query("filter_prototypes")) == "true"

	db := dbconf.DatabaseConnection()
	var query *gorm.DB

	if c.Param("id") != "" {
		query = db.Where("worksteps.workflow_id = ?", c.Param("id"))
	}
	if filterInstances {
		query = db.Where("worksteps.workstep_id IS NULL")
	}
	if filterPrototypes {
		query = db.Where("worksteps.workstep_id IS NOT NULL")
	}

	if query == nil {
		query = db.Order("worksteps.cardinality ASC")
	} else {
		query = query.Order("worksteps.cardinality ASC")
	}

	provide.Paginate(c, query, &Workstep{}).Find(&worksteps)
	provide.Render(worksteps, 200, c)
}

func workstepDetailsHandler(c *gin.Context) {
	organizationID := util.AuthorizedSubjectID(c, "organization")
	if organizationID == nil {
		provide.RenderError("unauthorized", 401, c)
		return
	} else if common.OrganizationID != nil && organizationID.String() != *common.OrganizationID {
		provide.RenderError("forbidden", 403, c)
		return
	}

	workstepID, err := uuid.FromString(c.Param("workstepId"))
	if err != nil {
		provide.RenderError(err.Error(), 422, c)
		return
	}

	workstep := FindWorkstepByID(workstepID)

	if workstep != nil {
		token, _ := util.ParseBearerAuthorizationHeader(c, nil)
		workstep.enrich(token.Raw)
		provide.Render(workstep, 200, c)
	} else {
		provide.RenderError("workstep not found", 404, c)
	}
}

func issueVerifiableCredentialHandler(c *gin.Context) {
	issueVCRequest := &baseline.IssueVerifiableCredentialRequest{}

	buf, err := c.GetRawData()
	if err != nil {
		provide.RenderError(err.Error(), 400, c)
		return
	}

	err = json.Unmarshal(buf, &issueVCRequest)
	if err != nil {
		msg := fmt.Sprintf("failed to umarshal workgroup invitation acceptance request; %s", err.Error())
		common.Log.Warning(msg)
		provide.RenderError(msg, 422, c)
		return
	}

	if issueVCRequest.Address == nil {
		provide.RenderError("address is required", 422, c)
		return
	}

	// FIXME-- make general with PublicKey
	if issueVCRequest.PublicKey == nil {
		provide.RenderError("public_key is required", 422, c)
		return
	}

	if issueVCRequest.Signature == nil {
		provide.RenderError("signature is required", 422, c)
		return
	}

	msg := crypto.Keccak256Hash([]byte(*issueVCRequest.Address))
	sig, _ := hex.DecodeString(*issueVCRequest.Signature)
	pubkey, err := crypto.Ecrecover(msg.Bytes(), []byte(sig))
	if err != nil {
		msg := fmt.Sprintf("failed to recover public key from signature: %s; %s", *issueVCRequest.Signature, err.Error())
		common.Log.Warning(msg)
		provide.RenderError(msg, 422, c)
		return
	}

	// pubkeyBytes := crypto.Keccak256Hash(pubkey).Bytes()
	// recoveredAddress := fmt.Sprintf("0x%s", pubkeyBytes[12:32])
	// common.Log.Debugf("recovered public key: 0x%s; recovered address: %s", hex.EncodeToString(pubkeyBytes), recoveredAddress)

	signerPubkey, err := hex.DecodeString((*issueVCRequest.PublicKey)[2:])
	if err != nil {
		msg := fmt.Sprintf("failed to recover public key from signature: %s; %s", *issueVCRequest.Signature, err.Error())
		common.Log.Warning(msg)
		provide.RenderError(msg, 422, c)
		return
	}

	if !bytes.Equal(pubkey, signerPubkey) {
		// common.Log.Warningf("recovered address %s did not match expected signer %s", string(recoveredAddress), *issueVCRequest.Address)
		common.Log.Warningf("recovered public key %s did not match expected signer %s", string(pubkey), *issueVCRequest.PublicKey)
		provide.RenderError("recovered address did not match signer", 422, c)
		return
	}

	credential, err := IssueVC(*issueVCRequest.Address, map[string]interface{}{})

	if err == nil {
		provide.Render(&baseline.IssueVerifiableCredentialResponse{
			VC: credential,
		}, 201, c)
	} else {
		obj := map[string]interface{}{}
		obj["errors"] = []interface{}{} // FIXME
		provide.Render(obj, 422, c)
	}
}

func listBPIAccountsHandler(c *gin.Context) {
	provide.RenderError("not implemented", 501, c)
}

func bpiAccountDetailsHandler(c *gin.Context) {
	provide.RenderError("not implemented", 501, c)
}

func createBPIAccountHandler(c *gin.Context) {
	provide.RenderError("not implemented", 501, c)
}

func listSubjectsHandler(c *gin.Context) {
	provide.RenderError("not implemented", 501, c)
}

func subjectDetailsHandler(c *gin.Context) {
	provide.RenderError("not implemented", 501, c)
}

func createSubjectHandler(c *gin.Context) {
	provide.RenderError("not implemented", 501, c)
}

func updateSubjectHandler(c *gin.Context) {
	provide.RenderError("not implemented", 501, c)
}

func listSubjectAccountsHandler(c *gin.Context) {
	provide.RenderError("not implemented", 501, c)
}

func subjectAccountDetailsHandler(c *gin.Context) {
	provide.RenderError("not implemented", 501, c)
}

func createSubjectAccountsHandler(c *gin.Context) {
	provide.RenderError("not implemented", 501, c)
}

func updateSubjectAccountsHandler(c *gin.Context) {
	provide.RenderError("not implemented", 501, c)
}
