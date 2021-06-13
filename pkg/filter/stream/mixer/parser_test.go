package mixer

import (
	"testing"

	v1 "mosn.io/mosn/pkg/mixer/v1"
)

func TestParseMixerFilter(t *testing.T) {
	m := map[string]interface{}{
		"mixer_attributes": map[string]interface{}{
			"attributes": map[string]interface{}{
				"context.reporter.kind": map[string]interface{}{
					"string_value": "outbound",
				},
			},
		},
	}

	mixer, _ := ParseMixerFilter(m)
	if mixer == nil {
		t.Errorf("parse mixer config error")
	}

	if mixer.MixerAttributes == nil {
		t.Errorf("parse mixer config error")
	}
	val, exist := mixer.MixerAttributes.Attributes["context.reporter.kind"]
	if !exist {
		t.Errorf("parse mixer config error")
	}

	strVal, ok := val.Value.(*v1.Attributes_AttributeValue_StringValue)
	if !ok {
		t.Errorf("parse mixer config error")
	}
	if strVal.StringValue != "outbound" {
		t.Errorf("parse mixer config error")
	}
}
