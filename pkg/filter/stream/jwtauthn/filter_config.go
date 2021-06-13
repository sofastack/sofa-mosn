package jwtauthn

import (
	jwtauthnv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/jwt_authn/v3"
	"mosn.io/api"
)

// FilterConfig is the filter config interface.
type FilterConfig interface {
	BypassCorsPreflightRequest() bool
	// Finds the matcher that matched the request
	FindVerifier(headers api.HeaderMap, requestArg, requestPath string) Verifier
}

// NewFilterConfig creates a new filter config.
func NewFilterConfig(config *jwtauthnv3.JwtAuthentication) (FilterConfig, error) {
	var rulePairs []*MatcherVerifierPair
	for _, rule := range config.Rules {
		verifier, err := NewVerifier(rule.GetRequires(), config.Providers, nil, nil)
		if err != nil {
			return nil, err
		}
		rulePairs = append(rulePairs, &MatcherVerifierPair{
			matcher:  NewMatcher(rule),
			verifier: verifier,
		})
	}
	return &filterConfig{
		config:    config,
		rulePairs: rulePairs,
	}, nil
}

// MatcherVerifierPair is a pair of matcher and Verifier.
type MatcherVerifierPair struct {
	matcher  Matcher
	verifier Verifier
}

type filterConfig struct {
	config    *jwtauthnv3.JwtAuthentication
	rulePairs []*MatcherVerifierPair
}

func (f *filterConfig) BypassCorsPreflightRequest() bool {
	return f.config.BypassCorsPreflight
}

func (f *filterConfig) FindVerifier(headers api.HeaderMap, requestArg, requestPath string) Verifier {
	for _, rulePair := range f.rulePairs {
		if !rulePair.matcher.Matches(headers, requestPath) {
			continue
		}
		return rulePair.verifier
	}
	return nil
}
