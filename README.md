# Weiqi (Go) — Android MVP

Implements the MVP from the SRS:

- Pure-Kotlin rules engine: capture, suicide, simple ko, positional superko (Zobrist).
- Chinese area scoring with dead-stone removal.
- SGF export.
- Beginner heuristic AI (capture / save atari / shape / contact preference).
- Jetpack Compose board UI for 9×9, 13×13, 19×19.
- Local 2-player and human-vs-AI.
- Room persistence layer for saved games.
- JUnit tests for the rules engine, scoring, and SGF.

## Layout

```
app/src/main/java/com/weiqi/
  engine/   # Pure-Kotlin rules, board, scoring (no Android deps)
  ai/       # Beginner AI
  sgf/      # SGF exporter
  data/     # Room database, serializer
  ui/       # Compose theme, board canvas, screens, ViewModel
  app/      # MainActivity + nav graph
app/src/test/  # Unit tests
```

## Build

This is an Android Gradle project (AGP 8.5, Kotlin 2.0, Compose).

You will need to populate the Gradle wrapper before first build:

```sh
gradle wrapper --gradle-version 8.7
./gradlew test           # runs the JVM unit tests
./gradlew assembleDebug  # builds the APK
```

Open in Android Studio (Hedgehog or newer) for normal development.

## Out of scope (deferred from the SRS)

Online multiplayer, ranked / matchmaking / clocks, MCTS or strong AI,
tutorials/puzzles content, SGF import, AI review, tournaments,
backend services, anti-cheat. The architecture keeps the engine
isolated, so these can be layered on later without touching the rules.
