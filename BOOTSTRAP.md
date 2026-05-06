# Bootstrap

This is now a root-level Flutter project. After cloning, install Flutter
(>= 3.27) and run:

```sh
flutter pub get
```

The standard commands are:

```sh
flutter test                              # runs the rules / scoring / SGF tests
flutter run                               # debug build on a connected device
flutter build appbundle --release         # Google Play AAB
flutter build ipa --release               # iOS .ipa (signed via Xcode)
```

## Android signing

Copy `android/key.properties.example` to `android/key.properties` and fill in
your release keystore details. `android/key.properties` and keystore files are
ignored by Git. If the file is present, Gradle signs release builds with that
upload key; otherwise local release builds fall back to debug signing for smoke
testing.

The `applicationId`, `bundleId`, and namespace are already set to `com.weiqi`.
