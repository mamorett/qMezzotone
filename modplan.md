# Mod Plan — qMezzotone

> **Status:** Implemented. All four features (rename, CLI image arg, letter-jump,
> Makefile) are complete and verified: `go build ./...` and `go test ./...` pass,
> `make build` emits the six `qmezzotone-*` binaries. This plan was executed against
> the local fork whose module path is `github.com/mamorett/qMezzotone` (the upstream
> template referenced `github.com/joaoheitorgarcia/Mezzotone`; the rename target
> `qMezzotone`/`qmezzotone` is identical either way).

## Goal

Rename the project from **Mezzotone** to **qMezzotone**, plus two user-facing improvements to the TUI image-to-ASCII app and a Makefile for multiplatform builds.

> **Scope note for the implementer:** Confine all reads/edits to the repository at `/gorgon/dev/qMezzotone`. Do **not** search the surrounding filesystem (no `find /`, no broad globs outside this repo). Read the files listed below; that is everything needed.

---

## Context (read these first)

The plan below was drafted from a full read of these files. Re-read them before implementing to confirm exact signatures and current structure:

- `main.go` — entrypoint; parses `-debug`, `-font-ttf`; constructs the model via `NewMezzotoneModelWithConfig`, runs the `bubbletea` program.
- `internal/app/mezzotone.go` — the core Bubble Tea model. Holds `selectedFile`, the `filepicker`, `renderSettings`, `renderView`, menu navigation state, and the giant `Update`/`View` switch.
- `internal/app/appStrings.go` — `buildRenderHelpText(...)` builds the help overlay.
- `internal/ui/settings_panel.go` — `SettingsPanel`, `SettingItem`, `SettingType`.
- `internal/ui/animation_renderer.go` — `AnimationFrame`, `AnimationRenderer`, `TickMsg`.
- `internal/services/image_converter.go` — `RenderOptions`, `ConvertImageToString`, `ImageRuneArrayIntoString`, `NewRenderOptions`.
- `internal/services/log_service.go` — `FileLogger`, `InitLogger`, `Logger()`.
- `internal/export/export_to_png.go`, `export_to_gif.go`, `export_to_txt.go` — export pipelines.
- `internal/termtext/truncate.go` — `TruncateLinesANSI`.
- Tests: `internal/app/mezzotone_internal_test.go`, `internal/app/mezzotone_model_test.go`, `internal/app/mezzotone_render_view_keys_test.go`.

Module path: `github.com/mamorett/qMezzotone` (Go 1.25). Stack: `charm.land/bubbletea/v2`, `charm.land/bubbles/v2`, `charm.land/lipgloss/v2`.

---

## Feature 1 — Accept an image path as a CLI argument and open it directly

### 1.1 — Motivation
Today you must always navigate the file picker. The user wants to pass an image path on the command line and jump straight to the render-options panel for that file (skipping manual picking). Implemented as the first positional argument `mezzotone <image-path>`.

### 1.2 — Changes in `main.go`

1. Add a new flag for the image path. **Use a positional argument**, not a flag:
   ```bash
   mezzotone [flags] [<image-path>]
   ```
2. After `flag.Parse()`, grab the first positional arg:
   ```go
   imagePath := ""
   if args := flag.Args(); len(args) > 0 {
       imagePath = args[0]
   }
   ```
3. Pass it into the model config:
   ```go
   p := tea.NewProgram(app.NewMezzotoneModelWithConfig(app.MezzotoneModelConfig{
       ExportFontTTFPath: *fontTTF,
       ImagePath:         imagePath,
   }))
   ```
4. Keep `-debug` and `-font-ttf` working unchanged. If both a positional arg and flags are given, flags still apply.

### 1.3 — Changes in `internal/app/mezzotone.go`

1. Add the field to `MezzotoneModelConfig`:
   ```go
   type MezzotoneModelConfig struct {
       ExportFontTTFPath string
       ImagePath         string
   }
   ```
2. Add a field on `MezzotoneModel`:
   ```go
   initialImagePath string
   ```
3. In `NewMezzotoneModelWithConfig`, store it:
   ```go
   model.initialImagePath = strings.TrimSpace(config.ImagePath)
   ```
4. In `Init()`, if an initial image path was supplied, validate it and auto-select it so the model boots into the render-options menu:
   ```go
   func (m *MezzotoneModel) Init() tea.Cmd {
       cmds := []tea.Cmd{m.filePicker.Init()}
       if m.initialImagePath != "" {
           if m.isValidImagePath(m.initialImagePath) {
               m.selectedFile = m.initialImagePath
               m.renderSettings.SetActive(0)
               m.renderSettings.Confirm = false
               m.currentActiveMenu = renderOptionsMenu
               m.updateMessageViewPortContent("Edit render options and confirm:", false)
               _ = services.Logger().Info(fmt.Sprintf("Initial image: %s", m.selectedFile))
           } else {
               m.updateMessageViewPortContent("⚠ Invalid image path: "+m.initialImagePath, true)
           }
       }
       return tea.Batch(cmds...)
   }
   ```
5. Add a small validation helper (new function in this file):
   ```go
   func (m *MezzotoneModel) isValidImagePath(path string) bool {
       info, err := os.Stat(path)
       if err != nil || info.IsDir() {
           return false
       }
       ext := strings.ToLower(filepath.Ext(path))
       for _, allowed := range m.filePicker.AllowedTypes {
           if ext == allowed {
               return true
           }
       }
       // fall back to content sniffing for files without a recognized extension
       f, err := os.Open(path)
       if err != nil {
           return false
       }
       defer f.Close()
       _, format, err := image.DecodeConfig(f)
       if err != nil {
           return false
       }
       switch format {
       case "png", "jpeg", "gif", "bmp", "tiff", "webp":
           return true
       }
       return false
   }
   ```
6. If the path is invalid, show the warning message in the message viewport and let the user fall back to the file picker (do **not** quit).

### 1.4 — Tests to add / update

- In `internal/app/mezzotone_model_test.go` (package `app_test`), add:
  - `TestInitialImagePath_SelectsFileAndJumpsToRenderOptions` — construct a model with a valid test image path (use `internal/services/testdata/gradient_edges.png` copied into a temp dir, or any repo-local fixture), assert `currentActiveMenu == renderOptionsMenu` and `selectedFile` is set after `Init()`.
  - `TestInitialImagePath_InvalidPath_ShowsErrorAndStaysInPicker` — pass a bogus path, assert the message viewport contains the warning and `currentActiveMenu == filePickerMenu`.
- Keep all existing tests green.

---

## Feature 2 — Jump to a file in the picker by typing the first letter of its name

### 2.1 — Motivation
The file picker lists many files. The user wants to press a letter (e.g. `a`) to jump the cursor to the first file/directory whose name starts with that letter (case-insensitive), like classic terminal file managers.

### 2.2 — Constraints

- The `bubbles/v2` `filepicker.Model` does **not** expose its file list or cursor index publicly. Do not fork or patch the vendored library.
- Implement the jump as a **wrapper** around the existing filepicker: intercept key presses in the app's `Update` while in `filePickerMenu`, and when a single printable character is pressed, move the cursor via repeated `Down`/`Up` key injections until the selected entry's name starts with that character.
- Only act on single printable rune presses (`a`–`z`, `0`–`9`, etc.). Do **not** intercept control keys, arrows, `enter`, `esc`, `h`, `j`, `k`, `pgup`, `pgdown`, etc. Those keep working as before.
- Case-insensitive prefix match on the entry's displayed name.
- If no entry matches, do nothing (no error, no cursor movement).

### 2.3 — Changes in `internal/app/mezzotone.go`

1. Add a helper that returns the currently selected file name from the picker. The `filepicker.Model` exposes `SelectedEntry` (verify by reading the installed package source under the Go module cache — **do not** search the whole filesystem; if the method is absent, fall back to sending `Down`/`Up` key messages and tracking the cursor yourself). Prefer reading the actual installed source to confirm the API rather than guessing.

2. Add a new method on `MezzotoneModel`:
   ```go
   // jumpToFilePrefix moves the filepicker cursor to the first entry whose
   // name starts with the given rune (case-insensitive). It injects Up/Down
   // key messages into the existing filepicker so we don't depend on any
   // unexported state.
   func (m *MezzotoneModel) jumpToFilePrefix(r rune) {
       target := unicode.ToLower(r)
       // try forward from current position, wrapping around
       steps := 0
       total := m.filePickerTotalEntries() // helper; see step 3
       for steps < total {
           name := m.filePickerCurrentName() // helper; see step 3
           if name != "" && unicode.ToLower([]rune(name)[0]) == target {
               return
           }
           m.filePicker, _ = m.filePicker.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
           steps++
       }
   }
   ```
   > **Note:** The two helpers `filePickerTotalEntries` and `filePickerCurrentName` must be implemented by reading the actual `bubbles/v2/filepicker` source in the Go module cache. If neither `SelectedEntry` nor a reliable entry count is exported, implement the jump by capturing the file list once at construction time (the picker is built in `NewMezzotoneModelWithConfig`) and tracking the cursor index internally, then injecting `Down`/`Up` until the internally-tracked index points at a matching name. Confirm the chosen approach against the installed source before writing code.

3. In the `Update` method, inside the `tea.KeyMsg` case and the `m.currentActiveMenu == filePickerMenu` branch, add handling **before** the existing `m.filePicker.Update(msg)` call:
   ```go
   if m.currentActiveMenu == filePickerMenu {
       if isFileJumpKey(msg) {
           r := []rune(msg.String())[0]
           m.jumpToFilePrefix(r)
           return m, nil
       }
   }
   ```
   With the predicate:
   ```go
   func isFileJumpKey(msg tea.KeyMsg) bool {
       if msg.Type != tea.KeyRunes || len(msg.Runes) != 1 {
           return false
       }
       r := msg.Runes[0]
       return unicode.IsLetter(r) || unicode.IsDigit(r)
   }
   ```

4. Update the help text in `internal/app/appStrings.go` (`buildRenderHelpText`) — under the `* File Picker` section, add:
   ```go
   helpBinding("<letter>", "Jump to file starting with that letter", keyStyle, descriptionStyle),
   ```

### 2.4 — Tests to add

- In `internal/app/mezzotone_model_test.go`, add:
  - `TestJumpToFilePrefix_MovesCursor` — build a model whose filepicker is rooted in a temp dir containing known files (e.g. `alpha.png`, `beta.png`, `gamma.png`), send a key press of `b`, and assert the picker's selected entry is `beta.png`.
  - `TestJumpToFilePrefix_NoMatch_LeavesCursor` — same setup, press `z`, assert selection unchanged.
  - `TestJumpToFilePrefix_IgnoreNonRunes` — send `enter`, `esc`, `j`, `pgup` and assert the jump logic does not fire (cursor unchanged).
- Keep all existing tests green.

---

## Feature 3 — Multiplatform Makefile

### 3.1 — Motivation
Replace the shell-based build (`tools/build.sh`) with a portable `Makefile` that builds all four target platforms. The existing `tools/build.sh` is the reference for target names and output layout.

### 3.2 — Output layout (keep compatible with existing `build/` contents)

Build into `build/`:

| Target              | Binary                            |
|---------------------|-----------------------------------|
| Linux amd64         | `build/app-linux-amd64`           |
| Linux arm64         | `build/app-linux-arm64`           |
| macOS (darwin) amd64| `build/app-macos-amd64`           |
| macOS (darwin) arm64| `build/app-macos-arm64`           |
| Windows amd64       | `build/app-windows-amd64.exe`     |
| Windows arm64       | `build/app-windows-arm64.exe`     |

> Drop the legacy `armv6`/`armv7` Raspberry-Pi targets that `tools/build.sh` currently emits — the user explicitly asked for the four platforms above only. If you keep them, note it; otherwise remove them.

### 3.3 — Makefile requirements

Create `Makefile` at repo root with:

```makefile
APP_NAME    := app
MAIN_PKG    := .
BUILD_DIR   := build
GO          := go
GOFLAGS     := -trimpath
LDFLAGS     := -s -w

PLATFORMS := \
	linux/amd64 \
	linux/arm64 \
	darwin/amd64 \
	darwin/arm64 \
	windows/amd64 \
	windows/arm64

.PHONY: all build clean test $(PLATFORMS)

all: build

build: $(PLATFORMS)

$(PLATFORMS):
	@$(eval GOOS = $(word 1,$(subst /, ,$@)))
	@$(eval GOARCH = $(word 2,$(subst /, ,$@)))
	@$(eval EXT = $(if $(filter windows,$(GOOS)),.exe,))
	@mkdir -p $(BUILD_DIR)
	GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=0 $(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(APP_NAME)-$(GOOS)-$(GOARCH)$(EXT) $(MAIN_PKG)

clean:
	rm -rf $(BUILD_DIR)

test:
	$(GO) test ./...
```

### 3.4 — Behavior

- `make` or `make build` — build all six platform binaries into `build/`.
- `make linux/amd64` — build a single target.
- `make clean` — remove `build/`.
- `make test` — run the Go test suite.
- `CGO_ENABLED=0` by default (matches `tools/build.sh`). Document in the README that CGO is optional.
- Use `-trimpath` and `-ldflags "-s -w"` to match the existing build script's reproducibility/strip behavior.

### 3.5 — README update

Update the **Build binaries** section of `README.md`:

```markdown
## Build binaries

Use the Makefile:

```bash
make          # build all platforms into build/
make clean    # remove build/
make test     # run tests
```

To build a single platform:

```bash
make linux/amd64
```

Override variables as needed:

```bash
APP_NAME=myapp MAIN_PKG=./cmd/myapp make build
```

> The legacy `tools/build.sh` is kept for reference but `make` is the canonical build.
```

---

## Feature 0 — Rename Mezzotone → qMezzotone (do this FIRST)

> **Why first:** Features 1, 2 and 3 all touch the same Go source and filenames. Rename once, up front, and the other features land on the new names. Doing it last would mean re-editing every file twice.
>
> **Binding constraint:** Treat every form of the old name as case-insensitive and rename **all** of them: `Mezzotone`, `mezzotone`, `MEZZOTONE`, `MezzotoneModel`, `MezzotoneModelConfig`, `NewMezzotoneModel*`, the export-filename prefix `Mezzotone_`, and the module path `github.com/joaoheitorgarcia/Mezzotone`. The **new** canonical short name is `qmezzotone` (CLI invocation / binary base name); the **new** human-readable title and exported identifiers use `qMezzotone`/`QMezzotone` as mapped in the table below.

### 0.1 — New name mapping

| What | Old | New |
|------|-----|-----|
| Module path | `github.com/joaoheitorgarcia/Mezzotone` | `github.com/mamorett/qMezzotone` |
| CLI invocation | `mezzotone` | `qmezzotone` |
| Binary base name (Makefile `APP_NAME`) | `app` | `qmezzotone` |
| Go identifiers | `MezzotoneModel`, `MezzotoneModelConfig`, `NewMezzotoneModel`, `NewMezzotoneModelWithConfig` | `QMezzotoneModel`, `QMezzotoneModelConfig`, `NewQMezzotoneModel`, `NewQMezzotoneModelWithConfig` |
| Export filename prefix (`Mezzotone_<uuid>.ext`) | `Mezzotone_` | `QMezzotone_` |
| Source filename prefix | `mezzotone_*.go` | `qmezzotone_*.go` |
| README / help heading | `Mezzotone` | `qMezzotone` |

> **Keep stable (do not rename):** function names on the model like `incrementCurrentActiveMenu`, `updateMessageViewPortContent`, etc. — only the type/config/constructor names in the table above change. Internal string keys (`"textSize"`, `"fontAspect"` …), `filepicker.AllowedTypes`, and package names (`app`, `ui`, `services`, `export`, `termtext`) stay exactly as-is.

### 0.2 — Module path

In `go.mod`, change:

```
module github.com/joaoheitorgarcia/Mezzotone
```
→
```
module github.com/joaoheitorgarcia/qMezzotone
```

Then update **every** `import` of the old module path. The affected files (grep `-i mezzotone` over the repo) are:

- `main.go`
- `internal/app/mezzotone.go` (+ its renamed self)
- `internal/app/mezzotone_internal_test.go`
- `internal/app/mezzotone_model_test.go`
- `internal/app/mezzotone_render_view_keys_test.go`
- `internal/app/mezzotone_export_clipboard_test.go`
- `internal/services/services_render_test.go`
- `internal/services/image_converter.go` (no import, verify) / `log_service.go`
- `internal/ui/animation_renderer.go` (+ test)
- `internal/ui/settings_panel.go` (+ test)
- `internal/termtext/truncate.go` (+ test)
- `internal/export/*.go`

> **Tooling note:** After editing imports, run `go mod tidy` from the repo root. Since `go.sum` currently holds zero occurrences of the old path (verified), tidy should not need network access beyond normal module resolution — confirm with `go build ./...`.

Because the old path is a GitHub/import URL, also update any code-references that aren't Go imports:

- `.github/workflows/manual.yml` — verified: **no occurrence** of the old name (skip).
- `tools/build.sh` — verified: **no occurrence** (skip). Keep as legacy.

### 0.3 — Rename identifiers (Go code)

Rename these four exported symbols **exactly** (case-sensitive):

```go
// before
MezzotoneModel      → QMezzotoneModel
MezzotoneModelConfig → QMezzotoneModelConfig
NewMezzotoneModel    → NewQMezzotoneModel
NewMezzotoneModelWithConfig → NewQMezzotoneModelWithConfig
```

Every reference must move with the rename — constructors in `mezzotone.go`, the call site in `main.go`, and all the tests (`*_test.go` use `NewMezzotoneModel()` and the config type).

### 0.4 — Rename export-filename prefix

In `internal/app/mezzotone.go`, three branches build the output path (`case "t"`, `case "i"`, `case "g"`). Each does:

```go
outPath := filepath.Join(homeDir, "Mezzotone_"+generatedUuid.String()+".ext")
```

Change `"Mezzotone_"` → `"QMezzotone_"` in all three.

### 0.5 — Rename source files

Rename (git move, to preserve history):

```
internal/app/mezzotone.go                      → internal/app/qmezzotone.go
internal/app/mezzotone_internal_test.go        → internal/app/qmezzotone_internal_test.go
internal/app/mezzotone_model_test.go           → internal/app/qmezzotone_model_test.go
internal/app/mezzotone_render_view_keys_test.go → internal/app/qmezzotone_render_view_keys_test.go
internal/app/mezzotone_export_clipboard_test.go → internal/app/qmezzotone_export_clipboard_test.go
```

> The **package declarations** (`package app` / `package app_test`) inside each file **stay the same** — Go allows a filename to differ from the package name. Only the filesystem name changes.

### 0.6 — Update tests that assert the old export prefix

`internal/app/qmezzotone_export_clipboard_test.go` asserts paths like
`filepath.Join(tmpHome, "Mezzotone_"+fixedUUID.String()+".txt")` (and `.png`, `.gif`).
Update **all four** of those assertions to `"QMezzotone_"`. If you miss these, the tests fail on every platform — note that on macOS `go test` runs from a temp dir, so a missed assertion would surface there too.

### 0.7 — Update strings shown to the user

In `internal/app/appStrings.go` → `buildRenderHelpText`, the help overlay currently
references the project heading via `sectionStyle.Render(...)`. Verify there is
no hardcoded "Mezzotine"/"Mezzotone" literal; if one is present, replace it
with `qMezzotone`. (The verbatim help text uses generic labels, so this is
typically a no-op — confirm by reading the file.)

Standing message strings — **update these** to the new product name where they mention the app:

In `internal/app/qmezzotone.go`:
- `"Select image or gif to convert:"` → keep (generic, no rename).
- The verbatim strings in `updateMessageTextOnMenuChange` and the welcome message are generic — only rename references that literally spell `Mezzotone`.

### 0.8 — Update README.md

Replace the title and every `mezzotone`/`Mezzotine`/`Mezzotone` mention (10 occurrences) with the new name, **keeping the badge URLs and GitHub links intact** (they point to the external repo and must not change). Specifically:

- `# Mezzotone` → `# qMezzotone`
- "Mezzotone is a terminal UI..." → "qMezzotone is a terminal UI..."
- CLI block `mezzotone [flags]` → `qmezzotone [flags]`
- All `go run .` / invocation examples: leave `go run .` as-is (it depends on module path, not the name), but where the **command name** `mezzotone` appears, change to `qmezzotone`.

### 0.9 — Update modplan.md itself

After implementing, update the heading and any instructional references to the
command/file names so the plan and the code agree.

### 0.10 — Rename verification (run before any feature work)

From the repo root:

```bash
grep -rni "mezzotine\|mezzotone" --include="*.go" --include="*.md" .
```

The matches that **should remain** after the rename:

- The new module path `github.com/joaoheitorgarcia/qMezzotone` (note the lowercase `q` — it still contains the substring `mezzotone`, so exclude `qmezzotone` from the check, or search for the literal `joaoheitorgarcia/Mezzotone` and `Mezzotine`/`NewMezzotone`/`APP_NAME := app`).

Specifically assert **zero** matches for:

- `joaoheitorgarcia/Mezzotone` (capital M, no q prefix) — must be fully gone.
- `NewMezzotoneModel` / `MezzotoneModel` / `MezzotoneModelConfig` — must be gone.
- filename string literal `Mezzotone_` in export paths — must be gone.
- `APP_NAME := app` in the Makefile → must become `APP_NAME := qmezzotone`.
- CLI command `mezzotone` (standalone) in README/prose → `qmezzotone`.

Then confirm compilation and tests:

```bash
go build ./...
go test ./...
```

Both must pass with zero matches for the old tokens (other than the expected `qmezzotone` substring in the new module path).

---

## Implementation order

1. **Feature 0 (rename)** — do this first. After this step, `go build ./...` and `go test ./...` must pass and the greps in §0.10 must come back clean.
2. **Feature 3 (Makefile)** — standalone, no code changes. Verify with `make build` and `make test`. Set `APP_NAME := qmezzotone`.
3. **Feature 1 (CLI image arg)** — touches `main.go` and `internal/app/qmezzotone.go`. Add tests.
4. **Feature 2 (letter-jump)** — touches `internal/app/qmezzotone.go` and `internal/app/appStrings.go`. Add tests. **Read the installed `bubbles/v2/filepicker` source from the Go module cache first** to confirm which public API is available for reading the current selection / entry count before writing the jump helper.

## Definition of done

- All tokens from §0.10 are gone; `go build ./...` and `go test ./...` pass.
- `make build` produces all six binaries in `build/` named `qmezzotone-*`.
- `make test` passes (existing + new tests).
- `qmezzotone /path/to/image.png` opens directly into the render-options panel for that image.
- `qmezzotone` (no arg) behaves exactly as before.
- In the file picker, pressing a letter jumps to the first matching entry; non-letter control keys are unaffected.
- Help text documents the new `<letter>` binding.
- README and in-app strings use `qMezzotone`.
- No behavior regressions in existing tests.
