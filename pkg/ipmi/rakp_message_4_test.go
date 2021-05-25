package ipmi

import (
	"bytes"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

var RAKPMessage4Data = []struct {
	wire  []byte
	layer *RAKPMessage4
}{
	{
		// success
		[]byte{
			0x00,
			0x00,
			0x00, 0x00,
			0xa4, 0xa3, 0xa2, 0xa0,
			0xa5, 0x33, 0xbe, 0xd8, 0x06, 0x65, 0x23, 0x14, 0xe0, 0xf0, 0x91, 0x6e, 0xaa, 0xe6, 0xa3, 0x6d, 0x1a, 0x9d, 0x2f, 0xac},
		&RAKPMessage4{
			BaseLayer: layers.BaseLayer{
				Contents: []byte{
					0x00, 0x00, 0x00, 0x00, 0xa4, 0xa3, 0xa2, 0xa0, 0xa5,
					0x33, 0xbe, 0xd8, 0x06, 0x65, 0x23, 0x14, 0xe0, 0xf0,
					0x91, 0x6e, 0xaa, 0xe6, 0xa3, 0x6d, 0x1a, 0x9d, 0x2f,
					0xac},
			},
			Tag:                    0,
			Status:                 StatusCodeOK,
			RemoteConsoleSessionID: 0xa0a2a3a4,
			ICV: []byte{
				0xa5, 0x33, 0xbe, 0xd8, 0x06, 0x65, 0x23, 0x14,
				0xe0, 0xf0, 0x91, 0x6e, 0xaa, 0xe6, 0xa3, 0x6d,
				0x1a, 0x9d, 0x2f, 0xac},
		},
	},
	{
		// failure
		[]byte{
			0x20,
			0x01,
			0x00, 0x00,
			0x1, 0x2, 0x3, 0x4},
		&RAKPMessage4{
			BaseLayer: layers.BaseLayer{
				Contents: []byte{
					0x20, 0x01, 0x00, 0x00, 0x1, 0x2, 0x3, 0x4},
			},
			Tag:                    0x20,
			Status:                 0x01,
			RemoteConsoleSessionID: 0x4030201,
		},
	},
}

func TestRAKPMessage4DecodeFromBytes(t *testing.T) {
	layer := &RAKPMessage4{}
	for _, test := range RAKPMessage4Data {
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

func TestRAKPMessage4SerializeTo(t *testing.T) {
	opts := gopacket.SerializeOptions{}
	for _, test := range RAKPMessage4Data {
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
