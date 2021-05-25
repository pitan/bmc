package ipmi

import (
	"bytes"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

var RAKPMessage3Data = []struct {
	wire  []byte
	layer *RAKPMessage3
}{
	{
		// success
		[]byte{
			0x01,
			0x00,
			0x00, 0x00,
			0x01, 0x02, 0x03, 0x04,
			0x02, 0x01},
		&RAKPMessage3{
			BaseLayer: layers.BaseLayer{
				Contents: []byte{
					0x01, 0x00, 0x00, 0x00, 0x01, 0x02, 0x03, 0x04, 0x02, 0x01,
				},
			},
			Tag:                    0x01,
			Status:                 0x00,
			ManagedSystemSessionID: 0x4030201,
			AuthCode:               []byte{0x2, 0x1},
		},
	},
	{
		// failure
		[]byte{
			0x00,
			0x02,
			0x00, 0x00,
			0x04, 0x03, 0x02, 0x01},
		&RAKPMessage3{
			BaseLayer: layers.BaseLayer{
				Contents: []byte{
					0x00, 0x02, 0x00, 0x00, 0x04, 0x03, 0x02, 0x01,
				},
			},
			Tag:                    0x00,
			Status:                 0x02,
			ManagedSystemSessionID: 0x1020304,
		},
	},
}

func TestRAKPMessage3DecodeFromBytes(t *testing.T) {
	layer := &RAKPMessage3{}
	for _, test := range RAKPMessage3Data {
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

func TestRAKPMessage3SerializeTo(t *testing.T) {
	opts := gopacket.SerializeOptions{}
	for _, test := range RAKPMessage3Data {
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
