package baseline

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	dbconf "github.com/kthomas/go-db-config"
	natsutil "github.com/kthomas/go-natsutil"
	uuid "github.com/kthomas/go.uuid"
	"github.com/nats-io/nats.go"
	"github.com/provideplatform/baseline/common"
	"github.com/provideplatform/provide-go/api/baseline"
	"github.com/provideplatform/provide-go/api/ident"
	"github.com/provideplatform/provide-go/api/privacy"
)

const defaultNatsStream = "baseline"

const protomsgPayloadTypeCircuit = "circuit"
const protomsgPayloadTypeWorkflow = "workflow"

const natsDispatchInvitationSubject = "baseline.invitation.outbound"
const natsDispatchInvitationMaxInFlight = 2048
const dispatchInvitationAckWait = time.Second * 30
const natsDispatchInvitationMaxDeliveries = 10

const natsBaselineWorkflowDeployMessageSubject = "baseline.workflow.deploy"
const natsBaselineWorkflowDeployMessageMaxInFlight = 2048
const baselineWorkflowDeployMessageAckWait = time.Second * 30
const natsBaselineWorkflowDeployMessageMaxDeliveries = 10

const natsBaselineWorkstepDeployMessageSubject = "baseline.workstep.deploy"
const natsBaselineWorkstepDeployMessageMaxInFlight = 2048
const baselineWorkstepDeployMessageAckWait = time.Second * 30
const natsBaselineWorkstepDeployMessageMaxDeliveries = 10

const natsBaselineWorkstepFinalizeDeployMessageSubject = "baseline.workstep.deploy.finalize"
const natsBaselineWorkstepFinalizeDeployMessageMaxInFlight = 2048
const baselineWorkstepFinalizeDeployMessageAckWait = time.Second * 30
const natsBaselineWorkstepFinalizeDeployMessageMaxDeliveries = 10

const natsDispatchProtocolMessageSubject = "baseline.protocolmessage.outbound"
const natsDispatchProtocolMessageMaxInFlight = 2048
const dispatchProtocolMessageAckWait = time.Second * 30
const natsDispatchProtocolMessageMaxDeliveries = 10

const natsBaselineProxyInboundSubject = "baseline.inbound"
const natsBaselineProxyInboundMaxInFlight = 2048
const baselineProxyInboundAckWait = time.Second * 30
const natsBaselineProxyInboundMaxDeliveries = 10

const natsBaselineSubject = "baseline"
const baselineProxyAckWait = time.Second * 30

// Message is a proxy-internal wrapper for protocol message handling
type Message struct {
	baseline.Message
	ProtocolMessage *ProtocolMessage `sql:"-" json:"protocol_message,omitempty"`
}

// ProtocolMessage is a baseline protocol message
// see https://github.com/ethereum-oasis/baseline/blob/master/core/types/src/protocol.ts
type ProtocolMessage struct {
	baseline.ProtocolMessage
}

func init() {
	if !common.ConsumeNATSStreamingSubscriptions {
		common.Log.Debug("proxy package consumer configured to skip NATS streaming subscription setup")
		return
	}

	var waitGroup sync.WaitGroup

	natsutil.EstablishSharedNatsConnection(nil)
	natsutil.NatsCreateStream(defaultNatsStream, []string{
		fmt.Sprintf("%s.>", defaultNatsStream),
	})

	createNatsBaselineProxySubscriptions(&waitGroup)
	createNatsBaselineWorkflowDeploySubscriptions(&waitGroup)
	createNatsBaselineWorkstepDeploySubscriptions(&waitGroup)
	createNatsBaselineWorkstepFinalizeDeploySubscriptions(&waitGroup)
	createNatsDispatchInvitationSubscriptions(&waitGroup)
	createNatsDispatchProtocolMessageSubscriptions(&waitGroup)
}

func createNatsBaselineProxySubscriptions(wg *sync.WaitGroup) {
	for i := uint64(0); i < natsutil.GetNatsConsumerConcurrency(); i++ {
		natsutil.RequireNatsJetstreamSubscription(wg,
			baselineProxyInboundAckWait,
			natsBaselineProxyInboundSubject,
			natsBaselineProxyInboundSubject,
			natsBaselineProxyInboundSubject,
			consumeBaselineProxyInboundSubscriptionsMsg,
			baselineProxyAckWait,
			natsBaselineProxyInboundMaxInFlight,
			natsBaselineProxyInboundMaxDeliveries,
			nil,
		)
	}

	conn, _ := natsutil.GetSharedNatsConnection(nil)
	conn.Subscribe(natsBaselineSubject, func(msg *nats.Msg) {
		common.Log.Debugf("consuming %d-byte NATS inbound protocol message on subject: %s", len(msg.Data), msg.Subject)
		_, err := natsutil.NatsJetstreamPublish(natsBaselineProxyInboundSubject, msg.Data)
		if err != nil {
			common.Log.Warningf("failed to publish inbound protocol message to local jetstream consumers; %s", err.Error())
			msg.Nak()
			return
		}

		msg.Ack()
	})
}

func createNatsBaselineWorkflowDeploySubscriptions(wg *sync.WaitGroup) {
	for i := uint64(0); i < natsutil.GetNatsConsumerConcurrency(); i++ {
		natsutil.RequireNatsJetstreamSubscription(wg,
			baselineWorkflowDeployMessageAckWait,
			natsBaselineWorkflowDeployMessageSubject,
			natsBaselineWorkflowDeployMessageSubject,
			natsBaselineWorkflowDeployMessageSubject,
			consumeBaselineWorkflowFinalizeDeploySubscriptionsMsg,
			baselineWorkflowDeployMessageAckWait,
			natsBaselineWorkflowDeployMessageMaxInFlight,
			natsBaselineWorkflowDeployMessageMaxDeliveries,
			nil,
		)
	}
}

func createNatsBaselineWorkstepDeploySubscriptions(wg *sync.WaitGroup) {
	for i := uint64(0); i < natsutil.GetNatsConsumerConcurrency(); i++ {
		natsutil.RequireNatsJetstreamSubscription(wg,
			baselineWorkstepDeployMessageAckWait,
			natsBaselineWorkstepDeployMessageSubject,
			natsBaselineWorkstepDeployMessageSubject,
			natsBaselineWorkstepDeployMessageSubject,
			consumeBaselineWorkstepDeploySubscriptionsMsg,
			baselineWorkstepDeployMessageAckWait,
			natsBaselineWorkstepDeployMessageMaxInFlight,
			natsBaselineWorkstepDeployMessageMaxDeliveries,
			nil,
		)
	}
}

func createNatsBaselineWorkstepFinalizeDeploySubscriptions(wg *sync.WaitGroup) {
	for i := uint64(0); i < natsutil.GetNatsConsumerConcurrency(); i++ {
		natsutil.RequireNatsJetstreamSubscription(wg,
			baselineWorkstepFinalizeDeployMessageAckWait,
			natsBaselineWorkstepFinalizeDeployMessageSubject,
			natsBaselineWorkstepFinalizeDeployMessageSubject,
			natsBaselineWorkstepFinalizeDeployMessageSubject,
			consumeBaselineWorkstepFinalizeDeploySubscriptionsMsg,
			baselineWorkstepFinalizeDeployMessageAckWait,
			natsBaselineWorkstepFinalizeDeployMessageMaxInFlight,
			natsBaselineWorkstepFinalizeDeployMessageMaxDeliveries,
			nil,
		)
	}
}

func createNatsDispatchInvitationSubscriptions(wg *sync.WaitGroup) {
	for i := uint64(0); i < natsutil.GetNatsConsumerConcurrency(); i++ {
		natsutil.RequireNatsJetstreamSubscription(wg,
			dispatchInvitationAckWait,
			natsDispatchInvitationSubject,
			natsDispatchInvitationSubject,
			natsDispatchInvitationSubject,
			consumeDispatchInvitationSubscriptionsMsg,
			dispatchInvitationAckWait,
			natsDispatchInvitationMaxInFlight,
			natsDispatchInvitationMaxDeliveries,
			nil,
		)
	}
}

func createNatsDispatchProtocolMessageSubscriptions(wg *sync.WaitGroup) {
	for i := uint64(0); i < natsutil.GetNatsConsumerConcurrency(); i++ {
		natsutil.RequireNatsJetstreamSubscription(wg,
			dispatchProtocolMessageAckWait,
			natsDispatchProtocolMessageSubject,
			natsDispatchProtocolMessageSubject,
			natsDispatchProtocolMessageSubject,
			consumeDispatchProtocolMessageSubscriptionsMsg,
			dispatchProtocolMessageAckWait,
			natsDispatchProtocolMessageMaxInFlight,
			natsDispatchProtocolMessageMaxDeliveries,
			nil,
		)
	}
}

func consumeBaselineProxyInboundSubscriptionsMsg(msg *nats.Msg) {
	common.Log.Debugf("consuming %d-byte NATS inbound protocol message on internal subject: %s", len(msg.Data), msg.Subject)

	protomsg := &ProtocolMessage{}
	err := json.Unmarshal(msg.Data, &protomsg)
	if err != nil {
		common.Log.Warningf("failed to umarshal inbound protocol message; %s", err.Error())
		msg.Nak()
		return
	}

	if protomsg.Opcode == nil {
		common.Log.Warningf("inbound protocol message specified invalid opcode; %s", err.Error())
		msg.Term()
		return
	}

	switch *protomsg.Opcode {
	case baseline.ProtocolMessageOpcodeBaseline:
		success := protomsg.baselineInbound()
		if !success {
			common.Log.Warning("failed to baseline inbound protocol message")
			return
		}

	case baseline.ProtocolMessageOpcodeJoin:
		common.Log.Warningf("JOIN opcode not yet implemented")
		// const payload = JSON.parse(msg.payload.toString());
		// const messagingEndpoint = await this.resolveMessagingEndpoint(payload.address);
		// if (!messagingEndpoint || !payload.address || !payload.authorized_bearer_token) {
		//   return Promise.reject('failed to handle baseline JOIN protocol message');
		// }
		// this.workgroupCounterparties.push(payload.address);
		// this.natsBearerTokens[messagingEndpoint] = payload.authorized_bearer_token;

		// const circuit = JSON.parse(JSON.stringify(this.baselineCircuit));
		// circuit.proving_scheme = circuit.provingScheme;
		// circuit.verifier_contract = circuit.verifierContract;
		// delete circuit.verifierContract;
		// delete circuit.createdAt;
		// delete circuit.vaultId;
		// delete circuit.provingScheme;
		// delete circuit.provingKeyId;
		// delete circuit.verifyingKeyId;
		// delete circuit.status;

		// // sync circuit artifacts
		// this.sendProtocolMessage(payload.address, Opcode.Sync, {
		//   type: 'circuit',
		//   payload: circuit,
		// });

	case baseline.ProtocolMessageOpcodeSync:
		token, err := vendOrganizationAccessToken()
		if err != nil {
			common.Log.Warningf("failed to handle inbound sync protocol message; %s", err.Error())
			return
		}

		// FIXME -- use switch and attempt nack if invalid sync type...
		if protomsg.Payload.Type != nil && *protomsg.Payload.Type == protomsgPayloadTypeCircuit {
			circuit, err := privacy.CreateCircuit(*token, protomsg.Payload.Object)
			if err != nil {
				common.Log.Warningf("failed to handle inbound sync protocol message; failed to create circuit; %s", err.Error())
				return
			}
			common.Log.Debugf("sync protocol message created circuit: %s", circuit.ID)
		} else if protomsg.Payload.Type != nil && *protomsg.Payload.Type == protomsgPayloadTypeWorkflow {
			workflow := &WorkflowInstance{}
			raw, err := json.Marshal(protomsg.Payload.Object)
			if err != nil {
				common.Log.Warningf("failed to handle inbound sync protocol message; failed to marshal payload object; %s", err.Error())
				return
			}
			json.Unmarshal(raw, &workflow)

			for _, workstep := range workflow.Worksteps {
				params := map[string]interface{}{}
				rawcircuit, _ := json.Marshal(workstep.Prover)
				json.Unmarshal(rawcircuit, &params)

				workstep.Prover, err = privacy.CreateCircuit(*token, params)
				if err != nil {
					common.Log.Warningf("failed to handle inbound sync protocol message; failed to create circuit; %s", err.Error())
					return
				}
				workstep.ProverID = &workstep.Prover.ID
				common.Log.Debugf("sync protocol message created circuit: %s", workstep.Prover.ID)
			}

			err = workflow.Cache()
			if err != nil {
				common.Log.Warningf("failed to handle inbound sync protocol message; failed to cache workflow; %s", err.Error())
				msg.Nak()
				return
			}

			if protomsg.BaselineID != nil {
				err = workflow.CacheByBaselineID(protomsg.BaselineID.String())
				if err != nil {
					common.Log.Warningf("failed to handle inbound sync protocol message; failed to cache workflow identifier by baseline id; %s", err.Error())
					msg.Nak()
					return
				}
			}

			common.Log.Debugf("cached %d-workstep workflow: %s", len(workflow.Worksteps), workflow.ID)
		}

	default:
		common.Log.Warningf("inbound protocol message specified invalid opcode; %s", err.Error())
		msg.Term()
		return
	}

	msg.Ack()
}

func consumeBaselineWorkflowFinalizeDeploySubscriptionsMsg(msg *nats.Msg) {
	common.Log.Debugf("consuming %d-byte NATS baseline workflow deploy message on subject: %s", len(msg.Data), msg.Subject)

	var params map[string]interface{}

	err := json.Unmarshal(msg.Data, &params)
	if err != nil {
		common.Log.Warningf("failed to umarshal baseline workflow deploy message; %s", err.Error())
		msg.Nak()
		return
	}

	workstepID, err := uuid.FromString(params["workflow_id"].(string))
	if err != nil {
		common.Log.Warningf("failed to parse baseline workflow id; %s", err.Error())
		msg.Nak()
		return
	}

	success := true

	workflow := FindWorkflowByID(workstepID)
	worksteps := FindWorkstepsByWorkflowID(workflow.ID)
	for _, workstep := range worksteps {
		if workstep.Status != nil && *workstep.Status != workstepStatusDeployed {
			common.Log.Debugf("waiting on workstep with id %s for pending deployment of workflow: %s", workstep.ID, workflow.ID)
			success = false
		}
	}

	if success {
		db := dbconf.DatabaseConnection()

		deployedAt := time.Now()
		workflow.DeployedAt = &deployedAt
		workflow.Status = common.StringOrNil(workstepStatusDeployed)

		db.Save(&workflow)
		msg.Ack()
	} else {
		msg.Nak()
	}
}

func consumeBaselineWorkstepDeploySubscriptionsMsg(msg *nats.Msg) {
	common.Log.Debugf("consuming %d-byte NATS baseline workstep deploy message on subject: %s", len(msg.Data), msg.Subject)

	var params map[string]interface{}

	err := json.Unmarshal(msg.Data, &params)
	if err != nil {
		common.Log.Warningf("failed to umarshal baseline workstep deploy message; %s", err.Error())
		msg.Nak()
		return
	}

	workstepID, err := uuid.FromString(params["workstep_id"].(string))
	if err != nil {
		common.Log.Warningf("failed to parse baseline workstep id; %s", err.Error())
		msg.Nak()
		return
	}

	workstep := FindWorkstepByID(workstepID)

	token, err := ident.CreateToken(*common.OrganizationRefreshToken, map[string]interface{}{
		"grant_type":      "refresh_token",
		"organization_id": *common.OrganizationID,
	})
	if err != nil {
		common.Log.Warningf("failed to vend organization access token; %s", err.Error())
		return
	}

	if workstep.deploy(*token.AccessToken) {
		common.Log.Debugf("workstep pending deployment: %s", workstep.ID)
		msg.Ack()
	} else {
		common.Log.Warningf("failed to deploy workstep: %s; nacking message...", workstep.ID)
	}
}

func consumeBaselineWorkstepFinalizeDeploySubscriptionsMsg(msg *nats.Msg) {
	common.Log.Debugf("consuming %d-byte NATS baseline workstep finalize deploy message on subject: %s", len(msg.Data), msg.Subject)

	var params map[string]interface{}

	err := json.Unmarshal(msg.Data, &params)
	if err != nil {
		common.Log.Warningf("failed to umarshal baseline workstep finalize deploy message; %s", err.Error())
		msg.Nak()
		return
	}

	workstepID, err := uuid.FromString(params["workstep_id"].(string))
	if err != nil {
		common.Log.Warningf("failed to parse baseline workstep id; %s", err.Error())
		msg.Nak()
		return
	}

	workstep := FindWorkstepByID(workstepID)

	token, err := ident.CreateToken(*common.OrganizationRefreshToken, map[string]interface{}{
		"grant_type":      "refresh_token",
		"organization_id": *common.OrganizationID,
	})
	if err != nil {
		common.Log.Warningf("failed to vend organization access token; %s", err.Error())
		return
	}

	if workstep.finalizeDeploy(*token.AccessToken) {
		common.Log.Debugf("deployed workstep: %s", workstep.ID)
		msg.Ack()
	} else {
		common.Log.Warningf("failed to finalize workstep deployment: %s; nacking message...", workstep.ID)
	}
}

func consumeDispatchInvitationSubscriptionsMsg(msg *nats.Msg) {
	common.Log.Debugf("consuming %d-byte NATS dispatch invitation message on subject: %s", len(msg.Data), msg.Subject)

	var params map[string]interface{}

	err := json.Unmarshal(msg.Data, &params)
	if err != nil {
		common.Log.Warningf("failed to umarshal dispatch invitation message; %s", err.Error())
		msg.Nak()
		return
	}

	// TODO!

	msg.Ack()
}

func consumeDispatchProtocolMessageSubscriptionsMsg(msg *nats.Msg) {
	common.Log.Debugf("consuming %d-byte NATS dispatch protocol message on subject: %s", len(msg.Data), msg.Subject)

	protomsg := &ProtocolMessage{}
	err := json.Unmarshal(msg.Data, &protomsg)
	if err != nil {
		common.Log.Warningf("failed to umarshal dispatch protocol message; %s", err.Error())
		msg.Nak()
		return
	}

	if protomsg.Recipient == nil {
		common.Log.Warningf("no participant specified in protocol message; %s", err.Error())
		msg.Term()
		return
	}

	url := lookupBaselineOrganizationMessagingEndpoint(*protomsg.Recipient)
	if url == nil {
		common.Log.Warningf("failed to lookup recipient messaging endpoint: %s", *protomsg.Recipient)
		msg.Nak()
		return
	}

	jwt := lookupBaselineOrganizationIssuedVC(*protomsg.Recipient)
	if jwt == nil {
		// request a VC from the counterparty
		jwt, err = requestBaselineOrganizationIssuedVC(*protomsg.Recipient)
		if err != nil {
			resolveBaselineCounterparties() // HACK-- this should not re-resolve all counterparties...

			common.Log.Warningf("failed to request verifiable credential from recipient counterparty: %s; %s", *protomsg.Recipient, err.Error())
			msg.Nak()
			return
		}
	}

	if jwt == nil {
		common.Log.Warningf("failed to dispatch protocol message to recipient: %s; no bearer token resolved", *protomsg.Recipient)
		msg.Nak()
		return
	}

	uuid, _ := uuid.NewV4()
	name := fmt.Sprintf("%s-%s", *common.BaselineOrganizationAddress, uuid.String())
	conn, err := natsutil.GetNatsConnection(name, *url, time.Second*10, jwt)
	if err != nil {
		common.Log.Warningf("failed to establish NATS connection to recipient: %s; %s", *protomsg.Recipient, err.Error())
		msg.Nak()
		return
	}

	defer conn.Close()

	err = conn.Publish(natsBaselineSubject, msg.Data)
	if err != nil {
		// clear cached endpoint so it will be re-fetched...
		// counterparty := lookupBaselineOrganization(*protomsg.Recipient)
		// counterparty.MessagingEndpoint = nil
		// counterparty.Cache()

		resolveBaselineCounterparties() // HACK-- this should not re-resolve all counterparties...

		common.Log.Warningf("failed to publish protocol message to recipient: %s; %s", *protomsg.Recipient, err.Error())
		msg.Nak()
		return
	}

	common.Log.Debugf("broadcast %d-byte protocol message to recipient: %s", len(msg.Data), *protomsg.Recipient)
	msg.Ack()
}
