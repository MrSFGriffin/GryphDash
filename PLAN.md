
  # Customizable Widget Model and Designer

  ## Summary

  Replace provider-owned fixed widget definitions with provider data-source descriptions, reusable HTML/CSS widget templates, configurable widget instances, and shared dashboard documents. Build the designer
  with GrapesJS Core, CodeMirror, Vite, and JSONata.

  The migration is intentionally breaking:

  - GryphDash and all core providers move to protocol v2 together.
  - Protocol-v1 providers are reported as incompatible and are not adapted.
  - Existing browser and TUI layouts are discarded.
  - Fresh default dashboards look and behave exactly as they do now.
  - Customization is optional; users who never open the designer see the current experience.
  - Custom widgets support HTML, CSS, declarative bindings, and composition, but not widget-authored JavaScript.
  - Web and desktop share dashboards from GryphDash’s configuration directory.
  - The TUI displays generated data fallbacks for custom templates.

  ## Core Interfaces

  ### Provider protocol v2

  Replace -widgets with -describe. A description returns:

  - Provider identity and protocol version.
  - Named data sources.
  - JSON Schema 2020-12 descriptions for each source.
  - Optional starter-template packs.
  - Optional widget-definition factories for repeated or dynamic data.
  - Source URL/method metadata where currently supplied.

  read continues returning namespaced results with data, update time, and errors.

  A source description contains:

  {
    "id": "codex/limits",
    "name": "Codex limits",
    "description": "Account limit windows and reset credits",
    "schema": {}
  }

  Dynamic currency configurations return concrete source descriptions during -describe. Codex describes fixed sources with schema support for dynamically named buckets.

  ### Widget template

  A versioned template contains:

  - Stable template ID, name, description, origin, and revision.
  - Declared input names.
  - GrapesJS project JSON as editable authoring state.
  - Generated HTML and CSS as the runtime representation.
  - JSONata expressions used by binding components.
  - A generated TUI fallback projection.
  - Optional references to other templates.
  - Default width and height.

  Built-in/provider templates are immutable. Choosing Customize creates a user-owned copy and changes that dashboard instance to use the copy.

  ### Widget definition and instance

  A definition is a reusable preset pairing a template with default inputs, properties, title, group, size, and default visibility. Definitions may repeat over a JSONata-selected collection to reproduce
  dynamic Codex bucket widgets.

  A dashboard instance has its own identity:

  {
    "id": "instance-uuid",
    "definitionId": "codex/primary-limit",
    "templateId": "core/limit-window",
    "inputs": {
      "limit": {
        "source": "codex/limits",
        "expression": "buckets.codex.primary"
      }
    },
    "props": {
      "title": "5-hour limit"
    }
  }

  Multiple instances may use the same template with different inputs and properties.

  ### Dashboard document

  Store named dashboards under GryphDash’s configuration directory:

  {
    "version": 1,
    "id": "default",
    "name": "Default",
    "revision": 1,
    "instances": [],
    "grid": [],
    "tuiOrder": []
  }

  - Browser and desktop share widget selection, GridStack positions, sizes, and named dashboards.
  - The TUI uses the same instances but maintains its own ordering.
  - Writes use atomic file replacement and optimistic revision checks.
  - Conflicting clients reload the latest document rather than silently overwriting it.

  ### HTML binding vocabulary

  Support ordinary HTML and CSS plus these first-class elements:

  - gd-value
  - gd-progress
  - gd-time
  - gd-countdown
  - gd-status
  - gd-repeat
  - gd-widget

  Bindings evaluate JSONata against:

  {
    "inputs": {},
    "props": {},
    "item": null
  }

  gd-repeat supplies item for its descendants. gd-widget instantiates another template and maps parent expressions into the child’s declared inputs and properties. Reject reference cycles and stop expansion
  beyond 16 nested templates.

  ## Step-by-Step Implementation

  ### 1. Freeze current behavior as parity fixtures

  - Capture fixture-driven descriptions of every current Codex, OpenRouter, and Currency widget.
  - Record current titles, groups, descriptions, kinds, default flags, sizes, value formatting, notes, reset text, progress values, and structured rows.
  - Record the exact fresh default selection and ordering:
      - Primary limit
      - Secondary limit
      - Credits remaining
      - Available resets
      - Lifetime tokens
      - Current streak
      - Daily token activity
      - Account plan

  - Add golden web DOM fixtures and TUI card fixtures without changing production behavior.
  - Use these fixtures as the acceptance baseline for the new renderer.

  ### 2. Introduce the frontend build pipeline

  - Add package.json, a lockfile, and Vite.
  - Move handwritten frontend code into ES modules without changing behavior.
  - Add pinned dependencies for GrapesJS Core, CodeMirror 6, and the JSONata reference implementation.
  - Continue embedding generated assets into the single Go executable.
  - Commit reproducible built assets so normal Go builds do not require Node.
  - Add npm scripts for build, unit tests, and source checks.
  - Update release scripts to regenerate assets and fail on an unclean generated diff.

  ### 3. Define and validate the new document schemas

  - Add Go types and JSON schemas for provider descriptions, data-source descriptions, template packs, widget templates, definitions, instances, and dashboards.
  - Add strict validation for IDs, duplicate definitions, source references, JSONata syntax, template references, document versions, and layout bounds.
  - Pin browser and Go JSONata implementations to compatible 2.0.6 semantics.
  - Add shared expression fixtures covering paths, arithmetic, filtering, arrays, missing values, booleans, and timestamps.
  - Require identical browser and Go results for the supported fixture set.

  ### 4. Implement protocol v2 in GryphDash

  - Change the subprocess client to request -describe and accept only protocol version 2.
  - Replace catalog merging with provider-description merging.
  - Detect duplicate provider, source, template, and definition IDs.
  - Keep provider read isolation and stale-result behavior unchanged.
  - Mark installed protocol-v1 providers as incompatible in provider status and show that an update is required.
  - Allow GryphDash itself to start when an incompatible provider exists; only that provider remains unavailable.

  ### 5. Convert the provider repository to protocol v2

  - Replace the duplicated dashboard widget types with protocol-v2 description types.
  - Make Codex describe codex/account, codex/limits, and codex/usage.
  - Make OpenRouter describe openrouter/key and openrouter/credits.
  - Make Currency describe each configured currency-pair source.
  - Retain current provider result shapes and fixture-driven reads.
  - Remove provider-side WidgetLogic, rendering kinds, and dashboard construction.
  - Update every provider executable to support -describe.
  - Update repository and per-provider manifests to carry protocol-v2 descriptions and optional template packs.
  - Update manifest generation to invoke -describe.
  - Reject protocol-v1 manifests in the new GryphDash release.

  ### 6. Build the shared template and dashboard store

  - Create configuration subdirectories for user templates and dashboards.
  - Embed shipped/provider starter templates separately from mutable user templates.
  - Implement list, load, create, update, duplicate, rename, and delete operations.
  - Prevent mutation or deletion of shipped templates.
  - Use UUIDs for user template and dashboard-instance IDs.
  - Write files atomically with restrictive permissions.
  - Seed a fresh default dashboard when no new-format dashboard exists.
  - Delete the known legacy TUI layout file when initializing the new store.
  - Have the new frontend remove the legacy gryphdash.layout.v1 and gryphdash.layouts.v1 browser keys.
  - Do not import or retain legacy layouts.

  ### 7. Add the data-source and binding runtime

  - Publish merged source descriptions and current provider results through a new data-source service.
  - Compile and cache JSONata expressions by template revision.
  - Evaluate instance input expressions against their selected source.
  - Build the common binding context containing inputs, props, and repeater item.
  - Represent missing expression results explicitly so bound elements can display the same unavailable states as current widgets.
  - Carry source update/error metadata alongside evaluated contexts.
  - Collect all sources reached through nested template references for desktop notification scope.

  ### 8. Implement the HTML/CSS widget runtime

  - Add a <gryphdash-widget> host that renders each instance in Shadow DOM.
  - Inject the template’s generated HTML and CSS into the shadow root.
  - Define the gd-* custom elements and update them whenever provider data refreshes.
  - Render bound values as text, apply formatters, update progress values, and tick countdowns locally each second.
  - Implement repeaters for daily activity and reset-detail rows.
  - Implement gd-widget by resolving the referenced template and passing mapped inputs and properties.
  - Use ResizeObserver to resize the outer GridStack item after internal content changes.
  - Expose GryphDash theme values through CSS custom properties without limiting template CSS.

  ### 9. Rebuild every existing widget as templates and definitions

  - Create shared templates for:
      - Scalar cards
      - Timestamp cards
      - Limit-window cards
      - Daily activity
      - Reset details

  - Recreate all 23 Codex definitions, all 9 OpenRouter definitions, and all 4 default Currency definitions.
  - Add a repeated-definition mechanism for every Codex bucket returned by codex/limits.
  - Preserve the current dynamic 5-hour, weekly, and arbitrary-minute titles.
  - Preserve current percent inversion, progress bars, reset timestamps, countdowns, descriptions, stale states, tables, reset rows, dimensions, and picker grouping.
  - Use composition internally where useful, but do not change the visible defaults.
  - Keep the primary and secondary limits as two separate default widgets.
  - Ensure the fresh dashboard has exactly the current default widgets and current appearance.
  - Place optional provider-specific starter templates in provider template packs; keep rendering and editing entirely within GryphDash.

  ### 10. Cut the dashboard over to instances

  - Replace the display-ready /api/widgets flow with templates, instances, definitions, and data sources.
  - Keep GridStack responsible only for top-level instance position and size.
  - Change the picker to list widget definitions and user templates.
  - Allow multiple instances of the same template.
  - Save layout changes to the shared dashboard service with a short debounce.
  - Reload and report revision conflicts instead of overwriting another client’s changes.
  - Preserve named-dashboard behavior using server-stored dashboard documents.
  - Remove the legacy dashboard-building logic once parity tests pass.

  ### 11. Rebuild the TUI on template fallbacks

  - Load the same dashboard instances used by web and desktop.
  - Evaluate bindings through the Go JSONata implementation.
  - Render each template’s fallback fields in declared order.
  - Generate fallback entries from the template’s GryphDash binding components when saving in the designer.
  - Allow starter templates to provide explicit fallback labels and summary expressions.
  - Reproduce the current TUI output for all rebuilt starter widgets.
  - Show nested custom widgets as flattened, indented fallback sections.
  - Preserve TUI-only ordering in tuiOrder without changing web/desktop positions.

  ### 12. Add the widget-library and dashboard APIs

  Provide:

  - GET /api/data-sources
  - GET /api/widget-library
  - GET|POST /api/widget-templates
  - GET|PUT|DELETE /api/widget-templates/{id}
  - GET|POST /api/dashboards
  - GET|PUT|DELETE /api/dashboards/{id}

  Behavior:

  - Return document revisions and require the expected revision for updates.
  - Reject edits to shipped templates.
  - Reject invalid GrapesJS documents, generated HTML/CSS pairs, JSONata expressions, bindings, and cyclic references.
  - Prevent deletion of a user template while instances or templates reference it; require callers to remove or replace those references first.
  - Return structured validation errors suitable for the designer.
  - Keep provider-management APIs independent of template/dashboard CRUD.

  ### 13. Build the GrapesJS designer shell

  Add a full-screen designer reachable from:

  - Create widget
  - Customize on an existing widget
  - Edit on a user-owned widget
  - The widget library

  The designer includes:

  - Component palette
  - Provider data-source browser
  - Central live canvas
  - GrapesJS layers panel
  - Style manager
  - Component/property inspector
  - Desktop, tablet, and mobile preview widths
  - Undo and redo
  - Save, save as, duplicate, rename, and delete
  - Live-data and representative-preview-data modes

  Register ordinary HTML blocks plus GryphDash binding components. Dragging a source field onto the canvas creates a suitable gd-value; dropping a field onto an existing binding component changes its
  expression.

  ### 14. Add direct HTML and CSS editing

  - Integrate CodeMirror HTML and CSS panels beside the visual designer.
  - Keep GrapesJS project JSON as the complete editable project state.
  - Applying source changes reparses them into GrapesJS and updates the project.
  - Visual changes regenerate the source views.
  - Show parse and binding errors inline.
  - Retain the last valid canvas when new source is invalid.
  - Include source edits in undo/redo.
  - Generate the runtime HTML/CSS and TUI fallback in the same save transaction.

  ### 15. Add reusable-widget composition

  - Register saved widget templates as draggable GrapesJS blocks.
  - Dropping one creates a gd-widget component with an instance alias.
  - Expose the child template’s declared inputs and properties in the inspector.
  - Let each child input bind to a provider source, parent input, repeater item, or JSONata result.
  - Render nested templates directly in the designer and runtime.
  - Double-clicking a nested user template opens its definition; provide Detach copy when the user wants a local variant.
  - Detect direct and indirect template-reference cycles before saving.
  - Do not add a separate widget-output graph in this release; reusable templates receive and forward data contexts through declared inputs.

  ### 16. Complete the coordinated cutover

  - Publish protocol-v2 core provider binaries and manifests together.
  - Release the matching GryphDash build with protocol-v2-only discovery.
  - Update local development scripts to build and launch the new providers.
  - Update Windows/macOS/Linux packaging to include the new frontend assets.
  - Remove obsolete WidgetCatalog, WidgetLogic, limitWindow, daily, resetDetails, and /api/widgets compatibility code.
  - Rewrite contributor documentation around data sources, starter template packs, custom templates, and the designer.
  - Document that old provider binaries require replacement and old layouts intentionally reset.

  ## Test Plan

  - Provider fixture tests verify every described source schema against representative read results.
  - Protocol tests cover valid descriptions, incompatible versions, duplicate IDs, malformed schemas, and invalid template packs.
  - Cross-runtime tests run the same JSONata fixtures in JavaScript and Go.
  - Store tests cover atomic writes, revisions, conflicts, immutable templates, references, deletion rules, and fresh-default seeding.
  - Runtime tests cover every gd-* element, repeaters, nested templates, missing values, stale values, countdowns, resizing, and cycle rejection.
  - Parity tests compare all rebuilt starter-widget metadata and rendered content with the frozen current fixtures.
  - Dashboard tests verify shared browser/desktop arrangements, multiple instances, named dashboards, picker behavior, and notification source collection.
  - TUI tests verify fallback ordering, formatting, nested sections, and current starter-widget parity.
  - Designer unit tests cover palette insertion, source binding, visual/source synchronization, invalid-source recovery, undo/redo, cloning, and persistence.
  - Update tests/browser.mjs for the new flows, but do not download or run browser automation unless separately requested.
  - Run both repositories’ Go formatting, unit tests, vet, race tests, builds, frontend tests/build, JavaScript checks, manifest generation checks, and git diff --check.

  ## Assumptions and Fixed Defaults

  - GrapesJS Core is the production designer foundation.
  - Vite owns the frontend build; built assets remain embedded in the Go executable.
  - JSONata 2.0.6 semantics are the supported expression contract.
  - Custom templates permit HTML and CSS but no scripts, inline event handlers, or widget-authored JavaScript.
  - Web and desktop share dashboards through GryphDash configuration files.
  - TUI uses the same instances with its own ordering and generated data fallback.
  - Provider starter-template packs are optional and separate from provider reads.
  - Shipped templates are customized by copying, never editing in place.
  - Protocol v1 is not supported after cutover.
  - Existing browser and TUI layouts are deliberately discarded.
  - Every fresh default, visual style, widget selection, title, value, and behavior remains exactly as it is now until a user chooses to customize it.