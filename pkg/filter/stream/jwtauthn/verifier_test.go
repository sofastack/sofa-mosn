package jwtauthn

import (
	"testing"

	envoycorev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	jwtauthnv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/jwt_authn/v3"
	"github.com/golang/mock/gomock"
	"github.com/golang/protobuf/ptypes/duration"
	"github.com/lestrrat/go-jwx/jwk"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/encoding/protojson"
)

func TestProviderVerifier(t *testing.T) {
	jwks, _ := jwk.Parse([]byte(publicKey))

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	t.Run("Ok JWT", func(t *testing.T) {
		config, err := getExampleConfig()
		if err != nil {
			t.Errorf("get example config: %v", err)
			t.FailNow()
		}

		jwksFetcher := NewMockJwksFetcher(ctrl)
		jwksFetcher.EXPECT().Fetch(gomock.Any()).Return(jwks, nil)

		verifier, err := NewVerifier(config.Rules[0].GetRequires(), config.Providers, nil, jwksFetcher)
		assert.Nil(t, err)

		headers := newHeaders(
			[2]string{"Authorization", "Bearer " + goodToken},
		)
		err = verifier.Verify(headers, "")
		assert.Nil(t, err)
		payloadValue, _ := headers.Get("sec-istio-auth-userinfo")
		assert.Equal(t, expectedPayloadValue, payloadValue)
	})

	t.Run("Missed JWT", func(t *testing.T) {
		config, err := getExampleConfig()
		if err != nil {
			t.Errorf("get example config: %v", err)
			t.FailNow()
		}

		verifier, err := NewVerifier(config.Rules[0].GetRequires(), config.Providers, nil, nil)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			t.FailNow()
		}

		headers := newHeaders()
		err = verifier.Verify(headers, "")
		assert.Equal(t, ErrJwtNotFound, err)
	})

	t.Run("JWT must be issued by the provider specified in the requirement.", func(t *testing.T) {
		configJSON := `
{
  "providers": {
    "example_provider": {
      "issuer": "https://example.com",
      "audiences": [
        "example_service",
        "http://example_service1",
        "https://example_service2/"
      ],
      "remoteJwks": {
        "httpUri": {
          "uri": "https://pubkey_server/pubkey_path",
          "cluster": "pubkey_cluster"
        }
      },
      "forwardPayloadHeader": "example-auth-userinfo"
    },
    "other_provider": {
      "issuer": "other_issuer",
      "forwardPayloadHeader": "other-auth-userinfo"
    }
  },
  "rules": [
    {
      "match": {
        "path": "/"
      },
      "requires": {
        "providerName": "other_provider"
      }
    }
  ]
}
`

		var config jwtauthnv3.JwtAuthentication
		if err := protojson.Unmarshal([]byte(configJSON), &config); err != nil {
			t.Errorf("unmarshal config: %v", err)
			t.FailNow()
		}
		provider := config.Providers[providerName]
		remoteJwks := &jwtauthnv3.RemoteJwks{}
		remoteJwks.HttpUri = &envoycorev3.HttpUri{
			Uri: "https://pubkey_server/pubkey_path",
			Timeout: &duration.Duration{
				Seconds: 5,
			},
			HttpUpstreamType: &envoycorev3.HttpUri_Cluster{
				Cluster: "pubkey_cluster",
			},
		}
		remoteJwks.CacheDuration = &duration.Duration{
			Seconds: 600,
		}
		provider.JwksSourceSpecifier = &jwtauthnv3.JwtProvider_RemoteJwks{
			RemoteJwks: remoteJwks,
		}

		config.Rules[0].GetRequires().RequiresType = &jwtauthnv3.JwtRequirement_ProviderName{
			ProviderName: "other_provider",
		}

		verifier, err := NewVerifier(config.Rules[0].GetRequires(), config.Providers, nil, nil)
		if err != nil {
			t.Errorf("create verifyer: %v", err)
			t.FailNow()
		}

		headers := newHeaders(
			[2]string{"Authorization", "Bearer " + goodToken},
		)
		err = verifier.Verify(headers, "")
		assert.Equal(t, ErrJwtUnknownIssuer, err)
		_, exists := headers.Get("other-auth-userinfo")
		assert.False(t, exists)
	})
}
