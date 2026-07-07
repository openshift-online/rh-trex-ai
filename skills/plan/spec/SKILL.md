---
name: spec
description: >
  Author a new specification following TRex spec format.
  Trigger phrases: "write a spec", "create spec", "new specification",
  "define requirements", "spec out"
---

# Spec Authoring

Author a new specification following the TRex spec format with RFC 2119 keywords.

## User Input
```text
$ARGUMENTS
```

## Steps

1. **Determine the domain** for the new spec:
   - `framework/` — core architecture patterns
   - `api/` — REST and gRPC conventions
   - `data/` — database and persistence patterns
   - `security/` — authentication, authorization, secrets
   - `codegen/` — code generation tools
   - `standards/` — cross-cutting conventions

2. **Assign a spec ID** following the domain prefix pattern:
   - FW-{next} for framework
   - API-{next} for api
   - DA-{next} for data
   - SEC-{next} for security
   - CG-{next} for codegen
   - STD-{next} for standards

3. **Write the spec** following this template:

   ```markdown
   # {Title} Specification

   **Date:** {YYYY-MM-DD}
   **Status:** Draft
   **ID:** {DOMAIN-NNN}
   **Related:** [links to related specs]
   **Implements:** `{code paths}`

   ---

   ## Purpose

   {One paragraph describing the spec's purpose}

   ## Requirements

   ### Requirement: {Name}

   {Description using RFC 2119: SHALL, MUST, SHOULD, MAY}

   #### Scenario: {Name}
   - GIVEN {precondition}
   - WHEN {action}
   - THEN {expected outcome}
   - AND {additional outcome}

   ## Design Decisions

   | Decision | Rationale |
   |----------|-----------|
   | ... | ... |
   ```

4. **Update the registries**:
   - Add the spec to `specs/{domain}/index.spec.md`
   - Add the spec to `specs/index.spec.md` registry table
   - Update the dependency order if it has dependencies

5. **Validate** the spec:
   - All requirements use RFC 2119 keywords
   - Every requirement has at least one scenario
   - `Implements` field points to real code paths
   - Dependencies are declared in the registry

## Related Specs
- `specs/index.spec.md` (registry)
