# Editing FilaTrack UI

`filatrack.cmb` at the repository root includes every GtkBuilder source view
and the application CSS. The `.ui` and `.css` files are the authoritative
layout sources loaded by the application, not exported screenshots or mockups.

Views are grouped by feature. Shared layouts are grouped by widget purpose:

```text
views/
├── app/                 # Main window, navigation, About dialog
├── dashboard/           # Metrics, activity, warnings, empty states
├── spools/              # Spool cards, dialogs, error labels
├── printers/            # Printer cards and dialogs
├── prints/              # History rows and active print cards
├── models/              # Model rows, upload/launch/file dialogs
├── drying/              # Drying history, recommendations, dialogs
├── settings/            # Material editor and rows
└── shared/
    ├── containers/      # Screen/dialog shells and action rows
    ├── labels/          # Titles and messages
    ├── dialogs/         # Shared modal window
    └── widgets/         # Color picker
```

| Source | Contents |
| --- | --- |
| `views/app/main-window.ui` | Application window and default dimensions |
| `views/app/main-layout.ui` | Header, sidebar, navigation, and all seven screens, including filters and settings sections |
| `views/dashboard/dashboard-metrics.ui` | All five metric cards, including optional captions |
| `views/dashboard/dashboard-*.ui` | Active printer cards, warning rows, recent print rows, and empty states |
| `views/spools/spool-dialog.ui` | Spool form, color chooser, errors, and actions |
| `views/spools/spool-card.ui` | Spool preview, remaining filament, drying status, and actions |
| `views/spools/dry-spool-dialog.ui`, `views/drying/drying-dialog.ui` | Drying forms and actions |
| `views/drying/drying-history-card.ui`, `views/drying/drying-recommendation-card.ui` | Repeated drying history and recommendation cards |
| `views/printers/printer-dialog.ui`, `views/printers/printer-card.ui` | Printer form and status/assignment card |
| `views/prints/print-row.ui`, `views/prints/active-print-card.ui` | Print history row and active print card |
| `views/models/model-row.ui` | Model preview and launch action |
| `views/models/model-upload-dialog.ui`, `views/models/print-launch-dialog.ui` | Upload and print launch workflows |
| `views/settings/material-dialog.ui`, `views/settings/material-row.ui` | Material editor and settings list row |
| `views/app/about-dialog.ui`, `views/models/gcode-file-dialog.ui` | Standard application and file dialogs |
| `views/shared/widgets/color-picker.ui`, `views/shared/labels/*-title.ui`, `views/shared/containers/dialog-*.ui` | Shared standalone widgets and shells |
| `styles/app.css`, `styles/macos.css` | Application styles and macOS window adjustments |

In Cambalache, select `app/main-layout.ui` and its `toolbarView` root to preview
the application shell. Change `stack.visible-child-name` to preview each page.
Edit a repeated row/card in its named view; runtime list factories instantiate
that layout for each record. The editor uses sample captions and does not
connect to databases or printers.

When moving a view, update its `file` entries in `components.json` and its
`filename` in `filatrack.cmb`. The loader embeds `views/` recursively, so new
subdirectories need no changes to Go embed patterns or controller names.

Save with Cambalache after editing. It writes both the project metadata and
its referenced UI/CSS files. The application embeds the saved sources directly,
so restart/rebuild it after each change. No UI generation step is required.

## Controller bindings

`layout.Load` creates an independent GtkBuilder instance. `layout.Scope`
binds a component controller to prefixed IDs inside a composed view.
`layout.Get` checks object IDs/types; `layout.Configure` applies runtime data.
For example, the main layout owns the spool screen's `spools_box` object,
while the spool screen controller addresses it as `box` through its scope.

`components.json` maps stable component names to their source path relative
to `views/`,
ID prefix, and object roots. This lets tests and standalone constructors load
a subtree from the same source used by the application. Keep IDs and widget
classes stable when changing layouts. If an ID or class intentionally changes,
update the controller bindings and component catalog together.

Translations, loaded field values, selected materials, validation, online
status, and event handling stay in Go. Widget geometry, initial properties,
and CSS live in the editor's source files. Dynamic lists keep their containers
in the layout and receive instances of the corresponding row/card view.
Cairo renders the spool illustration and filament level at runtime inside
`GtkDrawingArea`; their size and placement are editable, while Cambalache
shows their widget bounds rather than running the custom drawing code.

## Validation

On a desktop with GTK4/libadwaita and a display available:

```bash
go test ./internal/ui/...
```

The layout tests load every view and catalog component, check object IDs,
and exercise all navigation pages and toolbar attachments. Existing tests
cover forms, list factories, screen construction, and live data behavior.
