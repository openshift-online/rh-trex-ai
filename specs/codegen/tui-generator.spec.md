# TUI Generator Specification

**Date:** 2026-08-04
**Status:** Active
**ID:** CG-006
**Related:** [OpenAPI Intermediate Representation](openapi-ir.spec.md), [REST Conventions](../api/rest-conventions.spec.md), [Authentication](../security/authentication.spec.md), [Testing Standards](../standards/testing.spec.md), [Dependency Supply Chain](../standards/dependency-supply-chain.spec.md)
**Implements:** `scripts/tui-generator/`, `pkg/tui/`, `pkg/cmd/tui.go`, `data/generated/tui/`, `cmd/trex/main.go`, `Makefile`

---

## Purpose

Define an OpenAPI-generated, keyboard-driven terminal resource browser compiled directly into the primary service executable as its `tui` subcommand. The generator projects resource views, relationships, operations, and presentation-only `x-trex-tui` metadata into deterministic descriptors consumed by one reusable Bubble Tea runtime; it does not encode a fixed resource hierarchy or resource-specific client implementation.

## Conceptual Model

```text
resolved OpenAPI -> canonical IR -> TUI descriptors -> generic runtime
```

| Component | Responsibility |
|-----------|----------------|
| Canonical IR | Owns OpenAPI loading, operation and schema fidelity, resource views, relationships, capabilities, security state, and source diagnostics |
| TUI projection | Validates presentation metadata and produces deterministic view, operation, relationship, binding, and authentication descriptors |
| Generic runtime | Composes reusable pages and presentation components, executes descriptor-defined requests, manages filtering and the navigation stack, and sanitizes terminal content |
| Generated descriptor package | Embeds only the service-specific deterministic descriptors inside the service module under `data/generated/tui` |
| Service command | Registers `tui` on the primary Cobra root and constructs the reusable runtime directly without a child process, sidecar executable, or runtime source copy |

## Presentation Principles

| Principle | Contract |
|-----------|----------|
| Service-neutral identity | Chrome SHALL derive identity and connection context from OpenAPI and runtime configuration and SHALL contain no TRex- or resource-specific branding |
| One application shell | Header, command bar, page frame, breadcrumb footer, alert rail, modal host, sizing, and focus coordination SHALL each have one shared implementation |
| Semantic composition | Pages SHALL describe content and local actions by composing shared components; they SHALL NOT recreate chrome, dialogs, alerts, key handling, or styles |
| One source of presentation truth | Theme tokens, continuous layout policy, global keybindings, hints, alert policy, and dialog behavior SHALL each be defined centrally |
| Capability-derived interaction | Controls and forms SHALL be produced from canonical operation descriptors and SHALL NOT be added for a literal resource name |
| Responsive degradation | Constrained terminals SHALL compress content and expose navigable overflow before omitting navigation, active context, or alert visibility |
| Spatially stable errors | Every error SHALL have a sanitized summary in one bottom-anchored alert rail whose terminal row does not move between pages or interaction modes |
| Safe rendering boundary | All presentation components SHALL sanitize untrusted metadata and response content at the final rendering boundary |
| Deterministic output | The same descriptors, state, terminal size, and theme SHALL produce the same visible component tree and generated source |

## Reusable Presentation Architecture

The generic runtime SHALL compose screens through the following shell. The alert rail is the final terminal row and SHALL remain visible beneath page content, command input, and modal overlays.

```text
+ Header: service | server | auth | scope | refresh | contextual hints +
+ Command/filter bar (conditional)                                  +
+ Page frame: kind(scope)[count] -----------------------------------+
| active list, detail, stream, loading, empty, or fatal page         |
+ Breadcrumb/footer: actual navigation stack + global hints --------+
+ Alert rail: fixed bottom row; info | success | warning | error ----+
             modal host overlays only the page frame
```

| Component | Responsibility |
|-----------|----------------|
| Application shell | Owns full-screen layout, focus, sizing, component placement, and page/modal composition |
| Header | Renders service-neutral identity, connection, authentication, scope, refresh, and contextual action state |
| Command/filter bar | Provides the shared `:` command and `/` filter input surface and returns its space to the page when inactive |
| Page frame | Provides the shared title, border, loading, empty, forbidden, and stale presentation around active page content |
| Resource table page | Renders descriptor-defined lists through one reusable selectable, sortable, filterable, horizontally scrollable table component |
| Detail page | Renders readable fields through one reusable scrollable key-value component |
| Stream page | Renders bounded event output and stream state through one reusable viewport component |
| Breadcrumb/footer | Renders the actual navigation stack and globally applicable hints |
| Alert manager and rail | Applies one severity, lifetime, redaction, queuing, and fixed-location policy to every alert and error |
| Modal host | Owns overlay placement and focus for at most one help, choice, confirmation, or form dialog |
| Dialog primitives | Provide shared frame, buttons, focus order, cancellation, validation, and in-flight behavior |
| Theme and layout | Provide semantic styles and deterministic continuous measurements without imposing a minimum terminal width |
| Keybinding registry | Drives dispatch, reserved-key validation, contextual hints, and help from one definition |

## Requirements

### Requirement: Canonical IR Consumption

The TUI generator SHALL be the fourth in-repository consumer of the canonical OpenAPI IR. It SHALL obtain operations, schemas, resource views, relationships, capabilities, servers, parameters, and security states from that IR and SHALL NOT traverse raw OpenAPI documents to reinterpret them. A breaking IR change SHALL update and test the TUI generator atomically with the SDK, CLI, and console consumers as required by CG-005.

#### Scenario: Generate from a normalized document

- GIVEN the canonical IR contains a global collection, a parent-scoped collection, and their operations
- WHEN the TUI projection builds its descriptors
- THEN every descriptor SHALL refer to canonical IR identities and semantics
- AND no TUI parser SHALL independently infer those semantics from raw YAML or URL strings

### Requirement: Descriptor-Driven Generic Runtime

The generated TUI SHALL use a generic runtime driven by generated descriptors. Descriptors SHALL retain each view's operations, represented schemas, ordered columns, relationships, parameter-binding plans, capabilities, servers, and security state. Runtime source SHALL NOT contain hard-coded resource kind names, fixed parent-child chains, or resource-specific fetch functions.

#### Scenario: Add a new resource without runtime code

- GIVEN an OpenAPI change adds a valid resource view and no new terminal interaction primitive
- WHEN the TUI is regenerated
- THEN generated descriptors SHALL be sufficient to browse the new view
- AND the generic runtime source SHALL remain unchanged

### Requirement: Integrated Service Subcommand

The generator SHALL produce only a deterministic descriptor package beneath `data/generated/tui` in the service module. The primary service Cobra root SHALL register that package with a `tui` subcommand, and the primary service executable SHALL link the shared `pkg/tui` runtime and all required Bubble Tea dependencies directly. The generated package SHALL NOT contain a `go.mod`, executable entry point, copied runtime source, command wrapper, or shell invocation. The repository SHALL NOT generate or support a separate `trex-tui` executable.

The `tui` subcommand SHALL accept no positional arguments and SHALL preserve the existing `--server`, `--token-file`, `--insecure`, repeatable `--trust-origin`, and `--refresh-interval` behavior. It SHALL default the server from the generated OpenAPI descriptor, read credentials before starting the terminal program, construct the generic model directly, and run it in the alternate screen. The standard primary-binary build and install targets SHALL regenerate the embedded TUI descriptor before compiling so users build the CLI and TUI as one executable through one top-level target. Building the primary service executable and rendering its command help SHALL require no database or running service; only interactive API use SHALL require a reachable configured server.

#### Scenario: Build and discover the integrated command

- GIVEN the repository OpenAPI document is valid for TUI projection
- WHEN the user invokes the standard primary-binary build target and renders the resulting root help
- THEN that one target SHALL regenerate the embedded TUI descriptor and compile the primary executable
- AND the executable SHALL build successfully and list the `tui` subcommand
- AND the executable SHALL contain the generated descriptors and reusable runtime without locating or executing another binary or runtime file
- AND no separate TUI build target or executable SHALL be required for ordinary build, test, install, or use
- AND descriptor regeneration SHALL NOT query or otherwise require an interactive terminal
- AND the build and help commands SHALL NOT require a database or running TRex service

#### Scenario: Launch with the established runtime options

- GIVEN the integrated descriptor declares a default server
- WHEN the user runs `trex tui` with any supported connection, credential, trust, security, or refresh option
- THEN the command SHALL pass the resolved values directly into the generic TUI client configuration
- AND invalid descriptor, credential-file, client, or Bubble Tea startup errors SHALL return through Cobra after terminal restoration
- AND any positional argument SHALL be rejected before the TUI starts

### Requirement: Full-Screen Application Shell

The generated TUI SHALL run as a Bubble Tea alternate-screen application whose shared application shell owns all fixed presentation regions. In order from top to bottom, those regions SHALL be the header, conditional command or filter bar, page frame, breadcrumb footer, and alert rail. Opening or closing the command bar SHALL resize only the page frame. A page SHALL NOT render a second header, footer, alert area, or outer frame.

#### Scenario: Move between pages without moving chrome

- GIVEN the user moves from a resource table to detail and then stream pages
- WHEN each page is rendered at the same terminal size
- THEN the header, breadcrumb footer, and alert rail SHALL occupy the same terminal rows
- AND only the content inside the page frame SHALL change

### Requirement: Service-Neutral Header and Semantic Theme

The shared header's left region SHALL render sanitized `Key: Value` rows whose keys use one consistent semantic style and whose values retain their own semantic style. Its first row SHALL be `Service: <OpenAPI service title>` without appending the active page name. When the header has at least three rows, it SHALL leave flexible blank padding below that row, render `Context: <active server origin>` on the penultimate header row, and render `Status: <authentication state, active scope, and refresh state>` on the final header row. Authentication state SHALL contain no credential material. The page frame and breadcrumb SHALL remain authoritative for active page identity. When fewer than three header rows are available, the shell SHALL preserve complete left-region rows without overlap in Service, Context, then Status priority order. It SHALL omit unavailable optional values rather than display invented placeholders. Shared runtime source SHALL contain no TRex-specific logo, service name, resource kind, or color rule.

The runtime SHALL define semantic theme tokens for primary, secondary, normal, muted, success, warning, danger, border, selected foreground and background, detail keys and values, and raw-code keys, strings, numbers, literals, and punctuation. Pages and domain components SHALL use those tokens and SHALL NOT define raw terminal colors or ad hoc Lip Gloss styles.

#### Scenario: Generate for a differently named service

- GIVEN an OpenAPI document is titled `Inventory API` and configures an authenticated HTTPS server
- WHEN its TUI opens a scoped collection
- THEN the header SHALL place `Service: Inventory API` at the upper-left corner without the collection name
- AND SHALL place `Context: https://<active-origin>` on the penultimate left-region row
- AND SHALL place `Status: authenticated` plus current scope and refresh state on the final left-region row
- AND any rows between the service title and active origin SHALL be blank in the left region
- AND no TRex name, dinosaur label, or hard-coded service color SHALL appear

### Requirement: Contextual Header Shortcut Palette

The shared header SHALL render currently applicable keyboard shortcuts as a k9s-style, multi-row palette whose entries use the form `<key> Action`. The palette SHALL occupy the upper-right region alongside the vertically anchored left service region rather than consume a separate block below it. Every shortcut column SHALL have the same display-cell width, every entry SHALL be left-aligned within its column, and the complete palette block SHALL be right-aligned to the terminal edge. Within every palette column, a shared display-cell key width SHALL pad the complete `<key>` token so every Action begins at the same relative display-cell offset regardless of key length. The palette SHALL derive fixed bindings and generated operation hotkeys exclusively from the single keybinding registry used for dispatch and help. It SHALL preserve stable registry order and use no more than six shortcut rows. Hidden, unavailable, or inapplicable capabilities SHALL NOT appear.

The shared layout SHALL render only complete shortcut entries. When terminal width or height is constrained, it SHALL elide lower-priority entries deterministically before higher-priority entries, retain the help shortcut whenever any shortcut row can be rendered, and restore elided entries when space returns. The complete applicable binding set SHALL remain available through the help dialog. The palette SHALL NOT be duplicated in the breadcrumb, alert rail, or a separate bottom shortcut strip, and its layout SHALL NOT use fixed width breakpoints or a minimum terminal width.

#### Scenario: Show only current page capabilities

- GIVEN a table supports navigation, detail, sorting, horizontal scrolling, and one generated operation hotkey but does not support delete
- WHEN the table receives focus
- THEN the top-right header palette SHALL show the applicable fixed and generated shortcuts as `<key> Action` entries in equal-width columns
- AND the Action text following `<q>`, `<ctrl+x>`, and every other visible key token SHALL begin at the same relative display-cell offset within its shortcut column
- AND the service title, active server, and connection status SHALL retain their top, penultimate, and final left-region rows
- AND those entries and the help dialog SHALL use the same bindings that dispatch the actions
- AND no delete shortcut or separate bottom shortcut strip SHALL be rendered

#### Scenario: Continuously elide and restore shortcuts

- GIVEN more applicable shortcuts than fit in a constrained terminal
- WHEN the terminal is narrowed, shortened, and later restored
- THEN only complete lower-priority entries SHALL disappear in deterministic order
- AND the help shortcut SHALL remain visible whenever at least one shortcut row is available
- AND the shortcut block SHALL remain right-aligned without overlapping the left header region
- AND the page identity, breadcrumb, and fixed alert rail SHALL retain their locations
- AND widening or lengthening the terminal SHALL restore the applicable entries in stable registry order

### Requirement: Centralized Responsive Layout

One shared layout component SHALL calculate all shell and content dimensions continuously from Bubble Tea window-size messages. It SHALL NOT impose a minimum terminal width, use fixed width breakpoints, or replace the active page with a terminal-too-small screen. As horizontal space contracts, it SHALL first omit optional header metadata and low-priority hints according to measured fit, then compress table columns to their shared minimums and expose horizontal overflow rather than remove declared columns solely because of terminal width. An active command/filter prompt SHALL receive three rows for its complete border whenever three rows remain after the fixed alert and breadcrumb rows; only an extremely short terminal that cannot contain all three rows MAY use a clipped fallback. All dimensions SHALL be clamped to non-negative values, and child components SHALL NOT perform independent terminal-size arithmetic. Constrained layout SHALL preserve active page identity, navigation, overflow affordances, and the alert rail whenever the terminal has rows available for them.

#### Scenario: Continuously constrain and restore the terminal

- GIVEN a populated table and an active persistent error
- WHEN the terminal width repeatedly shrinks below the bounded table width and later grows
- THEN the shared layout SHALL continuously allocate non-negative component dimensions without a minimum-width replacement screen
- AND columns beyond the available width SHALL remain reachable through horizontal scrolling
- AND the error summary SHALL remain on the bottom terminal row
- AND growing the terminal SHALL restore the page without corrupting selection or navigation state

### Requirement: Reusable Presentation Component Architecture

The shared runtime SHALL have exactly one implementation for each shell and presentation primitive named in the Reusable Presentation Architecture. Pages SHALL compose those primitives using semantic data and SHALL NOT draw their own outer borders, create their own theme, implement global key dispatch, position dialogs, or manage alert lifetimes. Resource-specific names, routes, schemas, and operation rules SHALL exist only in generated descriptors and runtime state, not in reusable presentation source.

#### Scenario: Add a page without duplicating presentation policy

- GIVEN a future descriptor introduces a supported page type
- WHEN that page is implemented
- THEN it SHALL receive layout, theme, keys, alerts, and modal services from the shared shell
- AND it SHALL NOT require a copied header, footer, alert, dialog, or style implementation

### Requirement: Unified Page Contract

Every resource-catalog, list, detail, stream, loading, empty, forbidden, and fatal page SHALL implement one shared page lifecycle and SHALL provide only semantic page title, scope, count, content, local actions, and state to the application shell. The shell SHALL remain responsible for sizing, chrome, global keys, breadcrumbs, alerts, and modal overlays. Replacing a page SHALL preserve the shell, applicable alerts, navigation stack, and focus policy.

#### Scenario: Replace a loading page with content

- GIVEN a resource page is initially loading and an alert is already visible
- WHEN the request completes and the table page replaces the loading state
- THEN the shared shell and alert SHALL remain mounted in the same locations
- AND the table SHALL receive the page-frame dimensions without recreating chrome

### Requirement: Shared Resource Table Page

The top-level resource catalog and all collection views SHALL use one reusable resource-table page backed by the Bubbles table component. Descriptor collection views SHALL render a descriptor-derived `kind(context)[count]` title centered in the page frame's top border, with unscoped context rendered as `all`. The synthetic home catalog SHALL instead render the simple centered title `Resources` without a synthetic context or count. Kind, context, and count SHALL use distinct semantic theme styles, and any non-ready page state SHALL use a fourth state-appropriate semantic style. While a non-empty table filter is active, the same centered title SHALL append one visually distinct `</filter>` badge containing the sanitized active expression; the badge SHALL update with live filter input, remain after the prompt closes, and disappear immediately when the filter is cleared. A descriptor collection count SHALL describe the filtered visible rows.

The shared table theme SHALL render every unselected data-row foreground with the theme's selection-highlight accent. The selected full row SHALL use that exact same accent as its background with an explicitly black foreground so every selected cell remains legible. These foreground and background values SHALL override the Bubbles default table styles rather than inherit only unset properties. Headers SHALL retain the primary semantic style. The component SHALL also render full-row selection, active sort and filter state, deterministic adaptive columns, and shared loading, empty, forbidden, and stale-data states. An active ascending or descending sort marker SHALL be a protected left prefix of its header label, such as `↑ NAME`, so right-side ellipsis truncation cannot hide the sort state. Sorting, filtering, selection restoration by validated identity, and row navigation SHALL be implemented once for every resource descriptor and the catalog.

#### Scenario: Distinguish selected and unselected resource rows

- GIVEN a resource table contains two visible rows
- WHEN one row is selected
- THEN every cell in the selected row SHALL render with a black foreground on the selection-highlight background
- AND every unselected row cell SHALL use that exact selection-highlight color as its foreground
- AND the table header SHALL retain its primary semantic foreground

#### Scenario: Render unrelated collection schemas

- GIVEN two collection descriptors have different labels, scopes, columns, and identities
- WHEN the user switches between them
- THEN the same resource-table component SHALL render both using their descriptors
- AND each `kind(context)[count]` label SHALL remain centered in the frame border with visually distinct kind, context, and count segments
- AND neither view SHALL own a duplicated table setup, empty state, sorting function, or selection policy

#### Scenario: Keep sort direction visible in a narrow column

- GIVEN the active sort column has a header label wider than its allocated width
- WHEN the shared table truncates the header with a right-side ellipsis
- THEN the ascending or descending marker SHALL remain visible at the left edge
- AND changing sort direction SHALL replace that prefix without changing the declared label

### Requirement: Shared Breadcrumb Trail

The shared shell SHALL render navigation history through one reusable breadcrumb component. Each segment SHALL be a padded `<segment>` badge rather than separator-delimited plain text, with its sanitized resource label lowercased and any sanitized selected identity retained in brackets. Ancestor badges and the active rightmost badge SHALL use distinct semantic background and foreground styles, and the active badge SHALL remain visually identifiable when it is the only segment. Breadcrumb order and labels SHALL derive exclusively from the navigation stack. Pages SHALL NOT render or style breadcrumb segments themselves.

The breadcrumb component SHALL measure terminal display cells and render only complete badges. When all badges do not fit, it SHALL elide the oldest ancestors first while preserving the active rightmost badge whenever the available row can contain it. Widening the terminal SHALL restore elided ancestors in navigation order.

#### Scenario: Render and constrain a nested navigation trail

- GIVEN the user navigates from a resource collection through a related collection to an item detail
- WHEN the breadcrumb row is spacious
- THEN it SHALL render one padded `<segment>` badge for every navigation frame in order
- AND the active rightmost badge SHALL have a different semantic foreground and background from its ancestors
- WHEN the terminal narrows until the complete trail no longer fits
- THEN only complete badges SHALL remain
- AND the oldest ancestors SHALL disappear before the active rightmost badge

### Requirement: Content-Aware Column Sizing and Horizontal Overflow

The shared resource-table component SHALL calculate column widths from sanitized terminal display cells rather than byte length, rune count, or equal division of the viewport. For each declared column, its natural width SHALL be the maximum display width of its header including the protected left-prefix active sort decoration and every value in the currently loaded unfiltered result, plus the shared inter-column gutter. The sizing pass SHALL correctly measure combining characters, wide Unicode characters, and emoji and SHALL remain stable while the user scrolls rows or changes a filter.

One centralized sizing policy SHALL define and test semantic minimum widths, maximum widths, gutters, and expansion weights. Natural widths SHALL be clamped to those bounds. When bounded columns fit, unused space SHALL be distributed deterministically to eligible flexible text columns without needlessly expanding compact identifiers, statuses, booleans, or numbers. When they do not fit, lower-priority flexible columns SHALL shrink before higher-priority columns, but no declared column SHALL become inaccessible solely because of terminal width. Values wider than a column's maximum or current allocated width SHALL be truncated at a display-cell boundary with an ellipsis, while the complete sanitized value remains available in item detail.

Any remaining overflow SHALL form one horizontal table canvas controlled by the keybinding registry. While the table has focus, Left and Right arrow keys SHALL move the viewport by one column boundary and SHALL NOT change row selection. Each navigation frame SHALL retain its horizontal offset across filtering, sorting, refresh, detail round trips, and back navigation; a newly opened resource view SHALL begin at its left edge, and resize SHALL clamp an invalid offset to the nearest valid boundary.

The table chrome SHALL reserve non-data space for directional overflow indicators. A right indicator SHALL be visible whenever any column is fully or partially beyond the right edge, a left indicator SHALL be visible whenever content exists beyond the left edge, both SHALL be visible in the middle, and neither SHALL be visible when all columns fit. The indicators SHALL report the number of off-screen columns and SHALL be accompanied by a contextual `Left/Right: columns` hint from the shared keybinding registry. They SHALL NOT cover a header, cell value, scrollbar, breadcrumb, or alert.

#### Scenario: Size heterogeneous fields by their content

- GIVEN a table has a short numeric field, a medium identifier, a long free-text field, combining characters, CJK characters, and emoji
- WHEN the table is rendered at a width where its bounded natural columns fit
- THEN each width SHALL be based on sanitized terminal display cells across all loaded rows
- AND compact scalar columns SHALL NOT receive the same width as the long flexible text column
- AND row scrolling and filtering SHALL NOT cause column widths to jump

#### Scenario: Reveal every overflowing column

- GIVEN bounded table columns exceed the available page-frame width
- WHEN the table first renders, moves right twice, reaches the right edge, moves left, and is resized
- THEN only the right indicator SHALL appear at the left edge, both indicators SHALL appear in the middle, and only the left indicator SHALL appear at the right edge
- AND each indicator SHALL accurately report its off-screen column count after movement and resize
- AND each arrow press SHALL move exactly one column boundary without changing row selection
- AND every declared column SHALL be reachable without hiding the breadcrumb footer or fixed alert rail

### Requirement: Shared Detail and Stream Pages

Every item detail SHALL use one reusable scrollable key-value page with deterministic readable-field ordering and shared wrapping or truncation policy. Its field-name column SHALL use the dim detail-key foreground, be right-aligned to the widest visible field name subject to a one-third-content-width cap, omit colon punctuation, and be separated from the bright-white detail value by exactly two cells. Wrapped value continuations SHALL begin at the same value-column offset as their originating value. Detail layout SHALL be recomputed after a terminal resize. When an API resource row or loaded item is selected, the contextual `r` shortcut SHALL open a raw-resource presentation in the same shared scrollable detail viewport. The raw presentation SHALL format the selected decoded API object as indented JSON, preserve its object, array, scalar, and null structure, sanitize every untrusted key and string value at the rendering boundary, and perform no additional API request. It SHALL apply deterministic JSON syntax highlighting through semantic raw-code tokens for object keys, string values, numbers, boolean and null literals, and punctuation; whitespace SHALL remain unchanged, and removing presentation styles SHALL reproduce the exact sanitized indented JSON. `Esc` SHALL return to the immediately preceding list or item-detail presentation with selection, filter, sort, horizontal offset, and navigation history unchanged. The `r` shortcut SHALL be absent when no API resource object is selected and SHALL be reserved against generated operation hotkeys.

Every streaming operation SHALL use one reusable viewport page with connection state, autoscroll state, and a deterministically bounded event buffer. Detail, raw-resource, and stream pages SHALL receive their frame, header, footer, alerts, sizing, and global keys from the shell and SHALL NOT implement alternate chrome.

#### Scenario: Inspect the selected API resource as raw JSON

- GIVEN a collection row is selected from an API response containing nested objects, arrays, typed scalars, and an untrusted terminal-control string
- WHEN the user presses `r`
- THEN the shared detail viewport SHALL show the selected row as indented structurally equivalent JSON without issuing an API request
- AND the source shortcut palette SHALL advertise `<r> raw` while the raw frame SHALL identify the raw presentation
- AND terminal-control or framework-markup content SHALL be neutralized before rendering
- AND the JSON body SHALL distinguish keys, strings, numbers, boolean and null literals, and punctuation through semantic raw-code colors
- AND stripping those presentation styles SHALL reproduce the exact sanitized indented JSON, including its whitespace
- WHEN the user presses `Esc`
- THEN the original collection, selection, filter, sort, horizontal offset, and navigation history SHALL be restored unchanged

#### Scenario: Preserve shell while streaming

- GIVEN the user opens a streaming operation and events exceed the viewport and buffer limit
- WHEN the stream page drops the oldest buffered events and renders the newest safe content
- THEN the stream's connection and autoscroll state SHALL remain visible inside the page frame
- AND the header, breadcrumb footer, and alert rail SHALL remain unchanged

### Requirement: Command, Filter, and Help Chrome

The shell SHALL own one command/filter prompt for both `:` resource and action commands and `/` table filtering. While active and three terminal rows are available, the prompt SHALL render as one complete full-width border with a middle input row, use mode-specific semantic border color, and display `🦖>` for resource commands or `🦕/` for filters followed by the current input and any completion suffix through shared prompt styles. The underlying input widget SHALL NOT add a second prompt token. It SHALL appear only while one of those modes is active, return all of its rows to the page when closed, and use shared input, completion, validation, cancellation, and history behavior. Resource completion SHALL be derived only from globally addressable views and scoped views whose bindings are currently available. It SHALL update an inline visually muted suffix after every edit; `Up` and `Down` SHALL cycle deterministic matching candidates; and `Tab`, `Right`, or `Ctrl+F` SHALL accept the displayed suffix. Filter history SHALL remain available without fabricating resource suggestions. A shared help dialog SHALL derive its content from the keybinding registry and current page capabilities rather than from separately maintained help text.

#### Scenario: Enter and leave filter mode

- GIVEN a resource table is visible with a persistent error
- WHEN the user enters `/` filter mode, changes the filter, and presses `Esc`
- THEN one shared fully bordered command/filter prompt SHALL appear and then close
- AND the table SHALL regain the three released page rows
- AND the alert SHALL remain on the same bottom row throughout

#### Scenario: Complete an available resource

- GIVEN two addressable resource aliases match the current `:` input and one scoped view lacks its required binding
- WHEN the user types, cycles the inline suggestions, and accepts one with `Right`
- THEN candidates SHALL update after each edit in deterministic order
- AND the accepted suffix SHALL complete the selected available resource without inserting the unavailable view
- AND `Tab` and `Ctrl+F` SHALL provide the same acceptance behavior

### Requirement: Single Keybinding and Hint Registry

One keybinding registry SHALL be authoritative for global and local dispatch, contextual hints, help content, reserved-key validation, and conflict diagnostics. Global navigation, command, filter, help, cancellation, quit, confirmation, and focus keys SHALL be declared once. Generated operation hotkeys SHALL be registered through this authority, SHALL NOT override a global binding, and SHALL be shown only while their capability is applicable.

#### Scenario: Keep dispatch, hints, and help aligned

- GIVEN a page supports create and stream actions but not delete
- WHEN its controls, header shortcut palette, and help dialog are rendered
- THEN all three SHALL derive their applicable bindings from the same registry
- AND no delete binding or stale help entry SHALL be displayed or dispatched
- AND no stale or duplicate shortcut entry SHALL appear below the page

### Requirement: Consistent Alert and Error Rail

The shell SHALL reserve its final terminal row for one shared alert rail on every render, including when no alert is active. Every operational, validation, authentication, network, stream, configuration, and fatal error SHALL produce a sanitized and credential-redacted summary in that rail. Field validation SHALL additionally appear beside its field, recoverable failures SHALL preserve and mark stale content, and fatal startup or configuration failures SHALL additionally render the shared fatal page; those secondary presentations SHALL NOT replace or move the rail summary. A foreground API request failure SHALL additionally open the shared error dialog immediately. Validation and other local errors SHALL NOT open that dialog. Background polling failures SHALL remain non-disruptive in the rail and SHALL remain available through the alert-details shortcut rather than stealing focus on every refresh interval.

Alerts SHALL have semantic info, success, warning, and error severities. Info and success alerts SHALL expire after five seconds. Warning and error alerts SHALL persist until explicit dismissal or a successful retry or relevant corrective action. A lower-severity alert SHALL NOT displace a persistent higher-severity alert; queued alerts SHALL be ordered deterministically. The rail SHALL truncate or summarize within its fixed row using the shared layout policy while retaining full safe details in alert state for a shared detail presentation.

Successful initial list and detail reads, navigation reads, and background refreshes SHALL update content, stale state, and the header refresh timestamp without creating info or success alerts. Success alerts SHALL be reserved for successful user-initiated operations or explicit lifecycle transitions that require acknowledgement; a post-operation read used only to refresh the active view SHALL NOT replace the operation's success message with a routine load message.

#### Scenario: Error location remains constant

- GIVEN a request fails while a table is visible
- WHEN the user opens a dialog, enters command mode, navigates to detail, and resizes the terminal
- THEN the error summary SHALL remain in the final terminal row at every supported size
- AND the stale table or detail content SHALL remain available
- AND no page or dialog SHALL render a competing error location

#### Scenario: Inline validation also reaches the rail

- GIVEN a required form field is empty
- WHEN the user submits the form
- THEN the field SHALL display its inline validation message
- AND the shared alert rail SHALL display one summary of the validation failure in its fixed location
- AND no request SHALL be sent

#### Scenario: Open a foreground API failure without losing context

- GIVEN the user initiates a documented API operation from a collection, detail, form, or confirmation presentation
- WHEN the request fails because of transport, authentication, authorization, server status, response-size, response-read, or response-decode behavior
- THEN the persistent alert rail SHALL show a concise sanitized error summary
- AND the shared error dialog SHALL open over the unchanged source page or operation presentation
- AND dismissing the dialog SHALL restore that source presentation and its selection, inputs, navigation stack, filter, sort, and offsets
- AND a failed action that has an input form SHALL return to that retained editable form with every entered value preserved
- AND a periodic background-refresh failure SHALL NOT open or reopen the dialog automatically

#### Scenario: Routine data loading is silent

- GIVEN no persistent alert is active
- WHEN an initial list, item detail, navigation read, or polling refresh succeeds
- THEN the alert rail SHALL remain empty
- AND the content and header refresh state SHALL update normally
- WHEN a user-initiated mutation succeeds and triggers a follow-up read
- THEN one operation success alert MAY appear
- AND the follow-up read SHALL NOT add or replace it with a loaded-items success alert

### Requirement: Shared Dialog Host and Dialog Primitives

One modal host SHALL display at most one centered help, choice, confirmation, form, or error dialog over the page frame without covering or relocating the breadcrumb footer or alert rail. All dialogs SHALL use one frame, focus trap, button, cancellation, validation-summary, sizing, in-flight, and action-footer policy. In a form footer, navigation and choice actions SHALL precede cancellation and submission, the `Esc` cancellation action SHALL appear immediately left of the `Enter` submission action, and `Enter` SHALL be the rightmost action with the primary semantic style. `Esc` SHALL cancel when cancellation is safe, focus navigation SHALL be consistent, and an in-flight operation SHALL not be submitted twice.

The shared error dialog SHALL initially be a compact, content-sized summary with danger-semantic title and reason styling plus `Close` and `Details` buttons. `Close` SHALL hold default focus, so an immediate `Enter` or `Esc` restores the source presentation; `Tab`, `Shift+Tab`, Left, and Right SHALL move focus between the complete buttons. When a response body is a JSON object whose non-empty `kind` is `Error` and whose `reason` is non-empty, the dialog SHALL use that kind as its title and that reason as its user-facing message; it MAY show the error `code` and HTTP status as muted secondary context. Because the OpenAPI `Error` properties are currently optional and transport, proxy, or malformed responses may not contain a TRex envelope, the dialog SHALL fall back to a concise status or cause summary rather than assuming those properties exist. Activating `Details` SHALL open the bounded, resize-aware scrollable full-error presentation; `Esc` from full details SHALL return to the compact summary, while `Esc` from the summary and the focused `Close` button SHALL restore the source presentation.

For an API failure the full details SHALL retain the operation identity, HTTP method and safe URL when available, status code and reason when available, transport or decode cause when applicable, and the complete response body read within the configured response-size limit. It SHALL preserve response-body whitespace, neutralize terminal controls and framework markup, redact configured credentials and bearer values, wrap long lines without discarding content, and expose deterministic up/down, page, home/end, and back controls. The concise rail and compact-dialog summaries MAY truncate to their available widths, but the full details SHALL NOT be truncated; all retained content SHALL be reachable by scrolling. Response and request headers SHALL NOT be displayed because they may contain credentials or cookies.

#### Scenario: Inspect an entire API error safely

- GIVEN a foreground API response contains `{"kind":"Error","reason":"Unable to load dinosaurs","code":"rh-trex-ai-1"}` plus full multiline details longer and wider than the available detail viewport, terminal-control content, and a configured credential
- WHEN the compact failure dialog opens
- THEN its title SHALL be `Error`, its primary message SHALL be `Unable to load dinosaurs`, and it SHALL offer `Close` and `Details` buttons with `Close` focused by default
- WHEN the user opens `Details` and scrolls from its beginning to its end
- THEN the full presentation SHALL show request and status context plus every retained safe response-body character in order
- AND line wrapping SHALL preserve the body whitespace and SHALL NOT discard off-screen content
- AND terminal controls, configured credentials, bearer values, request headers, and response headers SHALL not appear
- AND the final alert rail SHALL remain visible beneath the modal
- WHEN the user presses `Esc`
- THEN the compact summary SHALL be restored before a subsequent dismissal returns to the unchanged source presentation

Every operation confirmation SHALL use one shared compact confirmation component. It SHALL render a centered inset title, a centered sanitized question, and one centered button row with `Cancel` on the left and the affirmative action on the right. The affirmative button SHALL read `Delete` for destructive confirmations and `Confirm` for other confirmations. The focused button SHALL use the selected semantic style; destructive confirmation SHALL initially focus the safe cancel action. `Tab`, `Shift+Tab`, `Left`, and `Right` SHALL move focus between complete buttons, `Enter` SHALL activate only the focused button, and `Esc` SHALL cancel. The compact component SHALL be content-sized with a stable minimum width and bounded terminal margins; it SHALL NOT expand to page width because of a redundant key-hint footer, render a separate `DESTRUCTIVE` banner, or expose generic implementation wording such as `Run <operation>`.

#### Scenario: Confirm a destructive operation safely

- GIVEN a delete operation is available for the selected item
- WHEN its confirmation dialog opens
- THEN the underlying page and selection SHALL remain visible and unchanged
- AND cancel SHALL initially hold focus
- AND the dialog SHALL show one compact centered question above `Cancel` and `Delete` buttons
- AND the focused Cancel button SHALL use the selected semantic style
- AND no destructive banner or key-hint footer SHALL be rendered inside the dialog
- AND the fixed alert rail SHALL remain visible below the overlay
- AND repeated submit keys SHALL cause at most one HTTP request

### Requirement: Schema-Driven Form Dialog

Create, update, and non-CRUD request input SHALL use one schema-driven form dialog generated from operation parameter and request-body descriptors. Fields SHALL be grouped with every required field before every optional field while preserving deterministic descriptor order within each group. Each field heading SHALL show only its sanitized field name, type including format when present, and the word `required` or `optional`; it SHALL NOT expose implementation labels such as `body field`, `query parameter`, or descriptor locations. Within a form, the shared component SHALL measure display-cell widths and pad the field-name, type, and requiredness columns so every value input begins in the same display column. The field name SHALL use the theme's bright normal foreground with emphasis, while type and requiredness SHALL use muted metadata styling. Forms SHALL use type- and format-appropriate inputs, exclude read-only properties, permit write-only input, constrain enum choices, preserve explicit zero values, and validate before submission. Invalid fields SHALL render a danger-colored `!` and sanitized message beneath the corresponding aligned input through the shared field-error component and summarize through the fixed alert rail. Submission SHALL be disabled while invalid or in flight. A submitted form SHALL remain retained and visibly in flight until the API operation succeeds; only success SHALL close it and clear its values. On failure, the shared error dialog SHALL appear over the retained action workflow, and dismissing that error SHALL re-enable the form with its exact field values and focus preserved so the user can revise and resubmit. When a confirmation step followed the form, failure dismissal SHALL return to the form rather than the completed confirmation, and a revised submission SHALL require confirmation again. A supported JSON request body that cannot be represented structurally MAY use a generic raw-JSON field named `body`; the runtime SHALL NOT add an operation-specific hand-written form.

#### Scenario: Reuse one form for different operations

- GIVEN create and action operations have different required parameters, enum fields, and writable body properties
- WHEN each operation opens its input dialog
- THEN the same form component SHALL render all required fields before all optional fields and retain deterministic order inside those groups
- AND each field heading SHALL contain only its visually emphasized name, muted type, and muted required or optional state
- AND every input SHALL begin at the same display-cell offset despite different field-name and type widths
- AND an invalid field SHALL show its `!` marker and safe diagnostic in the danger style beneath that input
- AND the footer SHALL end with `Esc` followed by a primary-styled `Enter`
- AND read-only fields SHALL be absent
- AND invalid or duplicate submission SHALL make no request

#### Scenario: Revise an action after an API failure

- GIVEN the user enters action values and submits a valid form, with or without a following confirmation
- WHEN the API request fails
- THEN the error dialog SHALL open while the form and its exact values remain retained
- AND dismissing the error SHALL restore the enabled form at its previous field focus
- WHEN the user revises a value and submits again
- THEN any documented confirmation SHALL be required again
- AND the form SHALL close and clear its retained values only after the API operation succeeds

### Requirement: Refresh and Stale-Data Lifecycle

The integrated `tui` subcommand SHALL accept a refresh interval whose default is five seconds and whose value `0` disables polling. Only the active readable page SHALL poll; it SHALL skip a tick while any request for that page is in flight, pause when hidden, and ignore or cancel results that no longer belong to the active navigation frame. Streaming operations SHALL use their stream lifecycle and SHALL NOT be duplicated by a polling loop. An action that changes the active resource SHALL trigger an immediate refresh after success.

The header SHALL show refresh activity and the last successful refresh age. A page SHALL become stale when the elapsed time since its last success exceeds the greater of three configured intervals or fifteen seconds. Refresh failure SHALL preserve existing content, mark it stale, and create a persistent rail error. A subsequent successful refresh SHALL clear the related error without flashing the initial loading page and SHALL restore table selection by validated identity when that row remains present.

#### Scenario: Slow request skips refresh ticks

- GIVEN the default five-second interval and a list request that remains in flight for twelve seconds
- WHEN two refresh ticks occur before it completes
- THEN no overlapping list request SHALL start
- AND the active page SHALL eventually be marked stale with any failure summarized in the fixed alert rail
- AND a later success SHALL clear that refresh error and preserve selection

#### Scenario: Disable polling

- GIVEN the user configures a refresh interval of `0`
- WHEN the active page remains open
- THEN no timer-driven request SHALL be sent
- AND manual navigation and post-action refreshes SHALL continue to work

### Requirement: Presentation Component Conformance Gate

The generated runtime SHALL have deterministic component tests for spacious, constrained, and extremely narrow continuous layouts and for list, detail, stream, loading, empty, forbidden, stale, and fatal pages. Tests SHALL cover semantic header key/value rows, contextual header shortcut ordering, equal shortcut-column widths, aligned Action offsets after variable-width key tokens, the six-row bound, responsive elision and restoration, help and dispatch parity, and absence of a duplicate bottom shortcut strip. Tests SHALL also cover centered and semantically segmented resource titles, live and persisted `</filter>` title badges with filtered counts, complete-badge breadcrumb rendering and responsive ancestor elision, content-aware Unicode column measurement, minimum and maximum bounds, priority-based compression, horizontal offsets, overflow indicators and counts, a complete three-row mode-colored prompt border, dinosaur icon and singular-prefix rendering, deterministic inline resource completion and cycling, every completion acceptance key, unavailable-view exclusion, filter history, right-aligned dim detail keys, two-cell detail gaps, aligned wrapped values, bright-white detail values, resize reflow, token-classified raw JSON whose styles strip losslessly, every alert severity and lifetime, fixed alert coordinates across page, command, dialog, and resize transitions, shared help, choice, confirmation, form, compact error-summary, and scrollable error-detail dialogs, TRex-envelope parsing with generic fallback, Close/Details focus and transitions, full safe API-error retention, error-detail resize and scrolling, background-error non-interruption, failed-action form/value/focus retention, confirmation re-entry on retry, success-only form closure, required-first field ordering, display-cell-aligned form columns and inputs, danger-styled inline field errors, cancel/submit footer ordering, primary submit styling, focus order, and selection preservation. A static architecture test SHALL fail if a page defines outer chrome, raw color styles, global key strings, dialog positioning, command/filter prompt layout, form-column layout, dialog-action layout, shortcut-palette layout, breadcrumb layout, column-sizing policy, or alert policy outside its designated shared component. Render assertions SHALL use a fixed terminal size and color profile so they are independent of the host terminal.

#### Scenario: Prove one presentation system

- GIVEN the shared runtime and its generated descriptor fixtures
- WHEN component, `teatest`, and architecture conformance tests run
- THEN snapshots SHALL prove consistent chrome and responsive behavior for every page and dialog state
- AND transition tests SHALL prove that every error occupies the same bottom-row coordinates
- AND the architecture test SHALL reject an intentionally duplicated page-owned presentation policy

### Requirement: Resource View Graph Projection

The TUI SHALL preserve resource views and navigation relationships as a directed graph rather than choosing one canonical tree. An explicit OpenAPI Link relationship SHALL take precedence over path-derived inference. The projection MAY expose a conservatively inferred containment edge only when the canonical IR marks one parent item and one child collection as unambiguous. It SHALL NOT connect an ambiguous relationship merely because route segments or schema names appear similar.

#### Scenario: Explicit link overrides path appearance

- GIVEN an explicit Link targets a child collection and maps its scope parameter
- AND route structure could imply a different parent
- WHEN TUI relationships are projected
- THEN the explicit relationship SHALL be the navigable edge
- AND no conflicting inferred parent edge SHALL be emitted

#### Scenario: Ambiguous path remains unconnected

- GIVEN two candidate parent views could supply a child collection's path parameter
- AND no explicit Link resolves the ambiguity
- WHEN TUI relationships are projected
- THEN neither candidate SHALL gain an inferred child edge
- AND generation SHALL report a non-fatal diagnostic identifying why explicit relationship metadata is required

### Requirement: Multi-Parent Views and Navigation Stack

A resource view MAY be reachable globally and through multiple parent relationships. Each navigation SHALL push an immutable frame containing the chosen edge, source view, selected item identity, bound scope values, and target view. `Esc` SHALL pop exactly one frame, and breadcrumbs SHALL be rendered from the actual stack rather than from a precomputed resource hierarchy. Reaching the same target through two parents SHALL preserve two distinct scope and back-navigation histories.

#### Scenario: Same child reached from different parents

- GIVEN a Messages view is reachable globally, from an Agent, and from a Session
- WHEN a user enters Messages from a selected Session and then presses `Esc`
- THEN the breadcrumb SHALL show the Session route used to enter the view
- AND `Esc` SHALL return to that Session context rather than a canonical Messages parent

### Requirement: Deterministic Path-Parameter Binding

Every navigable item or relationship descriptor SHALL contain a complete, generation-time binding plan for every target path parameter. The plan SHALL use the following precedence: an explicit Link parameter mapping; an already-bound navigation scope parameter with the same OpenAPI name and location; then the selected row's identity property only for the single item parameter introduced by an unambiguous collection-to-item or parent-to-child path edge. A selected-row property SHALL NOT be matched to a parameter through case folding, suffix stripping, singularization, or another naming heuristic.

An explicit Link mapping SHALL support standard literal values and OpenAPI runtime expressions available from the source request or response. A response-body expression used for row navigation SHALL resolve against the selected source representation defined by the source relationship; if that representation cannot supply the referenced value, the edge SHALL be non-navigable. All bound values SHALL be validated against the target parameter schema and serialized according to the target parameter's OpenAPI style and explode rules.

#### Scenario: Bind item and scoped child from a selected row

- GIVEN a collection row has `id: "agent-7"` as its validated identity property
- AND the unambiguous item path introduces `{agent_id}`
- AND a child collection extends that item path without introducing another unbound parent parameter
- WHEN the user selects the row and enters the item or child view
- THEN the binding plan SHALL bind `agent_id` to `agent-7`
- AND the exact target path SHALL contain the encoded value in the declared segment

#### Scenario: Explicit mapping is authoritative

- GIVEN a Link maps target `project_id` from `$request.path.project_id` and `agent_id` from `$response.body#/id`
- WHEN the relationship descriptor is generated and evaluated for a selected Agent
- THEN those two expressions SHALL be the only sources for those target parameters
- AND an absent or invalid mapped value SHALL produce an inline navigation error rather than falling back to a naming heuristic

#### Scenario: Multiple unbound target parameters

- GIVEN a target route requires two path parameters not supplied by explicit mappings or current scope
- AND selected-row identity could account for at most one of them
- WHEN the TUI projection validates the edge
- THEN it SHALL reject that edge as non-navigable
- AND the diagnostic SHALL identify the target operation and every unsatisfied parameter

### Requirement: Typed Resource Presentation Extension

The collection-operation form of `x-trex-tui` SHALL be an optional typed presentation block with only the following fields. It SHALL NOT define operations, relationships, authorization, or request semantics.

| Field | Type | Semantics |
|-------|------|-----------|
| `label` | non-empty string | Human-readable resource-view label |
| `aliases` | array of unique strings | Alternate resource-switcher commands |
| `identity-property` | string | Readable scalar property used to identify a selected row |
| `default-sort` | string | Readable scalar property used for the initial ascending sort |
| `columns` | ordered array | Explicit table columns in display order |
| `columns[].property` | string | Readable scalar item property |
| `columns[].label` | non-empty string | Column heading |
| `columns[].priority` | integer | Relative resistance to width compression; higher values shrink after lower values without changing declared order or accessibility |

```yaml
x-trex-tui:
  label: Agents
  aliases: [ag]
  identity-property: id
  default-sort: name
  columns:
    - property: name
      label: NAME
      priority: 100
    - property: status
      label: STATUS
      priority: 80
```

Aliases SHALL match `[a-z][a-z0-9-]*`. The generator SHALL reject a recognized field with the wrong type, duplicate aliases, unknown fields within this grammar revision, an empty explicit column list, duplicate column properties, references to missing or non-readable properties, a default sort absent from explicitly declared columns, conflicting aliases among simultaneously addressable views, and terminal control characters in presentation strings. The source file and JSON Pointer of invalid metadata SHALL appear in the diagnostic.

#### Scenario: Apply resource presentation metadata

- GIVEN a collection operation declares a label, alias, identity property, default sort property, and ordered columns
- WHEN the TUI descriptor is generated
- THEN it SHALL preserve the declared column order and labels
- AND it SHALL use priority only to order compression as available width shrinks
- AND every declared column SHALL remain reachable through horizontal scrolling
- AND the extension SHALL NOT change the operation's route, relationship, capability, or security state

#### Scenario: Reject a misspelled property

- GIVEN `default-sort` names a property absent from the list item schema
- WHEN the TUI projection validates `x-trex-tui`
- THEN generation SHALL fail before writing output
- AND the diagnostic SHALL identify the extension location and missing property

### Requirement: Deterministic Presentation Defaults

A collection view without `x-trex-tui` SHALL remain generatable. The projection SHALL derive a deterministic label from the canonical resource-view identity, SHALL assign no aliases, SHALL use a readable scalar `id` property as identity when present, and SHALL otherwise leave identity unset. It SHALL derive columns from readable scalar item properties in canonical deterministic order and SHALL choose the first displayed property as the default sort when no explicit sort is declared. A relationship that requires selected-row identity SHALL be non-navigable when no validated identity property exists.

#### Scenario: Metadata-free TRex resource

- GIVEN a collection view has readable scalar `id`, `kind`, and `name` properties but no TUI extension
- WHEN generation runs twice
- THEN both runs SHALL derive the same label, identity, columns, and default sort
- AND generated output SHALL be byte-for-byte identical

### Requirement: Typed Operation Presentation Metadata

The non-collection-operation form of `x-trex-tui` MAY contain only the following presentation fields. It SHALL NOT add, remove, hide, authorize, or change the request semantics of an OpenAPI operation. `visibility` remains reserved and unsupported because capability and authorization state are authoritative.

| Field | Type | Semantics |
|-------|------|-----------|
| `label` | non-empty string | Human-readable action label |
| `hotkey` | string matching `[a-z0-9]` or `ctrl-[a-z]` | Preferred local action binding |
| `confirmation` | object | Requests a confirmation dialog before a non-delete action and customizes confirmation presentation |
| `confirmation.title` | non-empty string | Static confirmation-dialog title |
| `confirmation.message` | non-empty string | Static confirmation explanation |
| `confirmation.destructive` | boolean | Applies destructive styling and safe initial focus |

Presentation strings SHALL be sanitized, SHALL contain no terminal controls, and SHALL be treated as static data without template evaluation. A selected item identity MAY be rendered separately by the runtime after sanitization but SHALL NOT be interpolated as executable template syntax. A hotkey that conflicts with a global key or another simultaneously visible local action SHALL fail generation with both source locations.

Every DELETE operation SHALL require the shared destructive confirmation dialog even when no extension is present. DELETE metadata MAY customize safe title and message text but SHALL NOT disable confirmation or the initially focused cancel action. A non-delete operation SHALL require confirmation when it declares a `confirmation` object.

#### Scenario: Delete always confirms

- GIVEN a DELETE operation has no `x-trex-tui` metadata
- WHEN the user invokes its generated action
- THEN the shared destructive confirmation dialog SHALL open
- AND cancel SHALL initially hold focus
- AND no request SHALL be sent before explicit confirmation

#### Scenario: Confirm a non-delete action

- GIVEN a POST action declares a static label, valid local hotkey, and non-destructive confirmation metadata
- WHEN the user invokes it
- THEN the keybinding registry and shared confirmation dialog SHALL use that presentation metadata
- AND the extension SHALL NOT change the operation's route, inputs, security, or capability

#### Scenario: Reject a conflicting hotkey

- GIVEN two actions visible on the same page declare the same hotkey
- WHEN the projection validates their metadata
- THEN generation SHALL fail with both operation identities and source pointers
- AND neither binding SHALL silently win

#### Scenario: Reject visibility metadata

- GIVEN a non-collection operation declares `x-trex-tui.visibility`
- WHEN the projection validates the operation
- THEN generation SHALL report that the field is unsupported
- AND the operation's descriptor capability and authorization state SHALL remain unchanged

### Requirement: Resource Switching, Tables, Filtering, and Detail

The runtime SHALL open on a top-level resource catalog containing exactly the descriptor collection views whose `ScopeParameters` are empty. The catalog SHALL present one visible resource-name column in deterministic label order, and that column SHALL fill the available table width so the header and full-row selection styling reach the table's right edge. It SHALL NOT place scope, readiness, route, operation, or other API-debugging metadata in the primary list. Such metadata MAY be exposed through a secondary help or detail presentation. Scoped collection views SHALL remain absent from the home catalog and SHALL remain discoverable through descriptor relationship navigation from their parents. `Enter` on a catalog row SHALL push that resource onto the navigation stack and perform its documented read; `Esc` from a top-level resource SHALL return to the catalog without issuing a catalog request. A descriptor with no unscoped collection view SHALL still render an empty Resources page without issuing a request.

The runtime SHALL also provide a resource switcher for globally addressable views and for scoped views whose required bindings are available in the current stack. It SHALL accept each validated alias and expose the same available labels, view identifiers, and aliases to the shared deterministic inline completion model. A successful switch SHALL retain the catalog as the navigation root. The runtime SHALL render list responses as selectable tables, apply `/` filtering across sanitized visible column values, and provide a scrollable item-detail view containing all readable response fields. `Enter` SHALL follow an available descriptor relationship from the selected row; when more than one edge is available it SHALL present a deterministic relationship chooser rather than choose a parent or child implicitly. A detail command SHALL remain available independently of child navigation.

#### Scenario: Browse the top-level resource catalog

- GIVEN descriptors define a global Dinosaurs collection and a scoped Fossils collection requiring `dinosaur_id`
- WHEN the TUI starts
- THEN the initial catalog SHALL use the simple title `Resources` and contain Dinosaurs in one resource-name column without making an API request
- AND the resource-name column and its selected-row styling SHALL fill the available table width
- AND Fossils SHALL be absent from the initial catalog
- AND no scope or readiness column SHALL appear
- WHEN the user selects Dinosaurs
- THEN the runtime SHALL push Dinosaurs, execute its documented list operation, and retain Resources as the breadcrumb root
- WHEN the user selects a dinosaur with a documented relationship to Fossils
- THEN the runtime SHALL expose Fossils through parent navigation using the bound `dinosaur_id`

#### Scenario: Browse without resource-specific code

- GIVEN generated descriptors define Dinosaurs and Fossils with different columns
- WHEN the user switches resources, filters rows, selects one, and opens detail
- THEN the generic runtime SHALL render the descriptor-defined table and detail fields
- AND the resource switcher SHALL complete only views available under the current bindings
- AND no behavior SHALL depend on the literal names Dinosaur or Fossil

#### Scenario: Select among multiple child edges

- GIVEN a selected row has two explicit outgoing relationships
- WHEN the user presses `Enter`
- THEN the runtime SHALL display both targets in stable descriptor order
- AND choosing one SHALL push only that relationship onto the navigation stack

### Requirement: Capability-Driven Operations

The runtime SHALL derive available list, get, CRUD, non-CRUD action, and streaming controls exclusively from descriptor capabilities backed by canonical IR operations. It SHALL NOT synthesize CRUD controls or invoke an undocumented method. On a collection page, the action chooser and applicable generated hotkeys SHALL include both operations on the collection view and operations on the item view for the currently highlighted row. Selected-item operations SHALL be discovered only through a navigable unambiguous collection-to-item edge, SHALL use that edge's complete binding plan to pre-bind the highlighted row, and SHALL be absent when no row is selected or the binding plan cannot be evaluated. The action set and bindings SHALL follow selection changes without requiring item-detail navigation.

Generic operation labels SHALL derive deterministically from OpenAPI summary and `operationId` when typed operation presentation metadata does not supply a label. Request input controls SHALL honor requiredness, schema types, read-only and write-only semantics, operation parameters, and values already supplied by the active stack or selected-row binding plan. Projection SHALL treat collection operations and selected-item operations exposed together as simultaneously visible when validating generated hotkey conflicts.

#### Scenario: Read-only view with one action

- GIVEN a view has list, get, and interrupt capabilities but no create, update, or delete operation
- WHEN the runtime renders available controls
- THEN list, get, and interrupt SHALL be available
- AND create, update, and delete SHALL be absent

#### Scenario: Offer actions for the highlighted item

- GIVEN a Dinosaurs collection exposes create and list operations
- AND its item view exposes get, update, and delete operations through a navigable collection-to-item edge binding `{id}` from the selected row
- WHEN a dinosaur row is highlighted and the user opens the action chooser
- THEN create, update, and delete SHALL be offered in deterministic order
- AND list and get SHALL remain read controls rather than actions
- AND update and delete SHALL receive the highlighted dinosaur's bound `id` without asking the user to re-enter it
- WHEN the collection has no selected row
- THEN only the collection-level create operation SHALL be offered

### Requirement: Exact HTTP Request Construction

The generated client SHALL construct requests from descriptor operations, using the exact HTTP method, server selection, ordered path segments, bound path values, query and header parameters, serialization rules, content type, and request-body schema retained by the IR. Dynamic values SHALL be encoded for their path, query, header, or body context and SHALL never be concatenated into an unparsed route template. The TUI SHALL communicate only through documented API operations.

#### Scenario: Multiply scoped request

- GIVEN the active stack binds organization, project, and agent identifiers
- AND the target operation is `GET /organizations/{organization_id}/projects/{project_id}/agents/{agent_id}/inbox`
- WHEN the runtime executes the operation
- THEN the test server SHALL receive that exact method and path with each value encoded in its declared segment
- AND no scope SHALL be dropped, reordered, or replaced by a selected row from another frame

#### Scenario: Selected-item action request

- GIVEN the highlighted collection row has identity `dinosaur/7`
- AND its collection-to-item edge binds that identity to the item operation's `{id}` path parameter
- WHEN the user chooses the documented update or delete action from the collection page
- THEN the request SHALL use the documented method and the encoded item path containing `dinosaur%2F7`
- AND the form SHALL omit the already-bound `id` from requested user input

### Requirement: Operation Security and Credential Safety

The client SHALL preserve the distinction among inherited document security, explicit `security: []`, and non-empty operation overrides. It SHALL apply runtime-supplied credentials only through a supported declared security alternative, SHALL support the TRex HTTP bearer scheme, and SHALL fail generation with an actionable diagnostic when a required operation has no supported security alternative. The absence of a runtime-supplied credential SHALL NOT be treated as a local request-validation failure: the client SHALL send the request without an `Authorization` header and defer authentication enforcement to the configured server. This permits the documented local `run-no-auth` workflow while allowing an authentication-enabled server to return its normal `401` response through the shared API-error presentation. Credentials SHALL be bound to the user-configured API origin; the client SHALL refuse to attach them to a different operation-level server origin unless the user explicitly trusts that origin. Non-loopback plaintext HTTP SHALL require an explicit insecure runtime option. The client SHALL NOT embed credentials in generated source or descriptors, send credentials to an explicitly unauthenticated operation, or include credential values in rendered errors, logs, panic output, or test snapshots.

#### Scenario: Public and authenticated operations

- GIVEN document security requires HTTP bearer authentication
- AND one operation explicitly declares `security: []`
- WHEN both operations are invoked with a configured token
- THEN the inherited operation SHALL receive the bearer credential
- AND the explicitly unauthenticated operation SHALL receive no credential

#### Scenario: Use the documented no-auth local server

- GIVEN an operation inherits required Bearer security from the OpenAPI document
- AND the TUI has no runtime-supplied token
- WHEN the configured server is running with authentication disabled and the operation is invoked
- THEN the client SHALL send the request without an `Authorization` header
- AND a successful server response SHALL be handled normally rather than replaced by a local missing-token error
- AND if the server instead requires authentication, its `401` response SHALL use the shared safe API-error presentation

#### Scenario: Unsupported required scheme

- GIVEN a required operation declares only an unsupported authentication scheme
- WHEN the TUI projection validates the operation
- THEN generation SHALL fail with the operation ID and scheme name
- AND it SHALL NOT silently generate an unauthenticated request

#### Scenario: Cross-origin server override

- GIVEN an authenticated operation overrides the configured API server with a different origin
- AND the user has not explicitly trusted that origin
- WHEN the runtime prepares the request
- THEN it SHALL refuse to attach or transmit the credential
- AND the inline diagnostic SHALL identify the untrusted origin without revealing credential data

### Requirement: Terminal-Safe Rendering

All OpenAPI-derived and API-returned strings SHALL be treated as untrusted at the final rendering boundary. Every table cell, detail value, breadcrumb, label, error, action result, stream event, and raw-mode field SHALL strip or neutralize ANSI CSI, OSC, DCS, C0, and C1 control sequences and any framework-specific markup before terminal output. Newline and tab handling SHALL be explicit for the destination view, and sanitized content SHALL NOT be able to move the cursor, set terminal titles, write clipboard data, create links, or inject key events. Sanitization SHALL be idempotent and SHALL occur in addition to the source-code and output-path protections required by CG-005.

#### Scenario: Escape injection in every view

- GIVEN an API value contains color escapes, an OSC terminal-title or clipboard command, control bytes, and framework markup
- WHEN the value appears in a table, detail view, breadcrumb, error, or stream/raw view
- THEN none of those control effects SHALL reach the terminal
- AND safe printable text SHALL remain visible

### Requirement: Actionable Projection Diagnostics

The generator SHALL validate the complete TUI projection before writing target files. A fatal diagnostic SHALL include the source file, JSON Pointer, affected operation or view identity, and a safe explanation for malformed metadata, incomplete bindings, unsupported required authentication, or descriptor conflicts. Ambiguous relationships that are valid in the canonical IR but intentionally unavailable for TUI navigation SHALL be reported as non-fatal diagnostics and SHALL remain absent from the generated navigation graph.

#### Scenario: Invalid projection writes nothing

- GIVEN a TUI extension selects an invalid identity property and a relationship lacks a required binding
- WHEN generation runs against an empty output directory
- THEN generation SHALL fail with both actionable diagnostics when safe to aggregate
- AND it SHALL leave no partially generated descriptor package

### Requirement: Repository Generation Workflow

The repository SHALL provide `make generate-tui`, SHALL invoke it from the normal `make generate` workflow, SHALL include it in `make generate-all`, and SHALL include the TUI projection and integrated-artifact acceptance suites in continuous testing. `make generate-tui` SHALL replace only the generated descriptor package beneath `data/generated/tui`; it SHALL NOT emit a standalone module or executable. Generation SHALL use an isolated temporary staging directory and replace the configured output only after successful validation and rendering. Output paths SHALL remain beneath the configured output root.

#### Scenario: Generate all clients

- GIVEN the repository OpenAPI document is valid for every generator
- WHEN `make generate-all` completes
- THEN SDK, CLI, console, and TUI artifacts SHALL have been generated
- AND a TUI failure SHALL prevent a partial staged TUI tree from replacing the previous output

#### Scenario: Normal API generation cannot leave stale TUI descriptors

- GIVEN the repository OpenAPI document changes a resource, operation, relationship, security requirement, or TUI presentation extension
- WHEN the normal `make generate` workflow succeeds
- THEN the OpenAPI server artifacts and integrated TUI descriptor package SHALL both reflect that same document
- AND a subsequent primary service build SHALL compile the updated descriptor into its `tui` subcommand

#### Scenario: Refuse an unowned output directory

- GIVEN the configured TUI output is a non-empty directory without the generator's exact ownership marker
- WHEN TUI generation would replace that directory
- THEN generation SHALL fail before renaming or deleting the existing directory
- AND every existing file SHALL remain unchanged
- AND a symbolic-link output SHALL be rejected rather than followed or replaced

### Requirement: Graph Conformance Gate

The TUI generator SHALL have fixture tests for flat resources, multiply scoped views, one view reachable through multiple parents, explicit Link precedence, conservative inference, and ambiguous relationships. Tests SHALL assert descriptor edges, provenance, scope, and addressability rather than only successful generation.

#### Scenario: Multi-parent and ambiguous fixture

- GIVEN a fixture contains a global view, two explicit parent paths to one child, and one ambiguous path-only candidate
- WHEN TUI descriptors are tested
- THEN both explicit parent edges and the global view SHALL be present
- AND the ambiguous candidate SHALL be absent with its expected diagnostic

### Requirement: Parameter-Binding and Request Gate

The TUI generator SHALL have `httptest` acceptance cases for item, child, action, and multiply scoped operations. The cases SHALL exercise explicit Link expressions, inherited stack scope, selected-row identity, missing values, validation, serialization, and exact method, route, query, header, body, and authentication behavior.

#### Scenario: Exact bound request test

- GIVEN a fixture provides values from a Link, two existing stack frames, and the selected row
- WHEN the generated runtime sends the target request to `httptest.Server`
- THEN the server SHALL observe the exact expected request and authentication state
- AND a test variant with an unsatisfied value SHALL make no request

### Requirement: Capability Conformance Gate

Fixture tests SHALL prove that controls and request inputs are projected only from documented capabilities, including a read-only view, partial CRUD, a non-CRUD action, a streaming operation, and selected-item operations exposed from a collection row. The tests SHALL fail when the runtime invents an absent CRUD operation, omits a documented supported capability, or requests a path value already supplied by a selected-row binding plan.

#### Scenario: Partial capability fixture

- GIVEN a fixture documents list, patch, and stream operations only
- WHEN descriptors and rendered controls are asserted
- THEN exactly those supported capabilities SHALL be available
- AND create, get, and delete SHALL be absent

### Requirement: Runtime Navigation Gate

Generated-runtime acceptance tests SHALL use `httptest` with `teatest` to send user keystrokes and assert initial one-column unscoped catalog rendering without an API request, scoped-view exclusion from the catalog, scoped discovery through parent navigation, catalog entry and return, compact safe destructive confirmation, protected left-prefix sort markers under truncation, resource switching, aliases, filtering, selected-item raw JSON inspection without another request, selected-item action discovery and binding, selected-row navigation, relationship choice, details, `Enter` push, `Esc` pop, breadcrumbs, multi-parent history, full scrollable foreground API-error dialogs with state-preserving dismissal, and failed-action form correction followed by a successful retry. The test SHALL exercise generated descriptors rather than a resource-specific fake runtime.

#### Scenario: Enter and Escape preserve scope

- GIVEN a test server serves two parent paths to the same child view
- WHEN `teatest` enters the child through one parent and then sends `Esc`
- THEN rendered breadcrumbs SHALL identify the chosen parent
- AND the restored table and selection SHALL belong to that same parent frame

### Requirement: Terminal Injection Gate

Automated tests SHALL inject ANSI, OSC, DCS, C0, C1, malformed escape sequences, newlines, tabs, Unicode, and framework-markup payloads through presentation metadata and API responses. Assertions SHALL cover tables, details, breadcrumbs, errors, and stream/raw output and SHALL verify both safe text preservation and absence of terminal effects.

#### Scenario: Malicious API fixture

- GIVEN every user-visible response field includes terminal-control payloads
- WHEN `teatest` renders every supported view type
- THEN captured output SHALL contain no prohibited control sequence
- AND repeated sanitization SHALL produce the same safe value

### Requirement: Deterministic Generation Gate

An unchanged resolved OpenAPI input SHALL produce byte-for-byte identical TUI output across repeated runs, regardless of YAML map iteration or referenced-file traversal order. The acceptance suite SHALL generate twice into separate temporary directories and compare sorted file paths, modes, and SHA-256 digests. Generated files SHALL carry stable generated-code notices and SHALL contain no timestamps or host-specific absolute paths.

#### Scenario: Compare generated trees

- GIVEN one resolved OpenAPI input and fixed generator dependencies
- WHEN the TUI is generated twice in isolated directories
- THEN both trees SHALL have identical relative paths, file modes, and SHA-256 digests
- AND each tree SHALL contain only the owned generated descriptor package files

### Requirement: Repository OpenAPI Acceptance Gate

Continuous integration SHALL run the TUI generator against the fully resolved repository `openapi/openapi.yaml`, generate an isolated descriptor package, compile that package together with the shared runtime and primary Cobra command, and leave the working tree unchanged. The gate SHALL prove that the root help exposes `tui`, that no standalone TUI entry point or module is emitted, and that runtime component and interaction tests still exercise the generated descriptor contract. This gate SHALL run with the shared IR conformance fixtures through the repository test workflow and SHALL require no database or external API service.

#### Scenario: Generate the real TRex TUI

- GIVEN the repository root OpenAPI document and all referenced entity documents
- WHEN generator CI runs
- THEN the integrated descriptor package and primary service `tui` subcommand SHALL be generated, built, and tested from that real document
- AND the same job SHALL verify the SDK, CLI, console, and TUI consumers against the current canonical IR

## Design Decisions

| Decision | Rationale |
|----------|-----------|
| Graph navigation, not a kind tree | A schema can be global, multiply scoped, or reachable through several semantic relationships |
| OpenAPI Links before conservative inference | Standard explicit mappings resolve meaning that path shape alone cannot establish |
| Stack frames retain edge and bindings | Breadcrumbs and `Esc` must return through the route actually used, especially for multi-parent views |
| Binding plans are generated, not guessed at runtime | Exact sources, validation, and serialization can be reviewed and fixture-tested before requests are sent |
| `x-trex-tui` is presentation-only | Standard OpenAPI remains authoritative for operations, relationships, security, and request semantics |
| Collection-operation metadata configures a view | One schema may have different global and scoped presentations without becoming different resource kinds |
| Typed operation presentation is capability-bound | Labels, hotkeys, and confirmation presentation can improve a documented action but cannot create, hide, authorize, or rewrite it; visibility remains unsupported |
| Generic Bubble Tea runtime | A descriptor-driven Elm-style runtime supports consistent tables, input modes, navigation, and `teatest` coverage |
| Service-neutral shell | OpenAPI identity and runtime connection state make the generated component reusable without carrying TRex-specific branding |
| Semantic components own presentation policy | One implementation of each page, dialog, alert, state, and chrome primitive prevents resource or operation views from drifting apart |
| Fixed bottom alert rail | Errors remain spatially predictable across pages, modes, dialogs, and responsive layouts while inline field and fatal context remain available |
| One keybinding registry | Dispatch, contextual hints, generated hotkeys, conflict validation, and help cannot silently disagree |
| Contextual shortcuts in a two-region top header | A terminal-right, equal-column k9s-style palette with one shared key-token subcolumn shares rows with a vertically anchored key/value service context, keeping actions aligned and discoverable without consuming a second vertical block or competing with the bottom rails |
| Centered semantic resource identity | A centered `kind(context)[count]` frame title mirrors established terminal navigation tools while distinct theme tokens keep resource, context, count, and state scannable without page-owned styling |
| Badge breadcrumb trail | Shared, semantically differentiated `<segment>` badges preserve navigation hierarchy and active-location emphasis while complete-badge elision keeps constrained layouts legible |
| Central theme and responsive layout | Semantic tokens and one measurement authority eliminate ad hoc styling and conflicting terminal arithmetic |
| Content-sized columns with signaled horizontal overflow | Natural display-cell widths preserve information density, bounded compression handles constrained terminals, and directional indicators make every off-screen column discoverable through arrow-key scrolling |
| Modal schema-driven forms | Descriptor inputs can share validation, focus, cancellation, and in-flight behavior without operation-specific form code |
| Five-second skip-on-inflight refresh | Timely defaults avoid overlapping requests; interval `0` permits deliberate opt-out and stale content remains usable on failure |
| API-only data path | The generated TUI works against documented REST operations and does not couple to a database, Kubernetes, or server internals |
| Sanitize at the rendering boundary | One mandatory boundary covers metadata, API data, errors, and future render modes without relying on every caller to remember |
| One integrated service command | The descriptor and shared runtime are compiled into the service users already install, eliminating duplicated entry points, copied runtime code, sidecar lookup, and release skew |
| Synthetic and real-spec gates | Focused fixtures prove hard graph semantics while repository generation proves end-to-end viability |
