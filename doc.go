// SPDX-License-Identifier: Apache-2.0

// Package forge is the agent definition, composition, and materialization
// layer for the praxis stack.
//
// Positioning:
//
//	praxis          invocation kernel (single-agent loop, stateless per turn)
//	praxis-forge    declarative agent definition + composition + build
//	praxis-os       multi-agent orchestration and control plane (future)
//
// Forge takes a declarative AgentSpec, resolves registered components
// (providers, tool packs, policy packs, filters, MCP bindings, skills)
// through typed factories, validates the resulting composition, and
// materializes a reproducible BuiltAgent backed by a configured
// praxis Orchestrator.
//
// Non-goals (enforced by design):
//
//   - no multi-agent coordination, routing, or delegation
//   - no planners, supervisors, or workflow graphs
//   - no long-lived sessions or conversation state
//   - no runtime plugin loading or reflection-heavy magic
//   - no arbitrary executable config behavior
//
// Config selects registered behavior; it never defines arbitrary runtime
// behavior. Anything that smells like orchestration belongs in praxis-os.
//
// Phase 0 of the module contains only architecture documents. Phase 1
// will introduce the first vertical slice (spec loader, typed registry,
// materialization into *orchestrator.Orchestrator). See docs/ for the
// ADR, overview, spec model, registry interfaces, and mismatch analysis.
package forge
