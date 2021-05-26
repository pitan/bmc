package ipmi

import (
	"bytes"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

var OpenSessionReqData = []struct {
	wire  []byte
	layer *OpenSessionReq
}{
	{
		[]byte{
			0x7b,
			0x02,
			0x00, 0x00,
			0x01, 0x04, 0x02, 0x03,
			0x00,       //Authentication Payload Type
			0x00, 0x00, //Authentication Payload reserved
			0x00, //Authentication PayloadLength
			0x00, 0x00, 0x00, 0x00,
			0x01,       //Integrity Payload Type
			0x00, 0x00, //reserved
			0x08, //
			0x01,
			0x00, 0x00, 0x00, //resetved
			0x02,       //Congidentiality Payload Type
			0x00, 0x00, //reserved
			0x08, //
			0x01,
			0x00, 0x00, 0x00, //reserved
			0x02,
			0x00, 0x00,
			0x08,
			0x02,
			0x00, 0x00, 0x00},
		&OpenSessionReq{
			BaseLayer: layers.BaseLayer{
				Contents: []byte{
					0x7b, 0x02, 0x00, 0x00, 0x01, 0x04, 0x02, 0x03,
					0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
					0x01, 0x00, 0x00, 0x08, 0x01, 0x00, 0x00, 0x00,
					0x02, 0x00, 0x00, 0x08, 0x01, 0x00, 0x00, 0x00,
					0x02, 0x00, 0x00, 0x08, 0x02, 0x00, 0x00, 0x00,
				},
			},
			Tag:               123,
			MaxPrivilegeLevel: PrivilegeLevelUser,
			SessionID:         0x03020401,
			AuthenticationPayloads: []AuthenticationPayload{
				{
					Wildcard: true,
				},
			},
			IntegrityPayloads: []IntegrityPayload{
				{
					Algorithm: IntegrityAlgorithmHMACSHA196,
				},
			},
			ConfidentialityPayloads: []ConfidentialityPayload{
				{
					Algorithm: ConfidentialityAlgorithmAESCBC128,
				},
				{
					Algorithm: ConfidentialityAlgorithmXRC4128,
				},
			},
		},
	},
}

func TestOpenSessionReqSerializeTo(t *testing.T) {
	opts := gopacket.SerializeOptions{}
	for _, test := range OpenSessionReqData {
		sb := gopacket.NewSerializeBuffer()
		if err := test.layer.SerializeTo(sb, opts); err != nil {
			t.Errorf("serialize %v = error %v, want %v", test.layer, err,
				test.wire)
			continue
		}
		got := sb.Bytes()
		if !bytes.Equal(got, test.wire) {
			t.Errorf("serialize %v = %v, want %v", test.layer, got, test.wire)
		}
	}
}

func TestOpenSessionReqDecodeFromBytes(t *testing.T) {
	layer := &OpenSessionReq{}
	for _, test := range OpenSessionReqData {
		err := layer.DecodeFromBytes(test.wire, gopacket.NilDecodeFeedback)
		switch {
		case err == nil && test.layer == nil:
			t.Errorf("decode %v succeeded with %v, wanted error", test.wire,
				layer)
		case err != nil && test.layer != nil:
			t.Errorf("decode %v failed with %v, wanted %v", test.wire, err,
				test.layer)
		case err == nil && test.layer != nil:
			if diff := cmp.Diff(test.layer, layer); diff != "" {
				t.Errorf("decode %v = %v, want %v: %v", test.wire, layer, test.layer, diff)
			}
		}
	}
}

var OpenSessionRspData = []struct {
	wire  []byte
	layer *OpenSessionRsp
}{
	{
		[]byte{
			0x00,
			0x00,
			0x04,
			0x00,
			0xa4, 0xa3, 0xa2, 0xa0,
			0x9c, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x08, 0x01, 0x00, 0x00, 0x00,
			0x01, 0x00, 0x00, 0x08, 0x01, 0x00, 0x00, 0x00,
			0x02, 0x00, 0x00, 0x08, 0x01, 0x00, 0x00, 0x00},
		&OpenSessionRsp{
			BaseLayer: layers.BaseLayer{
				Contents: []byte{
					0x00, 0x00, 0x04, 0x00, 0xa4, 0xa3, 0xa2, 0xa0, 0x9c,
					0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x08, 0x01, 0x00,
					0x00, 0x00, 0x01, 0x00, 0x00, 0x08, 0x01, 0x00, 0x00,
					0x00, 0x02, 0x00, 0x00, 0x08, 0x01, 0x00, 0x00, 0x00},
			},
			Tag:                    0,
			Status:                 StatusCodeOK,
			MaxPrivilegeLevel:      PrivilegeLevelAdministrator,
			RemoteConsoleSessionID: 0xa0a2a3a4,
			ManagedSystemSessionID: 0x9c,
			AuthenticationPayload: AuthenticationPayload{
				Algorithm: AuthenticationAlgorithmHMACSHA1,
			},
			IntegrityPayload: IntegrityPayload{
				Algorithm: IntegrityAlgorithmHMACSHA196,
			},
			ConfidentialityPayload: ConfidentialityPayload{
				Algorithm: ConfidentialityAlgorithmAESCBC128,
			},
		},
	},
}

func TestOpenSessionRspSerializeTo(t *testing.T) {
	opts := gopacket.SerializeOptions{}
	for _, test := range OpenSessionRspData {
		sb := gopacket.NewSerializeBuffer()
		if err := test.layer.SerializeTo(sb, opts); err != nil {
			t.Errorf("serialize %v = error %v, want %v", test.layer, err,
				test.wire)
			continue
		}
		got := sb.Bytes()
		if !bytes.Equal(got, test.wire) {
			t.Errorf("serialize %v = %v, want %v", test.layer, got, test.wire)
		}
	}
}

func TestOpenSessionRspDecodeFromBytes(t *testing.T) {
	layer := &OpenSessionRsp{}
	for _, test := range OpenSessionRspData {
		err := layer.DecodeFromBytes(test.wire, gopacket.NilDecodeFeedback)
		switch {
		case err == nil && test.layer == nil:
			t.Errorf("decode %v succeeded with %v, wanted error", test.wire,
				layer)
		case err != nil && test.layer != nil:
			t.Errorf("decode %v failed with %v, wanted %v", test.wire, err,
				test.layer)
		case err == nil && test.layer != nil:
			if diff := cmp.Diff(test.layer, layer); diff != "" {
				t.Errorf("decode %v = %v, want %v: %v", test.wire, layer, test.layer, diff)
			}
		}
	}
}
