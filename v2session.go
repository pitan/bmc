package bmc

import (
	"context"
	"encoding/hex"
	"fmt"
	"hash"
	"time"

	"github.com/gebn/bmc/pkg/ipmi"
	"github.com/gebn/bmc/pkg/layerexts"

	"github.com/cenkalti/backoff"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/prometheus/client_golang/prometheus"
)

// V2Session represents an established IPMI v2.0/RMCP+ session with a BMC.
type V2Session struct {
	v2ConnectionLayers
	*v2ConnectionShared

	// decode parses the layers in v2ConnectionShared, plus a confidentiality
	// layer.
	decode gopacket.DecodingLayerFunc

	// LocalID is the remote console's session ID, used by the BMC to send us
	// packets.
	LocalID uint32

	// RemoteID is the managed system's session ID, used by us to send the BMC
	// packets.
	RemoteID uint32

	// AuthenticatedSequenceNumbers is the pair of sequence numbers for
	// authenticated packets.
	AuthenticatedSequenceNumbers sequenceNumbers

	// UnauthenticatedSequenceNumbers is the pair of sequence numbers for
	// unauthenticated packets, i.e. those without an auth code. We only send
	// unauthenticated packets to the BMC if IntegrityAlgorithmNone was
	// negotiated.
	UnauthenticatedSequenceNumbers sequenceNumbers

	// SIK is the session integrity key, whose creation is described in section
	// 13.31 of the spes. It is the result of applying the negotiated
	// authentication algorithm (which is usually, but may not be, an HMAC) to
	// some random numbers, the remote console's requested maximum privilege
	// level, and username. The SIK is then used to derive K_1 and K_2 (and
	// possibly more, but not for any algorithms in the spec) which are the keys
	// for the integrity algorithm and confidentiality algorithms respectively.
	SIK []byte

	// AuthenticationAlgorithm is the algorithm used to authenticate the user
	// during establishment of the session. Given the session is already
	// established, this will not be used further.
	AuthenticationAlgorithm ipmi.AuthenticationAlgorithm

	// IntegrityAlgorithm is the algorithm used to sign, or authenticate packets
	// sent by the managed system and remote console. This library authenticates
	// all packets it sends inside a session, provided IntegrityAlgorithmNone
	// was not negotiated.
	IntegrityAlgorithm ipmi.IntegrityAlgorithm

	// ConfidentialityAlgorithm is the algorithm used to encrypt and decrypt
	// packets sent by the managed system and remote console. This library
	// encrypts all packets it sends inside a session, provided
	// ConfidentialityAlgorithmNone was not negotiated.
	ConfidentialityAlgorithm ipmi.ConfidentialityAlgorithm

	// AdditionalKeyMaterialGenerator is the instance of the authentication
	// algorithm used during session establishment, loaded with the session
	// integrity key. It has no further use as far as the BMC is concerned by
	// the time we have this struct, however we keep it around to allow
	// providing K_n for information purposes.
	AdditionalKeyMaterialGenerator

	integrityAlgorithm hash.Hash

	// confidentialityLayer is used to send packets (we encrypt all outgoing
	// packets), and to decode incoming packets. It is created during session
	// establishment, and loaded with the right key material. The session
	// layer's ConfidentialityLayerType field is set to this layer's type, so it
	// returns this as the NextLayerType() of encrypted packets. When sending a
	// message, this layer's SerializeTo is called before adding the session
	// wrapper.
	confidentialityLayer layerexts.SerializableDecodingLayer
}

// String returns a summary of the session's attributes on one line.
func (s *V2Session) String() string {
	return fmt.Sprintf("V2Session(Authentication: %v, Integrity: %v, Confidentiality: %v, LocalID: %v, RemoteID: %v, SIK: %v, K_1: %v, K_2: %v)",
		s.AuthenticationAlgorithm, s.IntegrityAlgorithm, s.ConfidentialityAlgorithm,
		s.LocalID, s.RemoteID,
		hex.EncodeToString(s.SIK),
		hex.EncodeToString(s.K(1)), hex.EncodeToString(s.K(2)))
}

func (s *V2Session) Version() string {
	return "2.0"
}

func (s *V2Session) ID() uint32 {
	return s.LocalID
}

func (s *V2Session) SendCommand(ctx context.Context, c ipmi.Command) (ipmi.CompletionCode, error) {
	timer := prometheus.NewTimer(commandDuration)
	defer timer.ObserveDuration()

	commandAttempts.WithLabelValues(c.Name()).Inc()
	code, err := s.sendCommand(ctx, c)
	if err != nil {
		commandFailures.WithLabelValues(c.Name()).Inc()
		return ipmi.CompletionCodeUnspecified, err
	}
	commandResponses.WithLabelValues(code.String()).Inc()
	return code, nil
}

func (s *V2Session) sendCommand(ctx context.Context, c ipmi.Command) (ipmi.CompletionCode, error) {
	s.rmcpLayer = layers.RMCP{
		Version:  layers.RMCPVersion1,
		Sequence: 0xFF, // do not send us an ACK
		Class:    layers.RMCPClassIPMI,
	}
	s.v2SessionLayer = ipmi.V2Session{
		Encrypted:                true,
		Authenticated:            true,
		ID:                       s.RemoteID,
		PayloadDescriptor:        ipmi.PayloadDescriptorIPMI,
		IntegrityAlgorithm:       s.integrityAlgorithm,
		ConfidentialityLayerType: s.confidentialityLayer.LayerType(),
	}
	s.messageLayer = ipmi.Message{
		Operation:     *c.Operation(),
		RemoteAddress: ipmi.SlaveAddressBMC.Address(),
		RemoteLUN:     ipmi.LUNBMC,
		LocalAddress:  ipmi.SoftwareIDRemoteConsole1.Address(),
		Sequence:      1, // used at the session level
	}

	response := []byte(nil)
	terminalErr := error(nil)
	firstAttempt := true
	retryable := func() error {
		if err := ctx.Err(); err != nil {
			terminalErr = err
			return nil
		}

		if firstAttempt {
			firstAttempt = false
		} else {
			commandRetries.Inc()
		}

		// TODO handle AuthenticationAlgorithmNone properly
		// TODO handle ConfidentialityAlgorithmNone properly
		s.AuthenticatedSequenceNumbers.Inbound++
		s.v2SessionLayer.Sequence = s.AuthenticatedSequenceNumbers.Inbound
		if err := gopacket.SerializeLayers(s.buffer, serializeOptions,
			&s.rmcpLayer,
			// session selector only used when decoding
			&s.v2SessionLayer,
			s.confidentialityLayer,
			&s.messageLayer,
			serializableLayerOrEmpty(c.Request())); err != nil {
			// this is not a retryable error
			terminalErr = err
			return nil
		}
		requestCtx, cancel := context.WithTimeout(ctx, time.Second*2) // TODO make configurable
		defer cancel()
		resp, err := s.transport.Send(requestCtx, s.buffer.Bytes())
		response = resp
		return err
	}
	s.backoff.Reset()
	if err := backoff.Retry(retryable, s.backoff); err != nil {
		return ipmi.CompletionCodeUnspecified, err
	}
	if terminalErr != nil {
		return ipmi.CompletionCodeUnspecified, terminalErr
	}
	if _, err := s.decode(response, &s.layers); err != nil {
		return ipmi.CompletionCodeUnspecified, err
	}

	// makes it easier to work with
	types := layerexts.DecodedTypes(s.layers)
	if err := types.InnermostEquals(ipmi.LayerTypeMessage); err != nil {
		return ipmi.CompletionCodeUnspecified, err
	}

	// if we have a message, we must have a session
	if c.Response() != nil {
		if err := c.Response().DecodeFromBytes(s.messageLayer.LayerPayload(), gopacket.NilDecodeFeedback); err != nil {
			return ipmi.CompletionCodeUnspecified, err
		}
	}
	return s.messageLayer.CompletionCode, nil
}

func (s *V2Session) GetSystemGUID(ctx context.Context) ([16]byte, error) {
	return getSystemGUID(ctx, s)
}

func (s *V2Session) GetChannelAuthenticationCapabilities(
	ctx context.Context,
	r *ipmi.GetChannelAuthenticationCapabilitiesReq,
) (*ipmi.GetChannelAuthenticationCapabilitiesRsp, error) {
	return getChannelAuthenticationCapabilities(ctx, s, r)
}

func (s *V2Session) GetDeviceID(ctx context.Context) (*ipmi.GetDeviceIDRsp, error) {
	cmd := &ipmi.GetDeviceIDCmd{}
	if err := ValidateResponse(s.SendCommand(ctx, cmd)); err != nil {
		return nil, err
	}
	return &cmd.Rsp, nil
}

func (s *V2Session) GetChassisStatus(ctx context.Context) (*ipmi.GetChassisStatusRsp, error) {
	cmd := &ipmi.GetChassisStatusCmd{}
	if err := ValidateResponse(s.SendCommand(ctx, cmd)); err != nil {
		return nil, err
	}
	return &cmd.Rsp, nil
}

func (s *V2Session) ChassisControl(ctx context.Context, c ipmi.ChassisControl) error {
	cmd := &ipmi.ChassisControlCmd{
		Req: ipmi.ChassisControlReq{
			ChassisControl: c,
		},
	}
	if err := ValidateResponse(s.SendCommand(ctx, cmd)); err != nil {
		return err
	}
	return nil
}

func (s *V2Session) closeSession(ctx context.Context) error {
	cmd := &ipmi.CloseSessionCmd{
		Req: ipmi.CloseSessionReq{
			ID: s.RemoteID,
		},
	}
	if err := ValidateResponse(s.SendCommand(ctx, cmd)); err != nil {
		return err
	}
	sessionsOpen.Dec()
	return nil
}

func (s *V2Session) Close(ctx context.Context) error {
	return s.closeSession(ctx)
}
