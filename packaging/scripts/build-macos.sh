#!/usr/bin/env bash
set -euo pipefail

: "${FILATRACK_BUILD_MANAGED:?Run this script through: go run ./cmd/build macos}"
: "${FILATRACK_APP_NAME:?Missing application metadata}"
: "${FILATRACK_APP_ID:?Missing application metadata}"
: "${FILATRACK_VERSION:?Missing application metadata}"
: "${FILATRACK_MACOS_MIN_VERSION:?Missing application metadata}"

APP_NAME="$FILATRACK_APP_NAME"
APP_ID="$FILATRACK_APP_ID"
VERSION="$FILATRACK_VERSION"
MACOS_MIN_VERSION="$FILATRACK_MACOS_MIN_VERSION"
INSTALL_DEPS=0
CREATE_DMG=0
SIGN_IDENTITY="-"
PREBUILT_BINARY=""

usage() {
  cat <<'EOF'
Build a standalone FilaTrack.app bundle for macOS.

Usage: go run ./cmd/build macos [options]

Options:
  --install-deps       Install build dependencies with Homebrew
  --dmg                Also create dist/FilaTrack-<version>-macos-<arch>.dmg
  --sign IDENTITY      Sign with an Apple Developer ID (default: ad-hoc signing)
  --binary PATH        Package an existing binary instead of running go build
  -h, --help           Show this help

Examples:
  go run ./cmd/build macos --install-deps --dmg
  go run ./cmd/build macos --sign "Developer ID Application: Example (TEAMID)" --dmg
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --install-deps) INSTALL_DEPS=1; shift ;;
    --dmg) CREATE_DMG=1; shift ;;
    --sign)
      [[ $# -ge 2 ]] || { echo "--sign requires an identity" >&2; exit 2; }
      SIGN_IDENTITY="$2"; shift 2 ;;
    --binary)
      [[ $# -ge 2 ]] || { echo "--binary requires a path" >&2; exit 2; }
      PREBUILT_BINARY="$2"; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "Unknown option: $1" >&2; usage >&2; exit 2 ;;
  esac
done

command -v brew >/dev/null 2>&1 || {
  echo "Homebrew is required: https://brew.sh" >&2
  exit 1
}

if [[ "$INSTALL_DEPS" -eq 1 ]]; then
  # GTK does not guarantee that its default symbolic icon theme is installed as
  # a runtime dependency.  The app uses Adwaita names such as
  # preferences-system-symbolic and help-about-symbolic, so bundle the theme
  # explicitly instead of leaving toolbar icons up to the build machine.
  brew install go pkg-config gobject-introspection gtk4 libadwaita librsvg adwaita-icon-theme hicolor-icon-theme fontconfig
fi

required_tools=(awk brew codesign file find iconutil install_name_tool otool perl pkg-config rsvg-convert)
if [[ -z "$PREBUILT_BINARY" ]]; then
  required_tools+=(go)
fi
for tool in "${required_tools[@]}"; do
  command -v "$tool" >/dev/null 2>&1 || {
    echo "Required tool '$tool' is missing. Try: go run ./cmd/build macos --install-deps" >&2
    exit 1
  }
done

pkg-config --exists gtk4 libadwaita-1 gobject-introspection-1.0 || {
  echo "GTK4, libadwaita, and GObject Introspection development files are required. Try: go run ./cmd/build macos --install-deps" >&2
  exit 1
}
brew --prefix adwaita-icon-theme >/dev/null 2>&1 || {
  echo "Adwaita icon theme is required for toolbar icons. Try: go run ./cmd/build macos --install-deps" >&2
  exit 1
}

PROJECT_ROOT="$(cd "$(dirname "$0")/../.." && pwd -P)"
DIST_DIR="$PROJECT_ROOT/dist"
APP="$DIST_DIR/$APP_NAME.app"
CONTENTS="$APP/Contents"
MACOS_DIR="$CONTENTS/MacOS"
RESOURCES="$CONTENTS/Resources"
FRAMEWORKS="$CONTENTS/Frameworks"
ARCH="$(uname -m)"
BREW_PREFIX="$(brew --prefix)"
GTK_PREFIX="$(brew --prefix gtk4)"
ADWAITA_PREFIX="$(brew --prefix libadwaita)"
GLIB_PREFIX="$(brew --prefix glib)"
PIXBUF_PREFIX="$(brew --prefix gdk-pixbuf)"
RSVG_PREFIX="$(brew --prefix librsvg)"

mkdir -p "$DIST_DIR"
rm -rf -- "$APP"
mkdir -p "$MACOS_DIR" "$RESOURCES" "$FRAMEWORKS"

if [[ -n "$PREBUILT_BINARY" ]]; then
  case "$PREBUILT_BINARY" in
    /*) SOURCE_BINARY="$PREBUILT_BINARY" ;;
    *) SOURCE_BINARY="$PROJECT_ROOT/$PREBUILT_BINARY" ;;
  esac
  [[ -f "$SOURCE_BINARY" ]] || { echo "Binary not found: $SOURCE_BINARY" >&2; exit 1; }
  cp "$SOURCE_BINARY" "$MACOS_DIR/$APP_NAME"
else
  echo "Building $APP_NAME for macOS ($ARCH)..."
  (
    cd "$PROJECT_ROOT"
    CGO_ENABLED=1 go build -trimpath -ldflags="-s -w" -o "$MACOS_DIR/$APP_NAME" ./cmd/filatrack
  )
fi
chmod 0755 "$MACOS_DIR/$APP_NAME"

cat > "$CONTENTS/Info.plist" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "https://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>CFBundleDevelopmentRegion</key><string>en</string>
  <key>CFBundleDisplayName</key><string>$APP_NAME</string>
  <key>CFBundleExecutable</key><string>$APP_NAME</string>
  <key>CFBundleIconFile</key><string>$APP_NAME.icns</string>
  <key>CFBundleIdentifier</key><string>$APP_ID</string>
  <key>CFBundleInfoDictionaryVersion</key><string>6.0</string>
  <key>CFBundleName</key><string>$APP_NAME</string>
  <key>CFBundlePackageType</key><string>APPL</string>
  <key>CFBundleShortVersionString</key><string>$VERSION</string>
  <key>CFBundleVersion</key><string>$VERSION</string>
  <key>LSMinimumSystemVersion</key><string>$MACOS_MIN_VERSION</string>
  <key>NSHighResolutionCapable</key><true/>
</dict>
</plist>
EOF

echo "Creating application icon..."
ICONSET="$DIST_DIR/.$APP_NAME.iconset"
rm -rf -- "$ICONSET"
mkdir -p "$ICONSET"
for spec in "16:16x16" "32:16x16@2x" "32:32x32" "64:32x32@2x" \
            "128:128x128" "256:128x128@2x" "256:256x256" \
            "512:256x256@2x" "512:512x512" "1024:512x512@2x"; do
  pixels="${spec%%:*}"
  name="${spec#*:}"
  rsvg-convert -w "$pixels" -h "$pixels" \
    -o "$ICONSET/icon_${name}.png" "$PROJECT_ROOT/packaging/$APP_ID.svg"
done
if ! iconutil -c icns "$ICONSET" -o "$RESOURCES/$APP_NAME.icns" 2>/dev/null; then
  # Some macOS releases reject otherwise valid iconsets produced by librsvg.
  # An ICNS file is a simple typed container, so assemble the same PNGs directly.
  echo "iconutil rejected the generated iconset; using the compatible ICNS packer..."
  perl -e '
    use strict;
    use warnings;
    my @types = qw(icp4 ic11 icp5 ic12 ic07 ic13 ic08 ic14 ic09 ic10);
    my $body = "";
    for my $i (0 .. $#types) {
      open my $input, "<:raw", $ARGV[$i] or die "$ARGV[$i]: $!\n";
      local $/;
      my $data = <$input>;
      $body .= $types[$i] . pack("N", length($data) + 8) . $data;
    }
    open my $output, ">:raw", $ARGV[10] or die "$ARGV[10]: $!\n";
    print {$output} "icns", pack("N", length($body) + 8), $body;
  ' \
    "$ICONSET/icon_16x16.png" "$ICONSET/icon_16x16@2x.png" \
    "$ICONSET/icon_32x32.png" "$ICONSET/icon_32x32@2x.png" \
    "$ICONSET/icon_128x128.png" "$ICONSET/icon_128x128@2x.png" \
    "$ICONSET/icon_256x256.png" "$ICONSET/icon_256x256@2x.png" \
    "$ICONSET/icon_512x512.png" "$ICONSET/icon_512x512@2x.png" \
    "$RESOURCES/$APP_NAME.icns"
fi
rm -rf -- "$ICONSET"

copy_tree_if_present() {
  local source="$1"
  local destination="$2"
  if [[ -d "$source" ]]; then
    mkdir -p "$(dirname "$destination")"
    cp -R "$source" "$destination"
  fi
}

echo "Copying GTK runtime resources..."
copy_tree_if_present "$PIXBUF_PREFIX/lib/gdk-pixbuf-2.0" "$RESOURCES/lib/gdk-pixbuf-2.0"
copy_tree_if_present "$GLIB_PREFIX/lib/gio" "$RESOURCES/lib/gio"

mkdir -p "$RESOURCES/share/glib-2.0/schemas"
for prefix in "$GLIB_PREFIX" "$GTK_PREFIX" "$ADWAITA_PREFIX"; do
  if [[ -d "$prefix/share/glib-2.0/schemas" ]]; then
    find "$prefix/share/glib-2.0/schemas" -maxdepth 1 -type f \( -name '*.xml' -o -name '*.override' \) \
      -exec cp {} "$RESOURCES/share/glib-2.0/schemas/" \;
  fi
done
if brew --prefix gsettings-desktop-schemas >/dev/null 2>&1; then
  SETTINGS_PREFIX="$(brew --prefix gsettings-desktop-schemas)"
  if [[ -d "$SETTINGS_PREFIX/share/glib-2.0/schemas" ]]; then
    find "$SETTINGS_PREFIX/share/glib-2.0/schemas" -maxdepth 1 -type f \( -name '*.xml' -o -name '*.override' \) \
      -exec cp {} "$RESOURCES/share/glib-2.0/schemas/" \;
  fi
fi
"$GLIB_PREFIX/bin/glib-compile-schemas" "$RESOURCES/share/glib-2.0/schemas"

for formula in adwaita-icon-theme hicolor-icon-theme shared-mime-info; do
  if brew --prefix "$formula" >/dev/null 2>&1; then
    RESOURCE_PREFIX="$(brew --prefix "$formula")"
    if [[ -d "$RESOURCE_PREFIX/share/icons" ]]; then
      mkdir -p "$RESOURCES/share/icons"
      cp -R "$RESOURCE_PREFIX/share/icons/." "$RESOURCES/share/icons/"
    fi
    copy_tree_if_present "$RESOURCE_PREFIX/share/mime" "$RESOURCES/share/mime"
  fi
done

# CFBundleIconFile supplies the Finder/Dock icon, while AdwAboutDialog resolves
# SetApplicationIcon through GTK's icon theme. Install the same artwork under
# the application ID so both macOS and GTK can find it.
APP_ICON_DIR="$RESOURCES/share/icons/hicolor/scalable/apps"
mkdir -p "$APP_ICON_DIR"
cp "$PROJECT_ROOT/packaging/$APP_ID.svg" "$APP_ICON_DIR/$APP_ID.svg"
"$GTK_PREFIX/bin/gtk4-update-icon-cache" --force --quiet "$RESOURCES/share/icons/hicolor"

if brew --prefix fontconfig >/dev/null 2>&1; then
  FONTCONFIG_PREFIX="$(brew --prefix fontconfig)"
  copy_tree_if_present "$FONTCONFIG_PREFIX/etc/fonts" "$RESOURCES/etc/fonts"
  copy_tree_if_present "$FONTCONFIG_PREFIX/share/fontconfig" "$RESOURCES/share/fontconfig"
fi

cp "$PIXBUF_PREFIX/bin/gdk-pixbuf-query-loaders" "$MACOS_DIR/gdk-pixbuf-query-loaders"
chmod 0755 "$MACOS_DIR/gdk-pixbuf-query-loaders"

# Homebrew libraries use absolute install names. Copy every non-system dependency
# into Contents/Frameworks and rewrite references to relocatable @rpath names.
echo "Bundling dynamic libraries..."
queue=("$MACOS_DIR/$APP_NAME" "$MACOS_DIR/gdk-pixbuf-query-loaders")
while IFS= read -r module; do
  queue+=("$module")
done < <(find "$RESOURCES/lib" -type f \( -name '*.so' -o -name '*.dylib' \) 2>/dev/null)
processed="|"
index=0

resolve_dependency() {
  local dependency="$1"
  local owner="$2"
  local base="${dependency##*/}"
  local candidate=""

  if [[ "$dependency" == /* && -f "$dependency" ]]; then
    printf '%s\n' "$dependency"
    return 0
  fi
  if [[ "$dependency" == @loader_path/* ]]; then
    candidate="$(dirname "$owner")/${dependency#@loader_path/}"
    if [[ -f "$candidate" ]]; then
      printf '%s\n' "$candidate"
      return 0
    fi
  fi
  for candidate in "$FRAMEWORKS/$base" "$BREW_PREFIX/lib/$base" "$RSVG_PREFIX/lib/$base"; do
    if [[ -f "$candidate" ]]; then
      printf '%s\n' "$candidate"
      return 0
    fi
  done
  candidate="$(find "$BREW_PREFIX/opt" -path "*/lib/$base" -type f -print -quit 2>/dev/null || true)"
  [[ -n "$candidate" ]] && { printf '%s\n' "$candidate"; return 0; }
  return 1
}

while [[ "$index" -lt "${#queue[@]}" ]]; do
  target="${queue[$index]}"
  index=$((index + 1))
  [[ -f "$target" ]] || continue
  case "$processed" in *"|$target|"*) continue ;; esac
  processed="${processed}${target}|"
  chmod u+w "$target"

  case "$target" in
    *.dylib) install_name_tool -id "@rpath/$(basename "$target")" "$target" 2>/dev/null ;;
  esac

  while IFS= read -r dependency; do
    case "$dependency" in
      /usr/lib/*|/System/*) continue ;;
    esac
    source_path="$(resolve_dependency "$dependency" "$target" || true)"
    if [[ -z "$source_path" ]]; then
      echo "Unable to resolve dependency '$dependency' required by '$target'" >&2
      exit 1
    fi
    library_name="$(basename "$source_path")"
    bundled_library="$FRAMEWORKS/$library_name"
    if [[ ! -f "$bundled_library" ]]; then
      cp -L "$source_path" "$bundled_library"
      chmod u+w "$bundled_library"
      queue+=("$bundled_library")
    fi
    install_name_tool -change "$dependency" "@rpath/$library_name" "$target" 2>/dev/null
  done < <(otool -L "$target" | awk 'NR > 1 { print $1 }')

  install_name_tool -add_rpath "@executable_path/../Frameworks" "$target" 2>/dev/null || true
done

echo "Checking bundle portability..."
unbundled=0
while IFS= read -r candidate; do
  case "$(file -b "$candidate")" in
    *Mach-O*) macho="$candidate" ;;
    *) continue ;;
  esac
  while IFS= read -r dependency; do
    case "$dependency" in
      /usr/lib/*|/System/*) ;;
      @rpath/*)
        if [[ ! -f "$FRAMEWORKS/${dependency##*/}" ]]; then
          echo "Missing bundled dependency: $macho -> $dependency" >&2
          unbundled=1
        fi
        ;;
      @loader_path/*|@executable_path/*) ;;
      /*)
        echo "External absolute dependency: $macho -> $dependency" >&2
        unbundled=1
        ;;
    esac
  done < <(otool -L "$macho" | awk 'NR > 1 { print $1 }')
done < <(find "$MACOS_DIR" "$FRAMEWORKS" "$RESOURCES/lib" -type f 2>/dev/null)
[[ "$unbundled" -eq 0 ]] || exit 1

echo "Signing application bundle..."
sign_options=(--force --deep --sign "$SIGN_IDENTITY")
if [[ "$SIGN_IDENTITY" != "-" ]]; then
  sign_options+=(--options runtime --timestamp)
fi
codesign "${sign_options[@]}" "$APP"
codesign --verify --deep --strict "$APP"

if [[ "$CREATE_DMG" -eq 1 ]]; then
  DMG="$DIST_DIR/$APP_NAME-$VERSION-macos-$ARCH.dmg"
  rm -f -- "$DMG"
  hdiutil create -volname "$APP_NAME" -srcfolder "$APP" -ov -format UDZO "$DMG"
  echo "Built $DMG"
fi

echo "Built $APP"
echo "Application data will be stored in ~/Library/Application Support/$APP_NAME"
