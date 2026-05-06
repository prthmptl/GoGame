# Weiqi — Flutter

Cross-platform Flutter port of the Weiqi (Go) Android app. Supports Android and iOS.

## Features

- Pure-Dart rules engine: capture, suicide, simple ko, positional superko (Zobrist).
- Chinese area scoring with dead-stone marking.
- Heuristic beginner AI (capture / save atari / shape / 1-ply lookahead).
- Hand-painted board canvas (`CustomPainter`) for 9×9, 13×13, 19×19.
- Local 2-player and human-vs-AI with main-time clock.
- SGF export & main-line import; share via the OS share sheet.
- Saved games (sqflite) + auto-resume.
- Tutorial, full Chinese rules reference, settings (themes, coords, hints, move numbers).

## Layout

```
lib/src/
  domain/      # rules engine, AI — no Flutter imports
  sgf/         # SGF read/write
  data/        # sqflite repo, shared_preferences settings
  ui/          # theme, widgets, screens
```

## Build

```sh
flutter pub get
flutter run
flutter test
```

### Android Play Store bundle

```sh
flutter build appbundle --release
# Output: build/app/outputs/bundle/release/app-release.aab
```

The Android `applicationId` is `com.weiqi`. For Play upload signing, copy
`android/key.properties.example` to ignored file `android/key.properties` and
fill in your upload keystore details.

### iOS

Open `ios/Runner.xcworkspace` in Xcode and configure signing. Bundle id is `com.weiqi`.

```sh
cd ios && pod install
flutter build ipa --release
```
