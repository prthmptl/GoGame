# Go — Flutter

A calm, cross-platform board for the game of Go. Pure-Dart rules engine,
hand-painted board, beginner AI, SGF round-trip, and a built-in tutorial and
rules reference. Built with Flutter for Android and iOS, with desktop targets
planned.

> **Status:** v0.2.0 — local MVP. No online play; everything runs offline on the
> device.

> **History:** This repository started as a native Kotlin / Jetpack Compose
> Android app. It was migrated to Flutter on 2026-05-07 to enable iOS and
> desktop from a single codebase. The original Kotlin source is archived at
> [`prthmptl/GoGame-kotlin-legacy`](https://github.com/prthmptl/GoGame-kotlin-legacy)
> (private).

---

## Features

**Gameplay**

- 9×9, 13×13, and 19×19 boards
- Local two-player (pass-and-play) and human-vs-AI
- Main-time clock with configurable time control
- Undo / pass / resign
- Optional move numbers and last-move highlight

**Rules engine** (`lib/src/domain/`, no Flutter imports — fully unit-testable)

- Capture, suicide prevention, simple ko
- Positional superko via Zobrist hashing
- Chinese area scoring with interactive dead-stone marking
- Group / liberty calculation with flood fill

**AI** (`lib/src/domain/ai/beginner_ai.dart`)

- Heuristic beginner: prioritises captures, saves stones in atari, prefers good
  shape, applies a 1-ply lookahead. Designed to feel like a forgiving opponent
  rather than a strong engine.

**SGF**

- Export the current game to SGF (`FF[4]`)
- Import the main line of an SGF file via the system file picker
- Share games through the OS share sheet (`share_plus`)

**Persistence**

- Saved games via `sqflite` with auto-resume on launch
- Settings via `shared_preferences` (theme, coordinates, hints, move numbers)

**Learning**

- Step-by-step tutorial covering captures, life-and-death, and basic shape
- Full Chinese-rules reference screen

---

## Project layout

```
lib/
  main.dart                       # entrypoint, opens repo + settings, runs app
  src/
    app.dart                      # GoApp: GoRouter shell, bottom-nav chrome
    domain/                       # pure Dart — rules, board, AI
      board.dart                  # bitboard + intersection helpers
      game_state.dart             # current position, history, ko hash
      groups.dart                 # connected-stone / liberty queries
      rules.dart                  # legal-move check, capture, superko
      scoring.dart                # Chinese area scoring with dead marking
      models.dart                 # Color, Move, GameConfig, etc.
      ai/beginner_ai.dart         # heuristic opponent
    sgf/
      sgf.dart                    # serializer (FF[4])
      sgf_import.dart             # main-line parser
    data/
      saved_game.dart             # row model
      saved_game_repo.dart        # sqflite-backed repo
      settings_store.dart         # SharedPreferences wrapper
    ui/
      theme.dart                  # zen-leaning Material 3 theme
      board/
        board_canvas.dart         # CustomPainter rendering board + stones
        mini_stone.dart           # small reusable stone widget
      components/
        zen_components.dart       # shared buttons, cards, dividers
      screens/
        home_screen.dart          # recent games, "new game"
        setup_screen.dart         # board size, players, time control
        game_screen.dart          # the play screen
        game_view_model.dart      # ChangeNotifier wrapping GameState
        review_screen.dart        # post-game / saved-game playback
        tutorial_screen.dart      # lessons list + detail
        rules_screen.dart         # rules reference
        settings_screen.dart      # preferences
test/
  domain/                         # rules + scoring tests
  sgf/                            # SGF round-trip + import tests
tool/
  gen_icon.py                     # regenerate adaptive launcher icons
```

The `domain/`, `sgf/`, and `data/` layers contain no Flutter imports, so they
can run under plain Dart and stay easy to test.

---

## Tech stack

- **Flutter** ≥ 3.27, **Dart** ≥ 3.6
- [`go_router`](https://pub.dev/packages/go_router) — declarative routing with a
  shell route for the bottom-nav chrome
- [`sqflite`](https://pub.dev/packages/sqflite) — saved games
- [`shared_preferences`](https://pub.dev/packages/shared_preferences) —
  user settings
- [`share_plus`](https://pub.dev/packages/share_plus) — share SGF
- [`file_picker`](https://pub.dev/packages/file_picker) — import SGF
- [`flutter_launcher_icons`](https://pub.dev/packages/flutter_launcher_icons) —
  generates adaptive Android + iOS icons from `assets/icon/`

---

## Getting started

Install Flutter (≥ 3.27) and fetch packages:

```sh
flutter pub get
```

Run on a connected device or emulator:

```sh
flutter run
```

Run the test suite (rules engine, scoring, SGF round-trip):

```sh
flutter test
```

---

## Building releases

### Android (Google Play AAB)

```sh
flutter build appbundle --release
# Output: build/app/outputs/bundle/release/app-release.aab
```

The Android `applicationId` is `com.gogame`. For Play uploads, copy the example
keystore config and fill in your details:

```sh
cp android/key.properties.example android/key.properties
```

`android/key.properties` and `*.jks` files are gitignored. If `key.properties`
is present, Gradle signs release builds with that upload key; otherwise local
release builds fall back to debug signing for smoke testing.

### iOS

```sh
cd ios && pod install
flutter build ipa --release
```

Open `ios/Runner.xcworkspace` in Xcode to configure signing. Bundle id is
`com.gogame`.

### Desktop (planned)

Desktop platforms are not yet scaffolded. To enable them:

```sh
flutter create --platforms=linux,windows,macos .
flutter run -d linux        # or windows / macos
```

The Dart code under `lib/` should run as-is; only platform folders need to be
generated.

---

## Regenerating launcher icons

Source artwork lives in `assets/icon/`. After editing, regenerate platform
icons with:

```sh
dart run flutter_launcher_icons
```

`tool/gen_icon.py` is a helper for producing the foreground / background PNGs
from a vector source.

---

## Versioning

Version is tracked in `pubspec.yaml` as `<semver>+<buildNumber>` (e.g.
`0.2.0+3`). Bump the build number on every Play / TestFlight upload.

---

## Contributing

This is a personal project, but issues and PRs are welcome. When changing the
rules engine or scoring code, please add or update tests in `test/domain/`.
