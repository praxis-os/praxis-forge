// SPDX-License-Identifier: Apache-2.0

// Package registry holds the typed component registry. Factories register at
// program start and are resolved by (Kind, ID) from declarative specs.
package registry

// Kind enumerates every component category Phase 1 knows about.
type Kind string

const (
	KindProvider           Kind = "provider"
	KindPromptAsset        Kind = "prompt_asset"
	KindToolPack           Kind = "tool_pack"
	KindPolicyPack         Kind = "policy_pack"
	KindPreLLMFilter       Kind = "pre_llm_filter"
	KindPreToolFilter      Kind = "pre_tool_filter"
	KindPostToolFilter     Kind = "post_tool_filter"
	KindBudgetProfile      Kind = "budget_profile"
	KindTelemetryProfile   Kind = "telemetry_profile"
	KindCredentialResolver Kind = "credential_resolver"
	KindIdentitySigner     Kind = "identity_signer"
)
